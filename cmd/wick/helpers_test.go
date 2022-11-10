/*
*
* Copyright 2021-2022 Simple Things Inc.
*
* Permission is hereby granted, free of charge, to any person obtaining a copy
* of this software and associated documentation files (the "Software"), to deal
* in the Software without restriction, including without limitation the rights
* to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
* copies of the Software, and to permit persons to whom the Software is
* furnished to do so, subject to the following conditions:
*
* The above copyright notice and this permission notice shall be included in all
* copies or substantial portions of the Software.
*
* THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
* IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
* FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
* AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
* LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
* OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
* SOFTWARE.
*
 */

package main_test

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gammazero/nexus/v3/router"
	"github.com/gammazero/nexus/v3/transport/serialize"
	"github.com/gammazero/nexus/v3/wamp"
	"github.com/gammazero/workerpool"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	main "github.com/s-things/wick/cmd/wick"
	"github.com/s-things/wick/core"
)

const (
	testRealm       = "wick.test"
	sessionCount    = 1000
	testConcurrency = 100
)

func startTestServer(t *testing.T, wss *router.WebsocketServer) (wsURL string) {
	mux := http.ServeMux{}
	mux.HandleFunc("/ws", wss.ServeHTTP)
	srv := httptest.NewServer(&mux)
	t.Cleanup(srv.Close)
	wsURL = strings.Replace(srv.URL, "http://", "ws://", 1) + "/ws"
	return wsURL
}

func startWsServer(t *testing.T) string {
	realmConfig := &router.RealmConfig{
		URI:              wamp.URI(testRealm),
		StrictURI:        true,
		AnonymousAuth:    true,
		AllowDisclose:    true,
		RequireLocalAuth: true,
	}
	config := &router.Config{
		RealmConfigs: []*router.RealmConfig{realmConfig},
	}
	rout, err := router.NewRouter(config, log.New())
	require.NoError(t, err)
	t.Cleanup(rout.Close)
	// Create websocket server.
	wss := router.NewWebsocketServer(rout)
	return startTestServer(t, wss)
}

func TestSessions(t *testing.T) {
	wsURL := startWsServer(t)
	testClientInfo := &core.ClientInfo{
		Realm:      testRealm,
		Serializer: serialize.JSON,
		AuthMethod: "anonymous",
		Url:        wsURL,
	}
	t.Run("TestConnect", func(t *testing.T) {
		session, err := main.Connect(testClientInfo, 1)
		require.NoError(t, err)
		require.Equal(t, true, session.Connected(), "get already closed session")
		session.Close()
	})

	t.Run("TestGetSessions", func(t *testing.T) {
		sessions, err := main.GetSessions(testClientInfo, sessionCount, testConcurrency, 0)
		defer func() {
			wp := workerpool.New(len(sessions))
			for _, sess := range sessions {
				s := sess
				wp.Submit(func() {
					// Close the connection to the router
					s.Close()
				})
			}
			wp.StopWait()
		}()
		require.NoError(t, err)
		require.Equal(t, sessionCount, len(sessions))
	})
}

func TestSerializerSelect(t *testing.T) {
	for _, data := range []struct {
		name               string
		expectedSerializer serialize.Serialization
		message            string
	}{
		{"json", serialize.JSON, fmt.Sprintf("invalid serializer id for json, expected=%d", serialize.JSON)},
		{"cbor", serialize.CBOR, fmt.Sprintf("invalid serializer id for cbor, expected=%d", serialize.CBOR)},
		{"msgpack", serialize.MSGPACK, fmt.Sprintf("invalid serializer id for msgpack, expected=%d", serialize.MSGPACK)},
		{"halo", -1, "should not accept as only anonymous,ticket,wampcra,cryptosign are allowed"},
	} {
		serializerId := main.GetSerializerByName(data.name)
		assert.Equal(t, data.expectedSerializer, serializerId, data.message)
	}
}

func TestSelectAuthMethod(t *testing.T) {
	for _, data := range []struct {
		privateKey     string
		ticket         string
		secret         string
		expectedMethod string
	}{
		{"b99067e6e271ae300f3f5d9809fa09288e96f2bcef8dd54b7aabeb4e579d37ef", "", "", "cryptosign"},
		{"", "williamsburg", "", "ticket"},
		{"", "", "williamsburg", "wampcra"},
		{"", "", "", "anonymous"},
	} {
		method := main.SelectAuthMethod(data.privateKey, data.ticket, data.secret)
		assert.Equal(t, data.expectedMethod, method, "problem in choosing auth method")
	}
}

