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
	"crypto/ed25519"
	"crypto/rand"
	_ "embed" // nolint:gci
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"strings"

	"github.com/gammazero/nexus/v3/client"
	"github.com/gammazero/nexus/v3/wamp"
	"github.com/gammazero/workerpool"
	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/yaml.v3"

	"github.com/s-things/wick/core"
	"github.com/s-things/wick/internal/util" // nolint:gci
)

var (
	//go:embed wick.yaml.in
	sampleConfig []byte
)

const ownerReadWritePermission = 0600

type cmd struct {
	parsed string

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

	join               *kingpin.CmdClause
	joinSessionCount   *int
	concurrentJoin     *int
	logJoinTime        *bool
	keepaliveJoin      *int
	joinNonInteractive *bool

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
	callRawOutArg    *int

	keyGen           *kingpin.CmdClause
	keyGenSaveToFile *bool

	configure *kingpin.CmdClause

	compose      *kingpin.CmdClause
	initCommand  *kingpin.CmdClause
	runCommand   *kingpin.CmdClause
	runTasksFile *string
}

func parseCmd(args []string) (*cmd, error) {
	app := kingpin.New(os.Args[0], "")
	app.Version(versionString).VersionFlag.Short('v')

	joinCommand := app.Command("join", "Start wamp session.")
	subscribeCommand := app.Command("subscribe", "Subscribe a topic.")
	publishCommand := app.Command("publish", "Publish to a topic.")
	registerCommand := app.Command("register", "Register a procedure.")
	callCommand := app.Command("call", "Call a procedure.")
	keyGenCommand := app.Command("keygen", "Generate a WAMP cryptosign ed25519 keypair.")

	composeCommand := app.Command("compose", "")
	runCommand := composeCommand.Command("run", "Execute tasks from 'wick.yml' file.")

	c := &cmd{
		url: app.Flag("url", "WAMP URL to connect to.").
			Default("ws://localhost:8080/ws").Envar("WICK_URL").String(),
		realm: app.Flag("realm", "The WAMP realm to join.").Default("realm1").
			Envar("WICK_REALM").String(),
		authMethod: app.Flag("authmethod", "The authentication method to use.").Envar("WICK_AUTHMETHOD").
			Default("anonymous").Enum("anonymous", "ticket", "wampcra", "cryptosign"),
		authid: app.Flag("authid", "The authid to use, if authenticating.").
			Envar("WICK_AUTHID").String(),
		authrole: app.Flag("authrole", "The authrole to use, if authenticating.").
			Envar("WICK_AUTHROLE").String(),
		secret: app.Flag("secret", "The secret to use in Challenge-Response Auth.").
			Envar("WICK_SECRET").String(),
		privateKey: app.Flag("private-key", "The ed25519 private key hex for cryptosign.").
			Envar("WICK_PRIVATE_KEY").String(),
		ticket: app.Flag("ticket", "The ticket when using ticket authentication.").
			Envar("WICK_TICKET").String(),
		serializer: app.Flag("serializer", "The serializer to use.").Envar("WICK_SERIALIZER").
			Default("json").Enum("json", "msgpack", "cbor"),
		profile: app.Flag("profile", "Get details from in '$HOME/.wick/config'.For default section use 'DEFAULT'.").
			Envar("WICK_PROFILE").String(),
		debug: app.Flag("debug", "Enable debug logging.").Bool(),

		join:             joinCommand,
		joinSessionCount: joinCommand.Flag("parallel", "Join requested number of wamp sessions.").Default("1").Int(),
		concurrentJoin: joinCommand.Flag("concurrency", "Join wamp session concurrently. "+
			"Only effective when called with --parallel.").Default("1").Int(),
		logJoinTime:        joinCommand.Flag("time", "Log session join time").Bool(),
		keepaliveJoin:      joinCommand.Flag("keepalive", "Interval between websocket pings.").Default("0").Int(),
		joinNonInteractive: joinCommand.Flag("non-interactive", "Join non interactive").Bool(),

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
		callRawOutArg: callCommand.Flag("raw-output-arg",
			"Output the content of this result argument directly to stdout").Default("-1").Int(),

		keyGen:           keyGenCommand,
		keyGenSaveToFile: keyGenCommand.Flag("output-file", "Write keypair to file.").Short('O').Bool(),

		configure: app.Command("configure", "Configure profiles."),

		compose:     composeCommand,
		initCommand: composeCommand.Command("init", "Initialize basic config"),
		runCommand:  runCommand,
		runTasksFile: runCommand.Flag("file-path", "Enter the file path to execute.").Short('f').
			Default("wick.yaml").String(),
	}
	parsed, err := app.Parse(args)
	if err != nil {
		return nil, err
	}
	c.parsed = parsed

	return c, nil
}

