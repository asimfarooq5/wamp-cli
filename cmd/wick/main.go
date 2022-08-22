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
	"time"

	"github.com/gammazero/nexus/v3/client"
	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/s-things/wick/core"
)

var (
	url = kingpin.Flag("url", "WAMP URL to connect to.").
		Default("ws://localhost:8080/ws").Envar("WICK_URL").String()
	realm = kingpin.Flag("realm", "The WAMP realm to join.").Default("realm1").
		Envar("WICK_REALM").String()
	authMethod = kingpin.Flag("authmethod", "The authentication method to use.").Envar("WICK_AUTHMETHOD").
			Default("anonymous").Enum("anonymous", "ticket", "wampcra", "cryptosign")
	authid = kingpin.Flag("authid", "The authid to use, if authenticating.").Envar("WICK_AUTHID").
		String()
	authrole = kingpin.Flag("authrole", "The authrole to use, if authenticating.").
			Envar("WICK_AUTHROLE").String()
	secret = kingpin.Flag("secret", "The secret to use in Challenge-Response Auth.").
		Envar("WICK_SECRET").String()
	privateKey = kingpin.Flag("private-key", "The ed25519 private key hex for cryptosign.").
			Envar("WICK_PRIVATE_KEY").String()
	ticket = kingpin.Flag("ticket", "The ticket when using ticket authentication.").
		Envar("WICK_TICKET").String()
	serializer = kingpin.Flag("serializer", "The serializer to use.").Envar("WICK_SERIALIZER").
			Default("json").Enum("json", "msgpack", "cbor")
	profile = kingpin.Flag("profile", "").Envar("WICK_PROFILE").String()

	subscribe             = kingpin.Command("subscribe", "Subscribe a topic.")
	subscribeTopic        = subscribe.Arg("topic", "Topic to subscribe.").Required().String()
	subscribeOptions      = subscribe.Flag("option", "Subscribe option. (May be provided multiple times)").Short('o').StringMap()
	subscribePrintDetails = subscribe.Flag("details", "Print event details.").Bool()

	publish            = kingpin.Command("publish", "Publish to a topic.")
	publishTopic       = publish.Arg("topic", "Topic to publish.").Required().String()
	publishArgs        = publish.Arg("args", "Provide the arguments.").Strings()
	publishKeywordArgs = publish.Flag("kwarg", "Provide the keyword arguments.").Short('k').StringMap()
	publishOptions     = publish.Flag("option", "Publish option. (May be provided multiple times)").Short('o').StringMap()
	repeatPublish      = publish.Flag("repeat", "Publish to the topic for the provided number of times.").Default("1").Int()
	logPublishTime     = publish.Flag("time", "Log publish return time.").Bool()
	delayPublish       = publish.Flag("delay", "Provide the delay in milliseconds.").Default("0").Int()
	concurrentPublish  = publish.Flag("concurrency", "Publish to the topic concurrently. "+
		"Only effective when called with --repeat.").Default("1").Int()

	register          = kingpin.Command("register", "Register a procedure.")
	registerProcedure = register.Arg("procedure", "Procedure name.").Required().String()
	onInvocationCmd   = register.Arg("command", "Shell command to run and return it's output.").String()
	delay             = register.Flag("delay", "Register procedure after delay.(in milliseconds)").Int()
	invokeCount       = register.Flag("invoke-count", "Leave session after it's called requested times.").Int()
	registerOptions   = register.Flag("option", "Procedure registration option. (May be provided multiple times)").Short('o').StringMap()

	call            = kingpin.Command("call", "Call a procedure.")
	callProcedure   = call.Arg("procedure", "Procedure to call.").Required().String()
	callArgs        = call.Arg("args", "Provide the arguments.").Strings()
	callKeywordArgs = call.Flag("kwarg", "Provide the keyword arguments.").Short('k').StringMap()
	logCallTime     = call.Flag("time", "Log call return time.").Bool()
	repeatCount     = call.Flag("repeat", "Call the procedure for the provided number of times.").Default("1").Int()
	delayCall       = call.Flag("delay", "Provide the delay in milliseconds.").Default("0").Int()
	callOptions     = call.Flag("option", "Procedure call option. (May be provided multiple times)").Short('o').StringMap()
	concurrentCalls = call.Flag("concurrency", "Make concurrent calls without waiting for the result for each to return. "+
		"Only effective when called with --repeat.").Default("1").Int()
)

