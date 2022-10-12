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

type cmd struct {
	url        *string
	realm      *string
	authMethod *string
	authid     *string
	authrole   *string
	secret     *string
	privateKey *string
	ticket     *string
	serializer *string
	profile    *string
	debug      *bool

	join             *kingpin.CmdClause
	joinSessionCount *int
	concurrentJoin   *int
	logJoinTime      *bool
	keepaliveJoin    *int

	subscribe             *kingpin.CmdClause
	subscribeTopic        *string
	subscribeOptions      *map[string]string
	subscribePrintDetails *bool
	subscribeEventCount   *int
	logSubscribeTime      *bool
	concurrentSubscribe   *int
	subscribeSessionCount *int
	keepaliveSubscribe    *int

	publish             *kingpin.CmdClause
	publishTopic        *string
	publishArgs         *[]string
	publishKeywordArgs  *map[string]string
	publishOptions      *map[string]string
	repeatPublish       *int
	logPublishTime      *bool
	delayPublish        *int
	concurrentPublish   *int
	publishSessionCount *int
	keepalivePublish    *int

	register             *kingpin.CmdClause
	registerProcedure    *string
	onInvocationCmd      *string
	delay                *int
	invokeCount          *int
	registerOptions      *map[string]string
	logRegisterTime      *bool
	concurrentRegister   *int
	registerSessionCount *int
	keepaliveRegister    *int

	call             *kingpin.CmdClause
	callProcedure    *string
	callArgs         *[]string
	callKeywordArgs  *map[string]string
	logCallTime      *bool
	repeatCount      *int
	delayCall        *int
	callOptions      *map[string]string
	concurrentCalls  *int
	callSessionCount *int
	keepaliveCall    *int

	keyGen     *kingpin.CmdClause
	saveToFile *bool
}

