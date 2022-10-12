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

package core_test

import (
	"testing"

	"github.com/gammazero/nexus/v3/client"
	"github.com/gammazero/nexus/v3/transport/serialize"
	"github.com/gammazero/nexus/v3/wamp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s-things/wick/core"
)

const (
	realm             = "realm1"
	serializer        = serialize.JSON
	authId            = "john"
	authRole          = "test"
	secret            = "williamsburg"
	keepaliveInterval = 0
)

func checkBaseConfig(cfg client.Config, t *testing.T) {
	assert.Equal(t, realm, cfg.Realm, "wrong realm")

	assert.Equal(t, serializer, cfg.Serialization, "wrong serializer")

	assert.Equal(t, authId, cfg.HelloDetails["authid"], "wrong authid")

	assert.Equal(t, authRole, cfg.HelloDetails["authrole"], "wrong authrole")
}

func TestAnonymousConfig(t *testing.T) {
	cfg := core.GetAnonymousAuthConfig(realm, serializer, authId, authRole, keepaliveInterval)

	checkBaseConfig(cfg, t)

	if len(cfg.AuthHandlers) != 0 {
		t.Error("no authentications needed in anonymous")
	}
}

func TestTicketConfig(t *testing.T) {
	cfg := core.GetTicketAuthConfig(realm, serializer, authId, authRole, secret, keepaliveInterval)

	checkBaseConfig(cfg, t)

	_, exists := cfg.AuthHandlers["ticket"]
	if !exists {
		t.Error("ticket auth not found in handlers")
	}
}

func TestCRAConfig(t *testing.T) {
	cfg := core.GetCRAAuthConfig(realm, serializer, authId, authRole, secret, keepaliveInterval)

	checkBaseConfig(cfg, t)

	_, exists := cfg.AuthHandlers["wampcra"]
	if !exists {
		t.Error("wampcra auth not found in handlers")
	}
}

func TestCryptoSignConfig(t *testing.T) {
	cfg, err := core.GetCryptosignAuthConfig(realm, serializer, authId, authRole, privateKeyHex, keepaliveInterval)
	require.NoError(t, err)
	checkBaseConfig(*cfg, t)

	_, exists := cfg.AuthHandlers["cryptosign"]
	if !exists {
		t.Error("cryptosign auth not found in handlers")
	}
}

func TestHandleCryptosign(t *testing.T) {
	_, pvk, err := core.GetKeyPair(privateKeyHex)
	require.NoError(t, err)
	callable := core.HandleCryptosign(pvk)

	challengeHex := "a1d483092ec08960fedbaed2bc1d411568a59077b794210e251bd3abb1563f7c"
	signedHex := "906b90ae9b8ebb76c0005e2092ea3c77e3d832d841909c18dd25a9d8c87681337a6fd9938c38f7c77216cd5915e7" +
		"396e942ed4de2eee71d4068f4cc12cb6a40a"

	fakeChallenge := wamp.Challenge{Extra: map[string]interface{}{"challenge": challengeHex}}

	response, _ := callable(&fakeChallenge)

	assert.Equalf(t, signedHex+challengeHex, response, "crytosign authentication failed")
}
