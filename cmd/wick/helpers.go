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

package main

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gammazero/nexus/v3/client"
	"github.com/gammazero/nexus/v3/transport/serialize"
	"github.com/gammazero/nexus/v3/wamp"
	"github.com/gammazero/workerpool"
	"gopkg.in/ini.v1"

	"github.com/s-things/wick/core"
)

const (
	cryptosignAuth = "cryptosign"
	ticketAuth     = "ticket"
	wampCraAuth    = "wampcra"
	anonymousAuth  = "anonymous"

	jsonSerializer    = "json"
	cborSerializer    = "cbor"
	msgpackSerializer = "msgpack"

	readExecutePermission = 0755
)

func getSerializerByName(name string) serialize.Serialization {
	switch name {
	case jsonSerializer:
		return serialize.JSON
	case msgpackSerializer:
		return serialize.MSGPACK
	case cborSerializer:
		return serialize.CBOR
	}
	return -1
}

func selectAuthMethod(privateKey string, ticket string, secret string) string {
	if privateKey != "" && (ticket == "" && secret == "") {
		return cryptosignAuth
	} else if ticket != "" && (privateKey == "" && secret == "") {
		return ticketAuth
	} else if secret != "" && (privateKey == "" && ticket == "") {
		return wampCraAuth
	}

	return anonymousAuth
}

func validateData(sessionCount int, concurrency int, keepAlive int) error {
	if sessionCount < 1 {
		return fmt.Errorf("parallel must be greater than zero")
	}
	if concurrency < 1 {
		return fmt.Errorf("concurrency must be greater than zero")
	}
	if keepAlive < 0 {
		return fmt.Errorf("keepalive interval must be greater than zero")
	}

	return nil
}