func parseCmd() (*cmd, string) {
	joinCommand := kingpin.Command("join-only", "Start wamp session.")
	subscribeCommand := kingpin.Command("subscribe", "Subscribe a topic.")
	publishCommand := kingpin.Command("publish", "Publish to a topic.")
	registerCommand := kingpin.Command("register", "Register a procedure.")
	callCommand := kingpin.Command("call", "Call a procedure.")
	keyGenCommand := kingpin.Command("keygen", "Generate a WAMP cryptosign ed25519 keypair.")

	c := &cmd{
		url: kingpin.Flag("url", "WAMP URL to connect to.").
			Default("ws://localhost:8080/ws").Envar("WICK_URL").String(),
		realm: kingpin.Flag("realm", "The WAMP realm to join.").Default("realm1").
			Envar("WICK_REALM").String(),
		authMethod: kingpin.Flag("authmethod", "The authentication method to use.").Envar("WICK_AUTHMETHOD").
			Default("anonymous").Enum("anonymous", "ticket", "wampcra", "cryptosign"),
		authid: kingpin.Flag("authid", "The authid to use, if authenticating.").
			Envar("WICK_AUTHID").String(),
		authrole: kingpin.Flag("authrole", "The authrole to use, if authenticating.").
			Envar("WICK_AUTHROLE").String(),
		secret: kingpin.Flag("secret", "The secret to use in Challenge-Response Auth.").
			Envar("WICK_SECRET").String(),
		privateKey: kingpin.Flag("private-key", "The ed25519 private key hex for cryptosign.").
			Envar("WICK_PRIVATE_KEY").String(),
		ticket: kingpin.Flag("ticket", "The ticket when using ticket authentication.").
			Envar("WICK_TICKET").String(),
		serializer: kingpin.Flag("serializer", "The serializer to use.").Envar("WICK_SERIALIZER").
			Default("json").Enum("json", "msgpack", "cbor"),
		profile: kingpin.Flag("profile", "Get details from in '$HOME/.wick/config'.For default section use 'DEFAULT'.").
			Envar("WICK_PROFILE").String(),
		debug: kingpin.Flag("debug", "Enable debug logging.").Bool(),

		join:             joinCommand,
		joinSessionCount: joinCommand.Flag("parallel", "Join requested number of wamp sessions.").Default("1").Int(),
		concurrentJoin: joinCommand.Flag("concurrency", "Join wamp session concurrently. "+
			"Only effective when called with --parallel.").Default("1").Int(),
		logJoinTime:   joinCommand.Flag("time", "Log session join time").Bool(),
		keepaliveJoin: joinCommand.Flag("keepalive", "Interval between websocket pings.").Default("0").Int(),

		subscribe:      subscribeCommand,
		subscribeTopic: subscribeCommand.Arg("topic", "Topic to subscribe.").Required().String(),
		subscribeOptions: subscribeCommand.Flag("option", "Subscribe option. (May be provided multiple times)").
			Short('o').StringMap(),
		subscribePrintDetails: subscribeCommand.Flag("details", "Print event details.").Bool(),
		subscribeEventCount: subscribeCommand.Flag("event-count", "Wait for a given number of events and exit.").
			Default("0").Int(),
		logSubscribeTime: subscribeCommand.Flag("time", "Log time to join session and subscribe a topic.").Bool(),
		concurrentSubscribe: subscribeCommand.Flag("concurrency", "Subscribe to topic concurrently. "+
			"Only effective when called with --parallel.").Default("1").Int(),
		subscribeSessionCount: subscribeCommand.Flag("parallel", "Join requested number of wamp sessions.").
			Default("1").Int(),
		keepaliveSubscribe: subscribeCommand.Flag("keepalive", "Interval between websocket pings.").
			Default("0").Int(),

		publish:      publishCommand,
		publishTopic: publishCommand.Arg("topic", "Topic URI to publish on.").Required().String(),
		publishArgs: publishCommand.Arg("args", `Positional arguments for the publish.
To enforce value is always a string, send value in quotes e.g."'1'" or '"true"'.`).Strings(),
		publishKeywordArgs: publishCommand.Flag("kwarg", `Keyword argument for the publish.To enforce value
is always a string, send value in quotes e.g."'1'" or '"true"'. (May be provided multiple times)`).
			Short('k').StringMap(),
		publishOptions: publishCommand.Flag("option", "WAMP publish option (May be provided multiple times).").
			Short('o').StringMap(),
		repeatPublish: publishCommand.Flag("repeat", "Publish to the topic for the provided number of times.").
			Default("1").Int(),
		logPublishTime: publishCommand.Flag("time", "Log time it took to publish the message. "+
			"Will only print sane numbers if used with WAMP acknowledge=true").Bool(),
		delayPublish: publishCommand.Flag("delay", "Delay (in milliseconds) between subsequent publishes").
			Default("0").Int(),
		concurrentPublish: publishCommand.Flag("concurrency", "Publish to the topic concurrently. "+
			"Only effective when called with --repeat and/or --parallel.").Default("1").Int(),
		publishSessionCount: publishCommand.Flag("parallel", "Join requested number of wamp sessions").
			Default("1").Int(),
		keepalivePublish: publishCommand.Flag("keepalive", "Interval between websocket pings.").
			Default("0").Int(),

		register:          registerCommand,
		registerProcedure: registerCommand.Arg("procedure", "Procedure URI.").Required().String(),
		onInvocationCmd:   registerCommand.Arg("command", "Shell command to run and return it's output.").String(),
		delay:             registerCommand.Flag("delay", "Register the procedure after delay (in milliseconds).").Int(),
		invokeCount: registerCommand.Flag("invoke-count", "Leave session after the procedure is invoked"+
			" the allowed number of times.").Int(),
		registerOptions: registerCommand.Flag("option", "WAMP procedure registration "+
			"option (May be provided multiple times).").Short('o').StringMap(),
		logRegisterTime: registerCommand.Flag("time", "Log time to join a session and registration of the procedure.").Bool(),
		concurrentRegister: registerCommand.Flag("concurrency", "Register procedures concurrently. "+
			"Only effective when called with --parallel.").Default("1").Int(),
		registerSessionCount: registerCommand.Flag("parallel", "Join requested number of wamp sessions.").
			Default("1").Int(),
		keepaliveRegister: registerCommand.Flag("keepalive", "Interval between websocket pings.").
			Default("0").Int(),

		call:          callCommand,
		callProcedure: callCommand.Arg("procedure", "Procedure to call.").Required().String(),
		callArgs: callCommand.Arg("args", `Positional arguments for the call. To enforce value is always
a string, send value in quotes e.g."'1'" or '"true"'.`).Strings(),
		callKeywordArgs: callCommand.Flag("kwarg", `Keyword argument for the call. To enforce value is always
a string, send value in quotes e.g."'1'" or '"true"'. (May be provided multiple times)`).
			Short('k').StringMap(),
		logCallTime: callCommand.Flag("time", "Log call return time (in milliseconds).").Bool(),
		repeatCount: callCommand.Flag("repeat", "Repeatedly call the procedure for the requested number of times.").
			Default("1").Int(),
		delayCall: callCommand.Flag("delay", "Delay (in milliseconds) between first and subsequent calls.").
			Default("0").Int(),
		callOptions: callCommand.Flag("option", "WAMP call option (May be provided multiple times).").
			Short('o').StringMap(),
		concurrentCalls: callCommand.Flag("concurrency", "Make concurrent calls without waiting for the "+
			"result of each to return. Only effective when called with --repeat and/or --parallel.").
			Default("1").Int(),
		callSessionCount: callCommand.Flag("parallel", "Join requested number of wamp sessions.").
			Default("1").Int(),
		keepaliveCall: callCommand.Flag("keepalive", "Interval between websocket pings.").
			Default("0").Int(),

		keyGen:     keyGenCommand,
		saveToFile: keyGenCommand.Flag("output-file", "Write keypair to file.").Short('O').Bool(),
	}
	return c, kingpin.Parse()
}

