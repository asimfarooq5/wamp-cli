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
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"time"

	"github.com/gammazero/workerpool"
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
	profile = kingpin.Flag("profile", "Get details from in '$HOME/.wick/config'.For default section use 'DEFAULT'.").Envar("WICK_PROFILE").String()
	debug   = kingpin.Flag("debug", "Enable debug logging.").Bool()

	join             = kingpin.Command("join-only", "Start wamp session.")
	joinSessionCount = join.Flag("parallel", "Join requested number of wamp sessions.").Default("1").Int()
	concurrentJoin   = join.Flag("concurrency", "Join wamp session concurrently. "+
		"Only effective when called with --parallel.").Default("1").Int()
	logJoinTime   = join.Flag("time", "Log session join time").Bool()
	keepaliveJoin = join.Flag("keepalive", "Interval between websocket pings.").Default("0").Int()

	subscribe             = kingpin.Command("subscribe", "Subscribe a topic.")
	subscribeTopic        = subscribe.Arg("topic", "Topic to subscribe.").Required().String()
	subscribeOptions      = subscribe.Flag("option", "Subscribe option. (May be provided multiple times)").Short('o').StringMap()
	subscribePrintDetails = subscribe.Flag("details", "Print event details.").Bool()
	subscribeEventCount   = subscribe.Flag("event-count", "Wait for a given number of events and exit.").Default("0").Int()
	logSubscribeTime      = subscribe.Flag("time", "Log time to join session and subscribe a topic.").Bool()
	concurrentSubscribe   = subscribe.Flag("concurrency", "Subscribe to topic concurrently. "+
		"Only effective when called with --parallel.").Default("1").Int()
	subscribeSessionCount = subscribe.Flag("parallel", "Join requested number of wamp sessions.").Default("1").Int()
	keepaliveSubscribe    = subscribe.Flag("keepalive", "Interval between websocket pings.").Default("0").Int()

	publish            = kingpin.Command("publish", "Publish to a topic.")
	publishTopic       = publish.Arg("topic", "Topic URI to publish on.").Required().String()
	publishArgs        = publish.Arg("args", "Positional arguments for the publish.").Strings()
	publishKeywordArgs = publish.Flag("kwarg", "Keyword argument for the publish (May be provided multiple times).").Short('k').StringMap()
	publishOptions     = publish.Flag("option", "WAMP publish option (May be provided multiple times).").Short('o').StringMap()
	repeatPublish      = publish.Flag("repeat", "Publish to the topic for the provided number of times.").Default("1").Int()
	logPublishTime     = publish.Flag("time", "Log time it took to publish the message. Will only print sane numbers if used with "+
		"WAMP acknowledge=true").Bool()
	delayPublish      = publish.Flag("delay", "Delay (in milliseconds) between subsequent publishes").Default("0").Int()
	concurrentPublish = publish.Flag("concurrency", "Publish to the topic concurrently. "+
		"Only effective when called with --repeat and/or --parallel.").Default("1").Int()
	publishSessionCount = publish.Flag("parallel", "Join requested number of wamp sessions").Default("1").Int()
	keepalivePublish    = publish.Flag("keepalive", "Interval between websocket pings.").Default("0").Int()

	register           = kingpin.Command("register", "Register a procedure.")
	registerProcedure  = register.Arg("procedure", "Procedure URI.").Required().String()
	onInvocationCmd    = register.Arg("command", "Shell command to run and return it's output.").String()
	delay              = register.Flag("delay", "Register the procedure after delay (in milliseconds).").Int()
	invokeCount        = register.Flag("invoke-count", "Leave session after the procedure is invoked the allowed number of times.").Int()
	registerOptions    = register.Flag("option", "WAMP procedure registration option (May be provided multiple times).").Short('o').StringMap()
	logRegisterTime    = register.Flag("time", "Log time to join a session and registration of the procedure.").Bool()
	concurrentRegister = register.Flag("concurrency", "Register procedures concurrently. "+
		"Only effective when called with --parallel.").Default("1").Int()
	registerSessionCount = register.Flag("parallel", "Join requested number of wamp sessions.").Default("1").Int()
	keepaliveRegister    = register.Flag("keepalive", "Interval between websocket pings.").Default("0").Int()

	call            = kingpin.Command("call", "Call a procedure.")
	callProcedure   = call.Arg("procedure", "Procedure to call.").Required().String()
	callArgs        = call.Arg("args", "Positional arguments for the call.").Strings()
	callKeywordArgs = call.Flag("kwarg", "Keyword argument for the call (May be provided multiple times).").Short('k').StringMap()
	logCallTime     = call.Flag("time", "Log call return time (in milliseconds).").Bool()
	repeatCount     = call.Flag("repeat", "Repeatedly call the procedure for the requested number of times.").Default("1").Int()
	delayCall       = call.Flag("delay", "Delay (in milliseconds) between first and subsequent calls.").Default("0").Int()
	callOptions     = call.Flag("option", "WAMP call option (May be provided multiple times).").Short('o').StringMap()
	concurrentCalls = call.Flag("concurrency", "Make concurrent calls without waiting for the result of each to return. "+
		"Only effective when called with --repeat and/or --parallel.").Default("1").Int()
	callSessionCount = call.Flag("parallel", "Join requested number of wamp sessions.").Default("1").Int()
	keepaliveCall    = call.Flag("keepalive", "Interval between websocket pings.").Default("0").Int()

	keyGen     = kingpin.Command("keygen", "Generate a WAMP cryptosign ed25519 keypair.")
	saveToFile = keyGen.Flag("output-file", "Write keypair to file.").Short('O').Bool()
)

