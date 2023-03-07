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
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/gammazero/nexus/v3/client"
	"github.com/gammazero/nexus/v3/wamp"
	"github.com/gammazero/workerpool"
	log "github.com/sirupsen/logrus"

	"github.com/s-things/wick/internal/util"
)

func connect(url string, cfg client.Config) (*client.Client, error) {
	url = sanitizeURL(url)

	session, err := client.ConnectNet(context.Background(), url, cfg)
	if err != nil {
		return nil, err
	} else {
		log.Debugln("Connected to", url)
		log.Debugf("attached session '%v' to realm '%s' (authid='%s', authrole='%s', authmethod='%s', authprovider='%s')",
			session.ID(), cfg.Realm, session.RealmDetails()["authid"], session.RealmDetails()["authrole"],
			session.RealmDetails()["authmethod"], session.RealmDetails()["authprovider"])
		// XXX: this is broken with some environments, bring back once that's fixed.
		//brokerFeatures := buildStringFromMap(session.RealmDetails()["roles"].(map[string]interface{})["broker"]
		//	.(map[string]interface{})["features"].(map[string]interface{}))
		//dealerFeatures := buildStringFromMap(session.RealmDetails()["roles"].(map[string]interface{})["dealer"]
		//	.(map[string]interface{})["features"].(map[string]interface{}))
		//log.Debugf("broker features(%s), dealer features(%s)", brokerFeatures, dealerFeatures)
	}

	return session, nil
}

func ConnectAnonymous(clientInfo *ClientInfo, keepaliveInterval int) (*client.Client, error) {
	cfg := getAnonymousAuthConfig(clientInfo.Realm, clientInfo.Serializer, clientInfo.Authid,
		clientInfo.Authrole, keepaliveInterval)

	return connect(clientInfo.Url, cfg)
}

func ConnectTicket(clientInfo *ClientInfo, keepaliveInterval int) (*client.Client, error) {
	cfg := getTicketAuthConfig(clientInfo.Realm, clientInfo.Serializer, clientInfo.Authid,
		clientInfo.Authrole, clientInfo.Ticket, keepaliveInterval)

	return connect(clientInfo.Url, cfg)
}

func ConnectCRA(clientInfo *ClientInfo, keepaliveInterval int) (*client.Client, error) {
	cfg := getCRAAuthConfig(clientInfo.Realm, clientInfo.Serializer, clientInfo.Authid,
		clientInfo.Authrole, clientInfo.Secret, keepaliveInterval)

	return connect(clientInfo.Url, cfg)
}

func ConnectCryptoSign(clientInfo *ClientInfo, keepaliveInterval int) (*client.Client, error) {
	cfg, err := getCryptosignAuthConfig(clientInfo.Realm, clientInfo.Serializer, clientInfo.Authid,
		clientInfo.Authrole, clientInfo.PrivateKey, keepaliveInterval)
	if err != nil {
		return nil, err
	}
	return connect(clientInfo.Url, *cfg)
}

type SubscribeOptions struct {
	WAMPOptions   map[string]string
	PrintDetails  bool
	LogTime       bool
	EventReceived chan struct{}
}

func Subscribe(session *client.Client, topic string, opts SubscribeOptions) error {
	eventHandler := func(event *wamp.Event) {
		if opts.PrintDetails {
			output, _ := ArgsKWArgs(event.Arguments, event.ArgumentsKw, event.Details)
			fmt.Println(output)
		} else {
			output, _ := ArgsKWArgs(event.Arguments, event.ArgumentsKw, nil)
			fmt.Println(output)
		}
		if opts.EventReceived != nil {
			opts.EventReceived <- struct{}{}
		}
	}

	var startTime int64
	if opts.LogTime {
		startTime = time.Now().UnixMilli()
	}

	// Subscribe to topic.
	if err := session.Subscribe(topic, eventHandler, dictToWampDict(opts.WAMPOptions)); err != nil {
		return err
	}
	if opts.LogTime {
		endTime := time.Now().UnixMilli()
		log.Printf("Subscribed to topic '%s' in %dms\n", topic, endTime-startTime)
	} else {
		log.Printf("Subscribed to topic '%s'\n", topic)
	}
	return nil
}

func actualPublish(session *client.Client, topic string, args wamp.List, kwargs wamp.Dict, opts PublishOptions) error {
	if opts.Delay > 0 {
		time.Sleep(time.Duration(opts.Delay) * time.Millisecond)
	}

	// Publish to topic.
	if err := session.Publish(topic, dictToWampDict(opts.WAMPOptions), args, kwargs); err != nil {
		return err
	}
	return nil
}

type PublishOptions struct {
	WAMPOptions map[string]string
	LogTime     bool
	Repeat      int
	Delay       int
	Concurrency int
}

func Publish(session *client.Client, topic string, args []string, kwargs map[string]string, opts PublishOptions) error {
	var startTime int64
	if opts.LogTime {
		startTime = time.Now().UnixMilli()
	}

	wp := workerpool.New(opts.Concurrency)
	resC := make(chan error, opts.Repeat)
	for i := 0; i < opts.Repeat; i++ {
		wp.Submit(func() {
			err := actualPublish(session, topic, listToWampList(args), dictToWampDict(kwargs), opts)
			resC <- err
		})
	}
	wp.StopWait()
	close(resC)
	if err := util.ErrorFromErrorChannel(resC); err != nil {
		return err
	}

	if opts.LogTime {
		endTime := time.Now().UnixMilli()
		log.Printf("%d calls took %dms\n", opts.Repeat, endTime-startTime)
	}
	return nil
}