func sessionsDone(sessions []*client.Client, allSessionDoneC chan struct{}) {
	wp := workerpool.New(len(sessions))
	for _, session := range sessions {
		sess := session
		wp.Submit(func() {
			<-sess.Done()
			if sess.RouterGoodbye() == nil {
				log.Println("client disconnect unexpectedly")
			} else if sess.RouterGoodbye().Reason == wamp.CloseSystemShutdown {
				log.Print("Router gone, exiting")
			} else {
				log.Println("client disconnected")
			}
		})
	}
	wp.StopWait()
	allSessionDoneC <- struct{}{}
}

func closeSessions(sessions []*client.Client) {
	wp := workerpool.New(len(sessions))
	for _, sess := range sessions {
		s := sess
		wp.Submit(func() {
			// Close the connection to the router
			s.Close()
		})
	}
	wp.StopWait()
}

func callProcedure(c *cmd, sessions []*client.Client) error {
	wp := workerpool.New(*c.concurrentCalls)
	errC := make(chan error, len(sessions))
	for _, session := range sessions {
		sess := session
		wp.Submit(func() {
			opts := core.CallOptions{
				LogTime:     *c.logCallTime,
				RepeatCount: *c.repeatCount,
				DelayCall:   *c.delayCall,
				Concurrency: *c.concurrentCalls,
				WAMPOptions: *c.callOptions,
			}
			if *c.callRawOutArg != -1 {
				opts.RawArgOut = true
				opts.RawArgOutIndex = *c.callRawOutArg
			}
			if err := core.Call(sess, *c.callProcedure, *c.callArgs, *c.callKeywordArgs, opts); err != nil {
				errC <- err
			}
		})
	}
	wp.StopWait()

	return util.ErrorFromErrorChannel(errC)
}

func registerProcedure(c *cmd, sessions []*client.Client) error {
	wp := workerpool.New(*c.concurrentRegister)
	opts := core.RegisterOption{
		Command:     *c.onInvocationCmd,
		Delay:       *c.delay,
		InvokeCount: *c.invokeCount,
		WAMPOptions: *c.registerOptions,
		LogTime:     *c.logRegisterTime,
	}
	errC := make(chan error, len(sessions))
	for _, session := range sessions {
		sess := session
		wp.Submit(func() {
			if err := core.Register(sess, *c.registerProcedure, opts); err != nil {
				errC <- err
			}
		})
	}
	wp.StopWait()

	return util.ErrorFromErrorChannel(errC)
}

func unregisterProcedure(sessions []*client.Client, procedure string) {
	wp := workerpool.New(len(sessions))
	for _, sess := range sessions {
		s := sess
		wp.Submit(func() {
			// Unregister procedure
			_ = s.Unregister(procedure)
		})
	}
	wp.StopWait()
}