// readFromProfile reads section from ini file.
func readFromProfile(profile, filePath string) (*core.ClientInfo, error) {
	clientInfo := &core.ClientInfo{}
	cfg, err := ini.Load(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	section, err := cfg.GetSection(fmt.Sprintf("profile %s", profile))
	if err != nil {
		return nil, fmt.Errorf("unable to read profile: %w", err)
	}

	clientInfo.Url = section.Key("url").String()
	if err = validateURL(clientInfo.Url); err != nil {
		return nil, err
	}

	clientInfo.Realm = section.Key("realm").String()
	if err = validateRealm(clientInfo.Realm); err != nil {
		return nil, err
	}

	serializer := section.Key("serializer").String()
	switch serializer {
	case jsonSerializer, msgpackSerializer, cborSerializer:
		clientInfo.Serializer = getSerializerByName(serializer)
	case "":
		// default to json if none was provided
		clientInfo.Serializer = getSerializerByName("json")
	default:
		return nil, fmt.Errorf("serailizer must be json, msgpack or cbor")
	}

	// FIXME: validate not empty
	clientInfo.Authid = section.Key("authid").String()
	clientInfo.Authrole = section.Key("authrole").String()

	clientInfo.AuthMethod = section.Key("authmethod").String()
	if err = validateAuthMethod(clientInfo.AuthMethod); err != nil {
		return nil, err
	}
	if clientInfo.AuthMethod == cryptosignAuth {
		clientInfo.PrivateKey = section.Key("private-key").String()
		if err = validatePrivateKey(clientInfo.PrivateKey); err != nil {
			return nil, err
		}
	} else if clientInfo.AuthMethod == ticketAuth {
		clientInfo.Ticket = section.Key("ticket").String()
		if clientInfo.Ticket == "" {
			return nil, fmt.Errorf("ticket is required for ticket authentication")
		}
	} else if clientInfo.AuthMethod == wampCraAuth {
		clientInfo.Secret = section.Key("secret").String()
		if clientInfo.Secret == "" {
			return nil, fmt.Errorf("secret is required for wampcra authentication")
		}
	}

	return clientInfo, nil
}

// validateURL returns error if given string is not valid url.
func validateURL(s string) error {
	if s == "" {
		return fmt.Errorf("invalid url: url must not be empty")
	}
	parse, err := url.ParseRequestURI(s)
	if err != nil {
		return err
	}
	switch parse.Scheme {
	case "ws", "rs", "tcp", "wss", "rss", "tcps":
		return nil
	default:
		return fmt.Errorf("invalid url: scheme must be 'ws', 'rs' or 'tcp'")
	}
}

// validateSerializer returns error if given string is not a valid serializer.
func validateSerializer(s string) error {
	switch s {
	case jsonSerializer, msgpackSerializer, cborSerializer:
		return nil
	default:
		return fmt.Errorf("invalid serializer: serailizer must be 'json', 'msgpack' or 'cbor'")
	}
}

// validateAuthMethod returns error if given string is not a valid authmethod.
func validateAuthMethod(s string) error {
	switch s {
	case anonymousAuth, ticketAuth, wampCraAuth, cryptosignAuth:
		return nil
	default:
		return fmt.Errorf("invalid authmethod: value must be one of 'anonymous', 'ticket', " +
			"'wampcra', 'cryptosign'")
	}
}

// validateRealm returns error if given string is empty or not a valid realm.
func validateRealm(s string) error {
	uri := wamp.URI(s)
	if !uri.ValidURI(false, wamp.MatchExact) {
		return fmt.Errorf("invalid realm: is unset or not a valid uri")
	}
	return nil
}

// validatePrivateKey return error if given string is not a valid private key.
func validatePrivateKey(s string) error {
	privateKeyRaw, err := hex.DecodeString(s)
	if err != nil {
		return fmt.Errorf("invalid private key: %w", err)
	}
	if len(privateKeyRaw) != 64 && len(privateKeyRaw) != 32 {
		return fmt.Errorf("invalid private key: private key must have length of 32 or 64")
	}
	return nil
}

// getInputFromUser ask user for input if not present in clientInfo.
func getInputFromUser(serializer string, clientInfo *core.ClientInfo) (*core.ClientInfo, string, error) {
	var writer = os.Stdout
	var reader = os.Stdin
	if clientInfo.Url == "" || clientInfo.Url == "ws://localhost:8080/ws" {
		inputUrl, err := askForInput(reader, writer, &inputOptions{
			Query:        "Enter url",
			DefaultVal:   "ws://localhost:8080/ws",
			Required:     true,
			Loop:         true,
			ValidateFunc: validateURL,
		})
		if err != nil {
			return nil, "", err
		}
		clientInfo.Url = inputUrl
	}

	if clientInfo.Realm == "" || clientInfo.Realm == "realm1" {
		inputRealm, err := askForInput(reader, writer, &inputOptions{
			Query:        "Enter realm",
			DefaultVal:   "realm1",
			Required:     true,
			Loop:         true,
			ValidateFunc: validateRealm,
		})
		if err != nil {
			return nil, "", err
		}
		clientInfo.Realm = inputRealm
	}

	if serializer == "json" || serializer == "" {
		inputSerializer, err := askForInput(reader, writer, &inputOptions{
			Query:        "Enter serializer(supported are 'json', 'msgpack', 'cbor')",
			DefaultVal:   "json",
			Required:     true,
			Loop:         true,
			ValidateFunc: validateSerializer,
		})
		if err != nil {
			return nil, serializer, err
		}
		serializer = inputSerializer
	}

	if clientInfo.Authid == "" {
		inputAuthid, err := askForInput(reader, writer, &inputOptions{
			Query:        "Enter authid",
			DefaultVal:   "",
			Required:     false,
			Loop:         false,
			ValidateFunc: nil,
		})
		if err != nil {
			return nil, serializer, err
		}
		clientInfo.Authid = inputAuthid
	}

	if clientInfo.AuthMethod == "" || clientInfo.AuthMethod == "anonymous" {
		inputAuthMethod, err := askForInput(reader, writer, &inputOptions{
			Query:        "Enter authmethod(supported are 'anonymous', 'ticket', 'wampcra', 'cryptosign')",
			DefaultVal:   "anonymous",
			Required:     true,
			Loop:         true,
			ValidateFunc: validateAuthMethod,
		})
		clientInfo.AuthMethod = inputAuthMethod
		if err != nil {
			return nil, serializer, err
		}
	}

	switch clientInfo.AuthMethod {
	case ticketAuth:
		if clientInfo.Ticket == "" {
			inputTicket, err := askForInput(reader, writer, &inputOptions{
				Query:        "Enter ticket",
				DefaultVal:   "",
				Required:     true,
				Loop:         true,
				ValidateFunc: nil,
			})
			if err != nil {
				return nil, serializer, err
			}
			clientInfo.Ticket = inputTicket
		}
	case wampCraAuth:
		if clientInfo.Secret == "" {
			inputSecret, err := askForInput(reader, writer, &inputOptions{
				Query:        "Enter secret",
				DefaultVal:   "",
				Required:     true,
				Loop:         true,
				ValidateFunc: nil,
			})
			if err != nil {
				return nil, serializer, err
			}
			clientInfo.Secret = inputSecret
		}
	case cryptosignAuth:
		if clientInfo.PrivateKey == "" {
			inputPrivateKey, err := askForInput(reader, writer, &inputOptions{
				Query:        "Enter private-key",
				DefaultVal:   "",
				Required:     true,
				Loop:         true,
				ValidateFunc: validatePrivateKey,
			})
			if err != nil {
				return nil, serializer, err
			}
			clientInfo.PrivateKey = inputPrivateKey
		}
	}

	return clientInfo, serializer, nil
}

// writeProfile write section in ini file.
func writeProfile(sectionName, serializerStr, filePath string, clientInfo *core.ClientInfo) error {
	// load from ini file
	cfg, err := ini.Load(filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("fail to load config: %w", err)
		}
		// no config, use an empty one
		cfg = ini.Empty()
	}

	// create a new section
	section, err := cfg.NewSection(fmt.Sprintf("profile %s", sectionName))
	if err != nil {
		return fmt.Errorf("fail to create config: %w", err)
	}

	for _, data := range []struct {
		key   string
		value string
	}{
		{"url", clientInfo.Url},
		{"realm", clientInfo.Realm},
		{"serializer", serializerStr},
		{"authid", clientInfo.Authid},
		{"authrole", clientInfo.Authrole},
		{"authmethod", clientInfo.AuthMethod},
		{"private-key", clientInfo.PrivateKey},
		{"ticket", clientInfo.Ticket},
		{"secret", clientInfo.Secret},
	} {
		if data.value != "" {
			// create new key in the section
			if _, err = section.NewKey(data.key, data.value); err != nil {
				return fmt.Errorf("error in creating key: %w", err)
			}
		}
	}

	if err = os.MkdirAll(filepath.Dir(filePath), readExecutePermission); err != nil {
		return fmt.Errorf("cannot create directory: %w", err)
	}
	return cfg.SaveTo(filePath)
}