const versionString = "0.6.0"

func main() {
	kingpin.Version(versionString).VersionFlag.Short('v')
	c, selectedCommand := parseCmd()

	if *c.debug {
		log.SetLevel(log.DebugLevel)
	}

	if *c.privateKey != "" && *c.ticket != "" {
		log.Fatal("Provide only one of private key, ticket or secret")
	} else if *c.ticket != "" && *c.secret != "" {
		log.Fatal("Provide only one of private key, ticket or secret")
	} else if *c.privateKey != "" && *c.secret != "" {
		log.Fatal("Provide only one of private key, ticket or secret")
	}

	// auto decide authmethod if user didn't explicitly request
	if *c.authMethod == "anonymous" {
		*c.authMethod = selectAuthMethod(*c.privateKey, *c.ticket, *c.secret)
	}

	var clientInfo *core.ClientInfo
	var err error
	if *c.profile != "" {
		clientInfo, err = readFromProfile(*c.profile)
		if err != nil {
			log.Fatalln(err)
		}
	} else {
		clientInfo = &core.ClientInfo{
			Url:        *c.url,
			Realm:      *c.realm,
			Serializer: getSerializerByName(*c.serializer),
			Authid:     *c.authid,
			Authrole:   *c.authrole,
			AuthMethod: *c.authMethod,
			PrivateKey: *c.privateKey,
			Ticket:     *c.ticket,
			Secret:     *c.secret,
		}
	}

	switch selectedCommand {
	case c.join.FullCommand():
		if err = validateData(*c.joinSessionCount, *c.concurrentJoin, *c.keepaliveJoin); err != nil {
			log.Fatalln(err)
		}

		var startTime int64
		if *c.logJoinTime {
			startTime = time.Now().UnixMilli()
		}
		sessions, err := getSessions(clientInfo, *c.joinSessionCount, *c.concurrentJoin, *c.keepaliveJoin)
		if err != nil {
			log.Fatalln(err)
		}
		if *c.logJoinTime {
			endTime := time.Now().UnixMilli()
			log.Printf("%v sessions joined in %dms\n", *c.joinSessionCount, endTime-startTime)
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

	case c.subscribe.FullCommand():
		if err = validateData(*c.subscribeSessionCount, *c.concurrentSubscribe, *c.keepaliveSubscribe); err != nil {
			log.Fatalln(err)
		}
		if *c.subscribeEventCount < 0 {
			log.Fatalln("event count must be greater than zero")
		}

		var startTime int64
		if *c.logSubscribeTime {
			startTime = time.Now().UnixMilli()
		}
		sessions, err := getSessions(clientInfo, *c.subscribeSessionCount, *c.concurrentSubscribe, *c.keepaliveSubscribe)
		if err != nil {
			log.Fatalln(err)
		}
		if *c.logSubscribeTime {
			endTime := time.Now().UnixMilli()
			log.Printf("%v sessions joined in %dms\n", *c.subscribeSessionCount, endTime-startTime)
		}

		defer func() {
			wp := workerpool.New(len(sessions))
			for _, sess := range sessions {
				s := sess
				wp.Submit(func() {
					// Unsubscribe from topic.
					_ = s.Unsubscribe(*c.subscribeTopic)
					// Close the connection to the router
					_ = s.Close()
				})
			}
			wp.StopWait()
		}()

		// buffer to match the number of sessions, otherwise we'd have to
		// drain the channel
		eventC := make(chan struct{}, len(sessions))
		wp := workerpool.New(*c.concurrentSubscribe)
		for _, session := range sessions {
			sess := session
			wp.Submit(func() {
				err := core.Subscribe(sess, *c.subscribeTopic, *c.subscribeOptions,
					*c.subscribePrintDetails, *c.logSubscribeTime, eventC)
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
				if *c.subscribeEventCount > 0 && events == *c.subscribeEventCount {
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

	case c.publish.FullCommand():
		if err = validateData(*c.publishSessionCount, *c.concurrentPublish, *c.keepalivePublish); err != nil {
			log.Fatalln(err)
		}

		var startTime int64
		if *c.repeatPublish < 1 {
			log.Fatalln("repeat count must be greater than zero")
		}
		if *c.logPublishTime {
			startTime = time.Now().UnixMilli()
		}
		sessions, err := getSessions(clientInfo, *c.publishSessionCount, *c.concurrentPublish, *c.keepalivePublish)
		if err != nil {
			log.Fatalln(err)
		}
		if *c.logPublishTime {
			endTime := time.Now().UnixMilli()
			log.Printf("%v sessions joined in %dms\n", *c.publishSessionCount, endTime-startTime)
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

		wp := workerpool.New(*c.concurrentPublish)
		for _, session := range sessions {
			sess := session
			wp.Submit(func() {
				if err = core.Publish(sess, *c.publishTopic, *c.publishArgs, *c.publishKeywordArgs, *c.publishOptions,
					*c.logPublishTime, *c.repeatPublish, *c.delayPublish, *c.concurrentPublish); err != nil {
					log.Fatalln(err)
				}
			})
		}
		wp.StopWait()

	case c.register.FullCommand():
		if err = validateData(*c.registerSessionCount, *c.concurrentRegister, *c.keepaliveRegister); err != nil {
			log.Fatalln(err)
		}

		var startTime int64
		if *c.logRegisterTime {
			startTime = time.Now().UnixMilli()
		}
		sessions, err := getSessions(clientInfo, *c.registerSessionCount, *c.concurrentRegister, *c.keepaliveRegister)
		if err != nil {
			log.Fatalln(err)
		}
		if *c.logRegisterTime {
			endTime := time.Now().UnixMilli()
			log.Printf("%v sessions joined in %dms\n", *c.registerSessionCount, endTime-startTime)
		}

		defer func() {
			wp := workerpool.New(len(sessions))
			for _, sess := range sessions {
				s := sess
				wp.Submit(func() {
					// Unregister procedure
					_ = s.Unregister(*c.registerProcedure)
					// Close the connection to the router
					_ = s.Close()
				})
			}
			wp.StopWait()
		}()

		wp := workerpool.New(*c.concurrentRegister)
		for _, session := range sessions {
			sess := session
			wp.Submit(func() {
				if err = core.Register(
					sess,
					*c.registerProcedure,
					*c.onInvocationCmd,
					*c.delay,
					*c.invokeCount,
					*c.registerOptions,
					*c.logRegisterTime); err != nil {
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

	case c.call.FullCommand():
		if err = validateData(*c.callSessionCount, *c.concurrentCalls, *c.keepaliveCall); err != nil {
			log.Fatalln(err)
		}

		var startTime int64
		if *c.repeatCount < 1 {
			log.Fatalln("repeat count must be greater than zero")
		}
		if *c.callSessionCount < 0 {
			log.Fatalln("parallel must be greater than zero")
		}

		if *c.logCallTime {
			startTime = time.Now().UnixMilli()
		}
		sessions, err := getSessions(clientInfo, *c.callSessionCount, *c.concurrentCalls, *c.keepaliveCall)
		if err != nil {
			log.Fatalln(err)
		}
		if *c.logCallTime {
			endTime := time.Now().UnixMilli()
			log.Printf("%v sessions joined in %dms\n", *c.callSessionCount, endTime-startTime)
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

		wp := workerpool.New(*c.concurrentCalls)
		for _, session := range sessions {
			sess := session
			wp.Submit(func() {
				if err = core.Call(sess, *c.callProcedure, *c.callArgs, *c.callKeywordArgs, *c.logCallTime,
					*c.repeatCount, *c.delayCall, *c.concurrentCalls, *c.callOptions); err != nil {
					log.Fatalln(err)
				}
			})
		}
		wp.StopWait()

	case c.keyGen.FullCommand():
		pub, pri, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			log.Fatalln(err)
		}
		publicString := hex.EncodeToString(pub)
		privateString := hex.EncodeToString(pri.Seed())
		if *c.saveToFile {
			err = ioutil.WriteFile("key", []byte(privateString), os.ModePerm)
			if err != nil {
				log.Fatalln(err)
			}
			err = ioutil.WriteFile("key.pub", []byte(publicString), os.ModePerm)
			if err != nil {
				log.Fatalln(err)
			}
		} else {
			fmt.Printf("Public Key: %s\nPrivate Key: %s\n", publicString, privateString)
		}
	}
}