func publishTopic(c *cmd, sessions []*client.Client) error {
	wp := workerpool.New(*c.concurrentPublish)
	opts := core.PublishOptions{
		WAMPOptions: *c.publishOptions,
		LogTime:     *c.logPublishTime,
		Repeat:      *c.repeatPublish,
		Delay:       *c.delayPublish,
		Concurrency: *c.concurrentPublish,
	}
	errC := make(chan error, len(sessions))
	for _, session := range sessions {
		sess := session
		wp.Submit(func() {
			if err := core.Publish(sess, *c.publishTopic, *c.publishArgs, *c.publishKeywordArgs, opts); err != nil {
				errC <- err
			}
		})
	}
	wp.StopWait()

	return util.ErrorFromErrorChannel(errC)
}

func subscribeTopic(c *cmd, sessions []*client.Client, eventC chan struct{}) error {
	wp := workerpool.New(*c.concurrentSubscribe)
	opts := core.SubscribeOptions{
		WAMPOptions:   *c.subscribeOptions,
		PrintDetails:  *c.subscribePrintDetails,
		LogTime:       *c.logSubscribeTime,
		EventReceived: eventC,
	}
	errC := make(chan error, len(sessions))
	for _, session := range sessions {
		sess := session
		wp.Submit(func() {
			if err := core.Subscribe(sess, *c.subscribeTopic, opts); err != nil {
				errC <- err
			}
		})
	}
	wp.StopWait()
	return util.ErrorFromErrorChannel(errC)
}

func unsubscribeTopic(sessions []*client.Client, topic string) {
	wp := workerpool.New(len(sessions))
	for _, sess := range sessions {
		s := sess
		wp.Submit(func() {
			// Unsubscribe from topic.
			_ = s.Unsubscribe(topic)
		})
	}
	wp.StopWait()
}

func startREPL(sessions []*client.Client) {
	reader := bufio.NewScanner(os.Stdin)
	fmt.Print("wick> ")
	for reader.Scan() {
		text := reader.Text()
		if strings.TrimSpace(text) == "exit" {
			closeSessions(sessions)
		}
		a := strings.Fields(text)
		if len(a) == 0 {
			fmt.Print("wick> ")
			continue
		}
		command, err := parseCmd(a)
		if err != nil {
			log.Errorln(err)
			fmt.Print("wick> ")
			continue
		}
		switch command.parsed {
		case command.call.FullCommand():
			if err = callProcedure(command, sessions); err != nil {
				log.Errorln(err)
			}

		case command.register.FullCommand():
			if err = registerProcedure(command, sessions); err != nil {
				log.Errorln(err)
				fmt.Print("wick> ")
				continue
			}
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt)
			<-sigChan
			unregisterProcedure(sessions, *command.registerProcedure)

		case command.publish.FullCommand():
			if err = publishTopic(command, sessions); err != nil {
				log.Errorln(err)
			}

		case command.subscribe.FullCommand():
			// buffer to match the number of sessions, otherwise we'd have to
			// drain the channel
			eventC := make(chan struct{}, len(sessions))
			if err = subscribeTopic(command, sessions, eventC); err != nil {
				log.Errorln(err)
				fmt.Print("wick> ")
				continue
			}

			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt)
			events := 0
			exit := false
			for {
				select {
				case <-eventC:
					events++
					if *command.subscribeEventCount > 0 && events == *command.subscribeEventCount {
						exit = true
					}
				case <-sigChan:
					exit = true
				}
				if exit {
					break
				}
			}
			unsubscribeTopic(sessions, *command.subscribeTopic)

		default:
			log.Errorln("unsupported command: expected one of 'register', 'call', 'subscribe' and 'publish'")
		}
		fmt.Print("wick> ")
	}
	// Close all sessions if we encountered an EOF character
	closeSessions(sessions)
	fmt.Println()
}

const versionString = "0.6.0"