type inputOptions struct {
	Query        string
	DefaultVal   string
	Required     bool
	Loop         bool
	ValidateFunc func(string) error
}

// askForInput asks the user for input for the given Query.
// If Loop is true, it continues to ask until it receives valid input.
func askForInput(reader io.Reader, writer io.Writer, options *inputOptions) (string, error) {
	// resultStr and resultErr are return val of this function
	var resultStr string
	var resultErr error

	for {
		// Display the query to the user.
		fmt.Fprintf(writer, "%s: ", options.Query)

		// Display default value if not empty.
		if options.DefaultVal != "" {
			fmt.Fprintf(writer, "(Default is %s): ", options.DefaultVal)
		}

		// Read user input from UI.Reader.
		line, err := read(bufio.NewReader(reader))
		if err != nil {
			resultErr = err
			break
		}

		// line is empty but a default is provided, so use it
		if line == "" && options.DefaultVal != "" {
			resultStr = options.DefaultVal
			break
		}

		if line == "" && options.Required {
			if !options.Loop {
				resultErr = fmt.Errorf("no input and no default value")
				break
			}

			fmt.Fprintf(writer, "Input must not be empty.\n")
			continue
		}

		// validate input by custom function
		if options.ValidateFunc != nil {
			if err = options.ValidateFunc(line); err != nil {
				if !options.Loop {
					resultErr = err
					break
				}

				fmt.Fprintf(writer, "Invalid input: %v\n", err)
				continue
			}
		}

		// Reach here means it gets ideal input.
		resultStr = line
		break
	}
	return resultStr, resultErr
}

// read reads input from reader.
func read(bReader *bufio.Reader) (string, error) {
	var resultStr string
	var resultErr error

	line, err := bReader.ReadString('\n')
	if err != nil && err != io.EOF {
		resultErr = fmt.Errorf("failed to read the input: %w", err)
	}

	resultStr = strings.TrimSuffix(line, "\n")
	return resultStr, resultErr
}

func getErrorFromErrorChannel(resC chan error) error {
	var errs []string
	for err := range resC {
		if err != nil {
			errs = append(errs, fmt.Sprintf("- %v", err))
		}
	}
	if len(errs) != 0 {
		return fmt.Errorf("got errors:\n%v", strings.Join(errs, "\n"))
	}
	return nil
}

func connect(clientInfo *core.ClientInfo, keepalive int) (*client.Client, error) {
	var session *client.Client
	var err error

	switch clientInfo.AuthMethod {
	case anonymousAuth:
		if clientInfo.PrivateKey != "" {
			return nil, fmt.Errorf("private key not needed for anonymous auth")
		}
		if clientInfo.Ticket != "" {
			return nil, fmt.Errorf("ticket not needed for anonymous auth")
		}
		if clientInfo.Secret != "" {
			return nil, fmt.Errorf("secret not needed for anonymous auth")
		}
		session, err = core.ConnectAnonymous(clientInfo, keepalive)
	case ticketAuth:
		if clientInfo.Ticket == "" {
			return nil, fmt.Errorf("must provide ticket when authMethod is ticket")
		}
		session, err = core.ConnectTicket(clientInfo, keepalive)
	case wampCraAuth:
		if clientInfo.Secret == "" {
			return nil, fmt.Errorf("must provide secret when authMethod is wampcra")
		}
		session, err = core.ConnectCRA(clientInfo, keepalive)
	case cryptosignAuth:
		if clientInfo.PrivateKey == "" {
			return nil, fmt.Errorf("must provide private key when authMethod is cryptosign")
		}
		session, err = core.ConnectCryptoSign(clientInfo, keepalive)
	}
	if err != nil {
		return nil, err
	}

	return session, err
}

func getSessions(clientInfo *core.ClientInfo, sessionCount int, concurrency int,
	keepalive int) ([]*client.Client, error) {
	var sessions []*client.Client
	var mutex sync.Mutex
	var session *client.Client
	var err error
	wp := workerpool.New(concurrency)
	resC := make(chan error, sessionCount)
	for i := 0; i < sessionCount; i++ {
		wp.Submit(func() {
			session, err = connect(clientInfo, keepalive)
			mutex.Lock()
			sessions = append(sessions, session)
			mutex.Unlock()
			resC <- err
		})
	}

	wp.StopWait()
	close(resC)
	if err = getErrorFromErrorChannel(resC); err != nil {
		return nil, err
	}
	return sessions, nil
}
