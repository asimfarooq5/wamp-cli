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

	"github.com/gammazero/nexus/v3/client"
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
	profile = kingpin.Flag("profile", "").Envar("WICK_PROFILE").String()

	join             = kingpin.Command("join-only", "Start wamp session.")
	joinSessionCount = join.Flag("parallel", "Start requested number of wamp sessions").Default("1").Int()
	concurrentJoin   = join.Flag("concurrency", "Start wamp session concurrently").Default("1").Int()
	logJoinTime      = join.Flag("time", "Log session join time").Bool()
	keepaliveJoin    = join.Flag("keepalive", "interval between websocket pings.").Default("0").Int()

	subscribe             = kingpin.Command("subscribe", "Subscribe a topic.")
	subscribeTopic        = subscribe.Arg("topic", "Topic to subscribe.").Required().String()
	subscribeOptions      = subscribe.Flag("option", "Subscribe option. (May be provided multiple times)").Short('o').StringMap()
	subscribePrintDetails = subscribe.Flag("details", "Print event details.").Bool()
	subscribeEventCount   = subscribe.Flag("event-count", "Wait for a given number of events and exit.").Default("0").Int()
	logSubscribeTime      = subscribe.Flag("time", "Log time to join session and subscribe a topic.").Bool()
	concurrentSubscribe   = subscribe.Flag("concurrency", "Subscribe to topic concurrently. "+
		"Only effective when called with --parallel.").Default("1").Int()
	subscribeSessionCount = subscribe.Flag("parallel", "Start requested number of wamp sessions").Default("1").Int()
	keepaliveSubscribe    = subscribe.Flag("keepalive", "interval between websocket pings.").Default("0").Int()

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
	publishSessionCount = publish.Flag("parallel", "Start requested number of wamp sessions").Default("1").Int()
	keepalivePublish    = publish.Flag("keepalive", "interval between websocket pings.").Default("0").Int()

	register           = kingpin.Command("register", "Register a procedure.")
	registerProcedure  = register.Arg("procedure", "Procedure name.").Required().String()
	onInvocationCmd    = register.Arg("command", "Shell command to run and return it's output.").String()
	delay              = register.Flag("delay", "Register procedure after delay.(in milliseconds)").Int()
	invokeCount        = register.Flag("invoke-count", "Leave session after it's called requested times.").Int()
	registerOptions    = register.Flag("option", "Procedure registration option. (May be provided multiple times)").Short('o').StringMap()
	logRegisterTime    = register.Flag("time", "Log time to join session and register procedure.").Bool()
	concurrentRegister = register.Flag("concurrency", "Register procedure concurrently. "+
		"Only effective when called with --parallel.").Default("1").Int()
	registerSessionCount = register.Flag("parallel", "Start requested number of wamp sessions").Default("1").Int()
	keepaliveRegister    = register.Flag("keepalive", "interval between websocket pings.").Default("0").Int()

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
	callSessionCount = call.Flag("parallel", "Start requested number of wamp sessions").Default("1").Int()
	keepaliveCall    = call.Flag("keepalive", "interval between websocket pings.").Default("0").Int()

	keyGen     = kingpin.Command("keygen", "Generate ed25519 keypair.").Hidden()
	saveToFile = keyGen.Flag("output-file", "Write keys to file.").Short('o').Hidden().Bool()
)

const versionString = "0.5.0"

func connect(logTime bool, keepalive int) (*client.Client, error) {
	var session *client.Client
	var err error
	var startTime int64
	serializerToUse := getSerializerByName(*serializer)

	if logTime {
		startTime = time.Now().UnixMilli()
	}
	switch *authMethod {
	case "anonymous":
		if *privateKey != "" {
			return nil, fmt.Errorf("private key not needed for anonymous auth")
		}
		if *ticket != "" {
			return nil, fmt.Errorf("ticket not needed for anonymous auth")
		}
		if *secret != "" {
			return nil, fmt.Errorf("secret not needed for anonymous auth")
		}
		session, err = core.ConnectAnonymous(*url, *realm, serializerToUse, *authid, *authrole, keepalive)
		if err != nil {
			return nil, err
		}
	case "ticket":
		if *ticket == "" {
			return nil, fmt.Errorf("must provide ticket when authMethod is ticket")
		}
		session, err = core.ConnectTicket(*url, *realm, serializerToUse, *authid, *authrole, *ticket, keepalive)
		if err != nil {
			return nil, err
		}
	case "wampcra":
		if *secret == "" {
			return nil, fmt.Errorf("must provide secret when authMethod is wampcra")
		}
		session, err = core.ConnectCRA(*url, *realm, serializerToUse, *authid, *authrole, *secret, keepalive)
		if err != nil {
			return nil, err
		}
	case "cryptosign":
		if *privateKey == "" {
			return nil, fmt.Errorf("must provide private key when authMethod is cryptosign")
		}
		session, err = core.ConnectCryptoSign(*url, *realm, serializerToUse, *authid, *authrole, *privateKey, keepalive)
		if err != nil {
			return nil, err
		}
	}

	if logTime {
		endTime := time.Now().UnixMilli()
		log.Printf("session joined in %dms\n", endTime-startTime)
	}
	return session, err
}

func getSessions(sessionCount int, concurrency int, logTime bool, keepalive int) ([]*client.Client, error) {
	var sessions []*client.Client
	wp := workerpool.New(concurrency)
	resC := make(chan error, sessionCount)
	for i := 0; i < sessionCount; i++ {
		wp.Submit(func() {
			session, err := connect(logTime, keepalive)
			sessions = append(sessions, session)
			resC <- err
		})
	}

	wp.StopWait()
	if err := getErrorFromErrorChannel(resC); err != nil {
		return nil, err
	}
	return sessions, nil
}

func main() {
	kingpin.Version(versionString).VersionFlag.Short('v')
	cmd := kingpin.Parse()

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

	switch cmd {
	case join.FullCommand():
		var startTime int64
		if *joinSessionCount < 0 {
			log.Fatalln("parallel must be greater than zero")
		}

		if *logJoinTime {
			startTime = time.Now().UnixMilli()
		}
		sessions, err := getSessions(*joinSessionCount, *concurrentJoin, *logJoinTime, *keepaliveJoin)
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
		var startTime int64
		if *subscribeSessionCount < 0 {
			log.Fatalln("parallel must be greater than zero")
		}
		if *subscribeEventCount < 0 {
			log.Fatalln("event count must be greater than zero")
		}
		if *logSubscribeTime {
			startTime = time.Now().UnixMilli()
		}
		sessions, err := getSessions(*subscribeSessionCount, *concurrentSubscribe, *logSubscribeTime, *keepaliveSubscribe)
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
		var startTime int64
		if *repeatPublish < 1 {
			log.Fatalln("repeat count must be greater than zero")
		}
		if *publishSessionCount < 0 {
			log.Fatalln("parallel must be greater than zero")
		}

		if *logPublishTime {
			startTime = time.Now().UnixMilli()
		}
		sessions, err := getSessions(*publishSessionCount, *concurrentPublish, *logPublishTime, *keepalivePublish)
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
		var startTime int64
		if *registerSessionCount < 0 {
			log.Fatalln("parallel must be greater than zero")
		}

		if *logRegisterTime {
			startTime = time.Now().UnixMilli()
		}
		sessions, err := getSessions(*registerSessionCount, *concurrentRegister, *logRegisterTime, *keepaliveRegister)
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
		sessions, err := getSessions(*callSessionCount, *concurrentCalls, *logCallTime, *keepaliveCall)
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