func TestValidateUrl(t *testing.T) {
	for _, invalidUrl := range []string{
		"foo",
		"localhost",
		"ws//localhost",
		"ws//:localhost",
	} {
		err := main.ValidateURL(invalidUrl)
		assert.Error(t, err, fmt.Sprintf("%s is an invalid url must return error.", invalidUrl))
	}

	for _, validUrl := range []string{
		"ws://localhost:8080/",
		"ws://localhost:8080/ws",
		"rs://localhost:8080/",
		"tcp://localhost:8080/",
		"wss://localhost:8080/",
		"rss://localhost:8080/",
		"wss://localhost:8080/wss",
	} {
		err := main.ValidateURL(validUrl)
		assert.NoError(t, err)
	}
}

func TestValidateSerializer(t *testing.T) {
	for _, validSerializer := range []string{
		main.Json,
		main.Cbor,
		main.MsgPack,
	} {
		err := main.ValidateSerializer(validSerializer)
		assert.NoError(t, err)
	}

	for _, invalidSerializer := range []string{
		"Json",
		"foo",
		"serializer",
		"",
	} {
		err := main.ValidateSerializer(invalidSerializer)
		assert.Error(t, err, fmt.Sprintf("%s is an invalid serializer must return error.", invalidSerializer))
	}
}

func TestValidateAuthMethod(t *testing.T) {
	for _, validAuthMethod := range []string{
		main.AnonymousAuth,
		main.TicketAuth,
		main.WampCraAuth,
		main.CryptosignAuth,
	} {
		err := main.ValidateAuthMethod(validAuthMethod)
		assert.NoError(t, err)
	}

	for _, invalidAuthMethod := range []string{
		"foo",
		"crypto",
		"WampCra",
	} {
		err := main.ValidateSerializer(invalidAuthMethod)
		assert.Error(t, err, fmt.Sprintf("%s is an invalid authmethod must return error.", invalidAuthMethod))
	}
}

func TestValidateRealm(t *testing.T) {
	for _, inValidRealm := range []string{
		"test realm",
		"",
	} {
		err := main.ValidateRealm(inValidRealm)
		assert.Error(t, err)
	}

	for _, validRealm := range []string{
		"com.test.realm",
		"com.test.realm_1",
		"com.test.realm-1",
	} {
		err := main.ValidateRealm(validRealm)
		assert.NoError(t, err)
	}

}

func TestValidatePrivateKey(t *testing.T) {
	for _, validPrivateKey := range []string{
		"a728764c7c53fd631c9266c92099bf62d3f72f56514bb7ed4bffdbea79a8d7d6",
		"e511d66398d742a3e1e962edb202702a924e0d9c33dfbf0d92e9b14bafad1663",
	} {
		err := main.ValidatePrivateKey(validPrivateKey)
		assert.NoError(t, err)
	}

	for _, invalidPrivateKey := range []string{
		"foo",
		"test key",
		"3fd631c9266c92099bf62d3f72f565143fd631c9266c92099bf62d3f72f565143fd631c9266c92099bf62d3f72f56514",
	} {
		err := main.ValidatePrivateKey(invalidPrivateKey)
		assert.Error(t, err, fmt.Sprintf("%s is an invalid privatekey must return error.", invalidPrivateKey))
	}
}

func TestAskForInput(t *testing.T) {
	for _, data := range []struct {
		options *main.InputOption

		userInput      io.Reader
		expectedOutput string
	}{
		{&main.InputOption{Query: "", DefaultVal: "", Required: true, Loop: false, ValidateFunc: nil},
			bytes.NewBufferString("hello test"), "hello test"},
		// test default
		{&main.InputOption{Query: "", DefaultVal: "foo", Required: false, Loop: false, ValidateFunc: nil},
			bytes.NewBufferString(""), "foo"},
	} {
		actualOutput, err := main.AskForInput(data.userInput, ioutil.Discard, data.options)
		assert.NoError(t, err)
		assert.Equal(t, data.expectedOutput, actualOutput)
	}
}

func TestRead(t *testing.T) {
	var userInput = bytes.NewBufferString("test input")
	var bReader = bufio.NewReader(userInput)
	var expectedOutput = "test input"

	output, err := main.Read(bReader)
	assert.NoError(t, err)
	assert.Equal(t, expectedOutput, output)
}
