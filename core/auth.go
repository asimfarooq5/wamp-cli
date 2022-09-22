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

package core

import (
	"encoding/hex"
	"time"

	"github.com/gammazero/nexus/v3/client"
	"github.com/gammazero/nexus/v3/transport"
	"github.com/gammazero/nexus/v3/transport/serialize"
	"github.com/gammazero/nexus/v3/wamp"
	log "github.com/sirupsen/logrus"
)

func getAnonymousAuthConfig(realm string, serializer serialize.Serialization, authid string,
	authrole string, keepAliveInterval int) client.Config {

	hello := getBaseHello(authid, authrole)

	cfg := client.Config{
		Realm:         realm,
		Logger:        log.New(),
		HelloDetails:  hello,
		Serialization: serializer,
		WsCfg:         transport.WebsocketConfig{KeepAlive: time.Duration(keepAliveInterval) * time.Second},
	}

	return cfg
}

func getTicketAuthConfig(realm string, serializer serialize.Serialization, authid string, authrole string,
	ticket string, keepAliveInterval int) client.Config {

	hello := getBaseHello(authid, authrole)

	cfg := client.Config{
		Realm:        realm,
		Logger:       log.New(),
		HelloDetails: hello,
		AuthHandlers: map[string]client.AuthFunc{
			"ticket": func(c *wamp.Challenge) (string, wamp.Dict) {
				return ticket, wamp.Dict{}
			},
		},
		Serialization: serializer,
		WsCfg:         transport.WebsocketConfig{KeepAlive: time.Duration(keepAliveInterval) * time.Second},
	}

	return cfg
}

func getCRAAuthConfig(realm string, serializer serialize.Serialization, authid string, authrole string,
	secret string, keepAliveInterval int) client.Config {

	hello := getBaseHello(authid, authrole)

	cfg := client.Config{
		Realm:        realm,
		Logger:       log.New(),
		HelloDetails: hello,
		AuthHandlers: map[string]client.AuthFunc{
			"wampcra": handleCRAAuth(secret),
		},
		Serialization: serializer,
		WsCfg:         transport.WebsocketConfig{KeepAlive: time.Duration(keepAliveInterval) * time.Second},
	}

	return cfg
}

func getCryptosignAuthConfig(realm string, serializer serialize.Serialization, authid string, authrole string,
	privateKey string, keepAliveInterval int) client.Config {

	hello := getBaseHello(authid, authrole)

	publicKey, pvk := getKeyPair(privateKey)
	// Extend hello details with pubkey
	hello["authextra"] = wamp.Dict{"pubkey": hex.EncodeToString(publicKey)}

	cfg := client.Config{
		Realm:        realm,
		Logger:       log.New(),
		HelloDetails: hello,
		AuthHandlers: map[string]client.AuthFunc{
			"cryptosign": handleCryptosign(pvk),
		},
		Serialization: serializer,
		WsCfg:         transport.WebsocketConfig{KeepAlive: time.Duration(keepAliveInterval) * time.Second},
	}

	return cfg
}