type RegisterOption struct {
	Command     string
	Delay       int
	InvokeCount int
	WAMPOptions map[string]string
	LogTime     bool
}

func Register(session *client.Client, procedure string, opts RegisterOption) error {

	// If the user has called with --invoke-count
	hasMaxInvokeCount := opts.InvokeCount > 0

	if opts.Delay > 0 {
		log.Printf("procedure will be registered after %d milliseconds.\n", opts.Delay)
		time.Sleep(time.Duration(opts.Delay) * time.Millisecond)
	}

	invocationHandler := registerInvocationHandler(session, procedure, opts.Command, opts.InvokeCount, hasMaxInvokeCount)

	var startTime int64
	if opts.LogTime {
		startTime = time.Now().UnixMilli()
	}
	//Register a procedure
	if err := session.Register(procedure, invocationHandler, dictToWampDict(opts.WAMPOptions)); err != nil {
		return err
	}
	if opts.LogTime {
		endTime := time.Now().UnixMilli()
		log.Printf("Registered procedure '%s' in %dms\n", procedure, endTime-startTime)
	} else {
		log.Printf("Registered procedure '%s'\n", procedure)
	}

	return nil
}

func dumpRawArg(args wamp.List, idx int, out io.Writer) error {
	if idx < 0 {
		return fmt.Errorf("cannot dump argument with negative index")
	}
	if idx >= len(args) {
		return fmt.Errorf("cannot dump argument %d with only %d args", idx, len(args))
	}
	idxArg := args[idx]
	if idxArg == nil {
		// could be an end of a binary sequence
		return nil
	}

	switch arg := idxArg.(type) {
	case string:
		_, err := out.Write([]byte(arg))
		return err
	case []byte:
		_, err := out.Write(arg)
		return err
	default:
		return fmt.Errorf("cannot produce raw output of argument of type %T", idxArg)
	}
}

func actuallyCall(session *client.Client, procedure string, args wamp.List, kwargs wamp.Dict,
	opts CallOptions) (*wamp.Result, error) {
	//	delayCall int, callOptions wamp.Dict)
	if opts.DelayCall > 0 {
		time.Sleep(time.Duration(opts.DelayCall) * time.Millisecond)
	}
	var wampOptions wamp.Dict
	if len(opts.WAMPOptions) != 0 {
		wampOptions = dictToWampDict(opts.WAMPOptions)
	}

	var result *wamp.Result
	var err error
	// FIXME use better way to pass error than through cancelable context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if wampOptions["receive_progress"] != nil && wampOptions["receive_progress"] == true {
		result, err = session.Call(ctx, procedure, wampOptions, args, kwargs, func(progress *wamp.Result) {
			if opts.RawArgOut {
				if err := dumpRawArg(progress.Arguments, opts.RawArgOutIndex, os.Stdout); err != nil {
					cancel()
				}
				return
			}
			output, _ := progressArgsKWArgs(progress.Arguments, progress.ArgumentsKw)
			fmt.Println(output)
		})
	} else {
		result, err = session.Call(context.Background(), procedure, wampOptions, args, kwargs, nil)
	}
	if err != nil {
		return nil, err
	} else if result != nil {
		if opts.RawArgOut {
			return nil, dumpRawArg(result.Arguments, opts.RawArgOutIndex, os.Stdout)
		}
		var builder strings.Builder
		if len(result.Arguments) > 0 {
			value, err := encodeToJson(result.Arguments)
			if err != nil {
				return nil, err
			}
			if len(result.ArgumentsKw) > 0 {
				fmt.Fprintf(&builder, "args:\n%s", value)
			} else {
				// this is for backwards compatibility so wick behaves same in case only args are provided
				fmt.Fprintf(&builder, value)
			}
		}

		if len(result.ArgumentsKw) > 0 {
			value, err := encodeToJson(result.ArgumentsKw)
			if err != nil {
				return nil, err
			}
			fmt.Fprintf(&builder, "kwargs:\n%s", value)
		}

		fmt.Println(builder.String())
	}

	return result, nil
}

type CallOptions struct {
	LogTime     bool
	RepeatCount int
	DelayCall   int
	Concurrency int
	WAMPOptions map[string]string
	// RawArgOut when true RawArgOutIndex contains an index
	// of the argument that shall be dumped to stdout
	RawArgOut      bool
	RawArgOutIndex int
}

func Call(session *client.Client, procedure string, args []string, kwargs map[string]string, opts CallOptions) error {
	var startTime int64
	if opts.LogTime {
		startTime = time.Now().UnixMilli()
	}
	if opts.RepeatCount == 0 {
		opts.RepeatCount = 1
	}

	wp := workerpool.New(opts.Concurrency)
	resC := make(chan error, opts.RepeatCount)

	for i := 0; i < opts.RepeatCount; i++ {
		wp.Submit(func() {
			_, err := actuallyCall(session, procedure, listToWampList(args), dictToWampDict(kwargs), opts)
			resC <- err
		})
	}
	wp.StopWait()
	close(resC)
	if err := util.ErrorFromErrorChannel(resC); err != nil {
		return err
	}

	if opts.LogTime {
		endTime := time.Now().UnixMilli()
		log.Printf("%d calls took %dms\n", opts.RepeatCount, endTime-startTime)
	}
	return nil
}
