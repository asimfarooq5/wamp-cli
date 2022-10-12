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
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/gammazero/nexus/v3/client"
	"github.com/gammazero/nexus/v3/transport/serialize"
	"github.com/gammazero/workerpool"
	"gopkg.in/ini.v1"

	"github.com/s-things/wick/core"
)

const (
	cryptosignAuth = "cryptosign"
	ticketAuth     = "ticket"
	wampCraAuth    = "wampcra"
	anonymousAuth  = "anonymous"
)

func getSerializerByName(name string) serialize.Serialization {
	switch name {
	case "json":
		return serialize.JSON
	case "msgpack":
		return serialize.MSGPACK
	case "cbor":
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

func readFromProfile(profile string) (*core.ClientInfo, error) {
	clientInfo := &core.ClientInfo{}
	cfg, err := ini.Load(os.ExpandEnv("$HOME/.wick/config"))
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	section, err := cfg.GetSection(profile)
	if err != nil {
		return nil, fmt.Errorf("unable to read profile: %w", err)
	}

	// FIXME: validate url is not empty and no need to fill it with
	//  a default
	clientInfo.Url = section.Key("url").Validate(func(s string) string {
		if len(s) == 0 {
			return "ws://localhost:8080/ws"
		}
		return s
	})

	// FIXME: validate realm is non empty and is a URI (no space at least)
	clientInfo.Realm = section.Key("realm").Validate(func(s string) string {
		if len(s) == 0 {
			return "realm1"
		}
		return s
	})
	serializer := section.Key("serializer").String()
	switch serializer {
	case "msgpack", "cbor", "json":
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
	// FIXME: validate authmethod is not of invalid type
	clientInfo.AuthMethod = section.Key("authmethod").String()
	if clientInfo.AuthMethod == cryptosignAuth {
		// Validate private key not empty and is valid length
		clientInfo.PrivateKey = section.Key("private-key").String()
	} else if clientInfo.AuthMethod == ticketAuth {
		// validate ticket not empty
		clientInfo.Ticket = section.Key("ticket").String()
	} else if clientInfo.AuthMethod == wampCraAuth {
		// validate not empty
		clientInfo.Secret = section.Key("secret").String()
	}

	return clientInfo, nil
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