const versionString = "0.6.0"

func main() {
	kingpin.Version(versionString).VersionFlag.Short('v')
	cmd := kingpin.Parse()

	if *debug {
		log.SetLevel(log.DebugLevel)
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

	clientInfo := &core.ClientInfo{}
	var err error
	if *profile != "" {
		clientInfo, err = readFromProfile(*profile)
		if err != nil {
			log.Fatalln(err)
		}
	} else {
		clientInfo = &core.ClientInfo{
			Url:        *url,
			Realm:      *realm,
			Serializer: getSerializerByName(*serializer),
			Authid:     *authid,
			Authrole:   *authrole,
			AuthMethod: *authMethod,
			PrivateKey: *privateKey,
			Ticket:     *ticket,
			Secret:     *secret,
		}
	}

	switch cmd {
	case join.FullCommand():
		if err = validateData(*joinSessionCount, *concurrentJoin, *keepaliveJoin); err != nil {
			log.Fatalln(err)
		}

		var startTime int64
		if *logJoinTime {
			startTime = time.Now().UnixMilli()
		}
		sessions, err := getSessions(clientInfo, *joinSessionCount, *concurrentJoin, *keepaliveJoin)
		if err != nil {
			log.Fatalln(err)
		}
		if *logJoinTime {
			endTime := time.Now().UnixMilli()
			log.Printf("%v sessions joined in %dms\n", *joinSessionCount, endTime-startTime)
		}

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

		// Wait for CTRL-c or client close while handling events.
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt)
		for _, session := range sessions {
			select {
			case <-sigChan:
				return
			case <-session.Done():
				log.Print("Router gone, exiting")
			}
		}

	case subscribe.FullCommand():
		if err = validateData(*subscribeSessionCount, *concurrentSubscribe, *keepaliveSubscribe); err != nil {
			log.Fatalln(err)
		}
		if *subscribeEventCount < 0 {
			log.Fatalln("event count must be greater than zero")
		}

		var startTime int64
		if *logSubscribeTime {
			startTime = time.Now().UnixMilli()
		}
		sessions, err := getSessions(clientInfo, *subscribeSessionCount, *concurrentSubscribe, *keepaliveSubscribe)
		if err != nil {
			log.Fatalln(err)
		}
		if *logSubscribeTime {
			endTime := time.Now().UnixMilli()
			log.Printf("%v sessions joined in %dms\n", *subscribeSessionCount, endTime-startTime)
		}

		defer func() {
			wp := workerpool.New(len(sessions))
			for _, sess := range sessions {
				s := sess
				wp.Submit(func() {
					// Unsubscribe from topic.
					s.Unsubscribe(*subscribeTopic)
					// Close the connection to the router
					s.Close()
				})
			}
			wp.StopWait()
		}()

		// buffer to match the number of sessions, otherwise we'd have to
		// drain the channel
		eventC := make(chan struct{}, len(sessions))
		wp := workerpool.New(*concurrentSubscribe)
		for _, session := range sessions {
			sess := session
			wp.Submit(func() {
				err := core.Subscribe(sess, *subscribeTopic, *subscribeOptions,
					*subscribePrintDetails, *logSubscribeTime, eventC)
				if err != nil {
					log.Fatalln(err)
				}
			})
		}
		wp.StopWait()

		// TODO find a nicer way of waiting for all sessions to complete
		allSessionsDoneC := make(chan struct{}, 1)
		go func() {
			for _, session := range sessions {
				<-session.Done()
			}
			log.Print("router gone")
			allSessionsDoneC <- struct{}{}
		}()

		// Wait for CTRL-c or client close while handling events.
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt)
		events := 0
		for {
			select {
			case <-eventC:
				events++
				if *subscribeEventCount > 0 && events == *subscribeEventCount {
					// note this will race against session
					// goroutines possibly trying to send to
					// event channel but nothing is
					// receiving from it
					return
				}
			case <-sigChan:
				return
			case <-allSessionsDoneC:
				return
			}
		}

	case publish.FullCommand():
		if err = validateData(*publishSessionCount, *concurrentPublish, *keepalivePublish); err != nil {
			log.Fatalln(err)
		}

		var startTime int64
		if *repeatPublish < 1 {
			log.Fatalln("repeat count must be greater than zero")
		}
		if *logPublishTime {
			startTime = time.Now().UnixMilli()
		}
		sessions, err := getSessions(clientInfo, *publishSessionCount, *concurrentPublish, *keepalivePublish)
		if err != nil {
			log.Fatalln(err)
		}
		if *logPublishTime {
			endTime := time.Now().UnixMilli()
			log.Printf("%v sessions joined in %dms\n", *publishSessionCount, endTime-startTime)
		}

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

		wp := workerpool.New(*concurrentPublish)
		for _, session := range sessions {
			sess := session
			wp.Submit(func() {
				if err = core.Publish(sess, *publishTopic, *publishArgs, *publishKeywordArgs, *publishOptions, *logPublishTime,
					*repeatPublish, *delayPublish, *concurrentPublish); err != nil {
					log.Fatalln(err)
				}
			})
		}
		wp.StopWait()

	case register.FullCommand():
		if err = validateData(*registerSessionCount, *concurrentRegister, *keepaliveRegister); err != nil {
			log.Fatalln(err)
		}

		var startTime int64
		if *logRegisterTime {
			startTime = time.Now().UnixMilli()
		}
		sessions, err := getSessions(clientInfo, *registerSessionCount, *concurrentRegister, *keepaliveRegister)
		if err != nil {
			log.Fatalln(err)
		}
		if *logRegisterTime {
			endTime := time.Now().UnixMilli()
			log.Printf("%v sessions joined in %dms\n", *registerSessionCount, endTime-startTime)
		}

		defer func() {
			wp := workerpool.New(len(sessions))
			for _, sess := range sessions {
				s := sess
				wp.Submit(func() {
					// Unregister procedure
					s.Unregister(*registerProcedure)
					// Close the connection to the router
					s.Close()
				})
			}
			wp.StopWait()
		}()

		wp := workerpool.New(*concurrentRegister)
		for _, session := range sessions {
			sess := session
			wp.Submit(func() {
				if err = core.Register(sess, *registerProcedure, *onInvocationCmd, *delay, *invokeCount, *registerOptions, *logRegisterTime); err != nil {
					log.Fatalln(err)
				}
			})
		}
		wp.StopWait()

		// Wait for CTRL-c or client close while handling events.
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt)
		for _, session := range sessions {
			select {
			case <-sigChan:
				return
			case <-session.Done():
				log.Print("Router gone, exiting")
			}
		}

	case call.FullCommand():
		if err = validateData(*callSessionCount, *concurrentCalls, *keepaliveCall); err != nil {
			log.Fatalln(err)
		}

		var startTime int64
		if *repeatCount < 1 {
			log.Fatalln("repeat count must be greater than zero")
		}
		if *callSessionCount < 0 {
			log.Fatalln("parallel must be greater than zero")
		}

		if *logCallTime {
			startTime = time.Now().UnixMilli()
		}
		sessions, err := getSessions(clientInfo, *callSessionCount, *concurrentCalls, *keepaliveCall)
		if err != nil {
			log.Fatalln(err)
		}
		if *logCallTime {
			endTime := time.Now().UnixMilli()
			log.Printf("%v sessions joined in %dms\n", *callSessionCount, endTime-startTime)
		}

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

		wp := workerpool.New(*concurrentCalls)
		for _, session := range sessions {
			sess := session
			wp.Submit(func() {
				if err = core.Call(sess, *callProcedure, *callArgs, *callKeywordArgs, *logCallTime, *repeatCount, *delayCall,
					*concurrentCalls, *callOptions); err != nil {
					log.Fatalln(err)
				}
			})
		}
		wp.StopWait()

	case keyGen.FullCommand():
		pub, pri, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			log.Fatalln(err)
		}
		publicString := hex.EncodeToString(pub)
		privateString := hex.EncodeToString(pri.Seed())
		if *saveToFile {
			err = ioutil.WriteFile("key", []byte(privateString), 0600)
			if err != nil {
				log.Fatalln(err)
			}
			err = ioutil.WriteFile("key.pub", []byte(publicString), 0644)
			if err != nil {
				log.Fatalln(err)
			}
		} else {
			fmt.Printf("Public Key: %s\nPrivate Key: %s\n", publicString, privateString)
		}
	}
}