func run(args []string) error {
	c, err := parseCmd(args)
	if err != nil {
		return err
	}

	if *c.debug {
		log.SetLevel(log.DebugLevel)
	}

	if *c.privateKey != "" && *c.ticket != "" {
		return fmt.Errorf("provide only one of private key, ticket or secret")
	} else if *c.ticket != "" && *c.secret != "" {
		return fmt.Errorf("provide only one of private key, ticket or secret")
	} else if *c.privateKey != "" && *c.secret != "" {
		return fmt.Errorf("provide only one of private key, ticket or secret")
	}

	// auto decide authmethod if user didn't explicitly request
	if *c.authMethod == "anonymous" {
		*c.authMethod = selectAuthMethod(*c.privateKey, *c.ticket, *c.secret)
	}

	var clientInfo *core.ClientInfo
	var filePath = os.ExpandEnv("$HOME/.wick/config")
	if *c.profile != "" && c.parsed != c.configure.FullCommand() {
		clientInfo, err = readFromProfile(*c.profile, filePath)
		if err != nil {
			return err
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

	switch c.parsed {
	case c.join.FullCommand():
		sessionOptions := &SessionOptions{
			SessionCount: *c.joinSessionCount,
			Concurrency:  *c.concurrentJoin,
			Keepalive:    *c.keepaliveJoin,
			LogTime:      *c.logJoinTime,
		}
		if err = sessionOptions.validate(); err != nil {
			return err
		}
		if !*c.joinNonInteractive {
			if *c.joinSessionCount != 1 {
				return fmt.Errorf("parallel is allowed for non-interactive join only")
			}
			if *c.concurrentJoin != 1 {
				return fmt.Errorf("concurrency is allowed for non-interactive join only")
			}
			if *c.logJoinTime {
				return fmt.Errorf("time is allowed for non-interactive join only")
			}
		}
		sessions, err := sessionOptions.getSessions(clientInfo)
		if err != nil {
			return err
		}
		defer closeSessions(sessions)

		allSessionsDoneC := make(chan struct{}, len(sessions))
		go sessionsDone(sessions, allSessionsDoneC)

		if !*c.joinNonInteractive {
			go startREPL(sessions)
			<-allSessionsDoneC
		} else {
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt)
			select {
			case <-sigChan:
			case <-allSessionsDoneC:
			}
		}

	case c.subscribe.FullCommand():
		sessionOptions := &SessionOptions{
			SessionCount: *c.subscribeSessionCount,
			Concurrency:  *c.concurrentSubscribe,
			Keepalive:    *c.keepaliveSubscribe,
			LogTime:      *c.logSubscribeTime,
		}
		if err = sessionOptions.validate(); err != nil {
			return err
		}
		if *c.subscribeEventCount < 0 {
			return fmt.Errorf("event count must be greater than zero")
		}

		sessions, err := sessionOptions.getSessions(clientInfo)
		if err != nil {
			return err
		}
		defer closeSessions(sessions)

		// buffer to match the number of sessions, otherwise we'd have to
		// drain the channel
		eventC := make(chan struct{}, len(sessions))
		if err = subscribeTopic(c, sessions, eventC); err != nil {
			return err
		}
		defer unsubscribeTopic(sessions, *c.subscribeTopic)

		allSessionsDoneC := make(chan struct{}, len(sessions))
		go sessionsDone(sessions, allSessionsDoneC)

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
					return nil
				}
			case <-sigChan:
				return nil
			case <-allSessionsDoneC:
				return nil
			}
		}

	case c.publish.FullCommand():
		sessionOptions := &SessionOptions{
			SessionCount: *c.publishSessionCount,
			Concurrency:  *c.concurrentPublish,
			Keepalive:    *c.keepalivePublish,
			LogTime:      *c.logPublishTime,
		}
		if err = sessionOptions.validate(); err != nil {
			return err
		}

		if *c.repeatPublish < 1 {
			return fmt.Errorf("repeat count must be greater than zero")
		}
		sessions, err := sessionOptions.getSessions(clientInfo)
		if err != nil {
			return err
		}

		defer closeSessions(sessions)

		if err = publishTopic(c, sessions); err != nil {
			return err
		}

	case c.register.FullCommand():
		sessionOptions := &SessionOptions{
			SessionCount: *c.registerSessionCount,
			Concurrency:  *c.concurrentRegister,
			Keepalive:    *c.keepaliveRegister,
			LogTime:      *c.logRegisterTime,
		}
		if err = sessionOptions.validate(); err != nil {
			return err
		}

		sessions, err := sessionOptions.getSessions(clientInfo)
		if err != nil {
			return err
		}
		defer closeSessions(sessions)

		if err = registerProcedure(c, sessions); err != nil {
			return err
		}
		defer unregisterProcedure(sessions, *c.registerProcedure)

		// Wait for CTRL-c or client close while handling events.
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt)
		allSessionsDoneC := make(chan struct{}, len(sessions))
		go sessionsDone(sessions, allSessionsDoneC)
		select {
		case <-sigChan:
		case <-allSessionsDoneC:
		}

	case c.call.FullCommand():
		sessionOptions := &SessionOptions{
			SessionCount: *c.callSessionCount,
			Concurrency:  *c.concurrentCalls,
			Keepalive:    *c.keepaliveCall,
			LogTime:      *c.logCallTime,
		}
		if err = sessionOptions.validate(); err != nil {
			return err
		}

		if *c.repeatCount < 1 {
			return fmt.Errorf("repeat count must be greater than zero")
		}

		sessions, err := sessionOptions.getSessions(clientInfo)
		if err != nil {
			return err
		}

		defer closeSessions(sessions)

		if err = callProcedure(c, sessions); err != nil {
			return err
		}

	case c.keyGen.FullCommand():
		pub, pri, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return err
		}
		publicString := hex.EncodeToString(pub)
		privateString := hex.EncodeToString(pri.Seed())
		if *c.keyGenSaveToFile {
			err = ioutil.WriteFile("key", []byte(privateString), os.ModePerm)
			if err != nil {
				return err
			}
			err = ioutil.WriteFile("key.pub", []byte(publicString), os.ModePerm)
			if err != nil {
				return err
			}
		} else {
			fmt.Printf("Public Key: %s\nPrivate Key: %s\n", publicString, privateString)
		}

	case c.configure.FullCommand():
		if *c.profile == "" {
			profile, err := askForInput(os.Stdin, os.Stdout, &inputOptions{
				Query:        "Enter profile name",
				DefaultVal:   "profile1",
				Required:     true,
				Loop:         true,
				ValidateFunc: nil,
			})
			if err != nil {
				return err
			}
			*c.profile = profile
		}
		clientInfo, *c.serializer, err = getInputFromUser(*c.serializer, clientInfo)
		if err != nil {
			return err
		}
		if err = writeProfile(*c.profile, *c.serializer, filePath, clientInfo); err != nil {
			return err
		}

	case c.runCommand.FullCommand():
		yamlFile, err := os.ReadFile(*c.runTasksFile)
		if err != nil {
			return err
		}

		// FIXME: find way to unmarshal into different struct based upon type
		var compose Compose
		err = yaml.Unmarshal(yamlFile, &compose)
		if err != nil {
			return err
		}

		producerSession, err := connect(clientInfo, 0)
		if err != nil {
			return err
		}
		defer producerSession.Close()

		consumerSession, err := connect(clientInfo, 0)
		if err != nil {
			return err
		}
		defer consumerSession.Close()

		if err = executeTasks(compose, producerSession, consumerSession); err != nil {
			return err
		}

	case c.initCommand.FullCommand():
		info, err := os.Stat("wick.yaml")
		if err == nil {
			log.Printf("file %s already exists", info.Name())
			return nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}

		if err = os.WriteFile("wick.yaml", sampleConfig, ownerReadWritePermission); err != nil {
			return fmt.Errorf("unable to write config: %w", err)
		}
	}
	return nil
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatalln(err)
	}
}