const versionString = "0.5.0"

func main() {
	kingpin.Version(versionString).VersionFlag.Short('v')
	cmd := kingpin.Parse()

	serializerToUse := getSerializerByName(*serializer)

	if *profile != "" {
		readFromProfile()
	}

	if *privateKey != "" && *ticket != "" {
		log.Fatal("Provide only one of private key, ticket or secret")
	} else if *ticket != "" && *secret != "" {
		log.Fatal("Provide only one of private key, ticket or secret")
	} else if *privateKey != "" && *secret != "" {
		log.Fatal("Provide only one of private key, ticket or secret")
	}

	// auto decide authmethod if user didn't explicitly request
	if *authMethod == "anonymous" {
		*authMethod = selectAuthMethod(*privateKey, *ticket, *secret)
	}

	var session *client.Client
	var err error
	var startTime int64

	if *logCallTime {
		startTime = time.Now().UnixMilli()
	}
	switch *authMethod {
	case "anonymous":
		if *privateKey != "" {
			log.Fatal("Private key not needed for anonymous auth")
		}
		if *ticket != "" {
			log.Fatal("ticket not needed for anonymous auth")
		}
		if *secret != "" {
			log.Fatal("secret not needed for anonymous auth")
		}
		session, err = core.ConnectAnonymous(*url, *realm, serializerToUse, *authid, *authrole)
		if err != nil {
			log.Fatal(err)
		}
	case "ticket":
		if *ticket == "" {
			log.Fatal("Must provide ticket when authMethod is ticket")
		}
		session, err = core.ConnectTicket(*url, *realm, serializerToUse, *authid, *authrole, *ticket)
		if err != nil {
			log.Fatal(err)
		}
	case "wampcra":
		if *secret == "" {
			log.Fatal("Must provide secret when authMethod is wampcra")
		}
		session, err = core.ConnectCRA(*url, *realm, serializerToUse, *authid, *authrole, *secret)
		if err != nil {
			log.Fatal(err)
		}
	case "cryptosign":
		if *privateKey == "" {
			log.Fatal("Must provide private key when authMethod is cryptosign")
		}
		session, err = core.ConnectCryptoSign(*url, *realm, serializerToUse, *authid, *authrole, *privateKey)
		if err != nil {
			log.Fatal(err)
		}
	}

	if *logCallTime {
		endTime := time.Now().UnixMilli()
		log.Printf("session joined in %dms\n", endTime-startTime)
	}

	defer session.Close()

	switch cmd {
	case subscribe.FullCommand():
		err = core.Subscribe(session, *subscribeTopic, *subscribeOptions, *subscribePrintDetails)
		if err != nil {
			log.Fatal(err)
		}
	case publish.FullCommand():
		if *repeatPublish < 1 {
			log.Fatal("repeat count must be greater than zero")
		}
		err = core.Publish(session, *publishTopic, *publishArgs, *publishKeywordArgs, *publishOptions, *logPublishTime,
			*repeatPublish, *delayPublish, *concurrentPublish)
		if err != nil {
			log.Fatal(err)
		}
	case register.FullCommand():
		err = core.Register(session, *registerProcedure, *onInvocationCmd, *delay, *invokeCount, *registerOptions)
		if err != nil {
			log.Fatal(err)
		}
	case call.FullCommand():
		if *repeatCount < 1 {
			log.Fatal("repeat count must be greater than zero")
		}
		err = core.Call(session, *callProcedure, *callArgs, *callKeywordArgs, *logCallTime, *repeatCount, *delayCall,
			*concurrentCalls, *callOptions)
		if err != nil {
			log.Fatal(err)
		}
	}
}
