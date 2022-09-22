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
	"encoding/json"
	"fmt"
	"time"

	"github.com/gammazero/nexus/v3/client"
	"github.com/gammazero/nexus/v3/transport/serialize"
	"github.com/gammazero/nexus/v3/wamp"
	"github.com/gammazero/workerpool"
	log "github.com/sirupsen/logrus"
)

func connect(url string, cfg client.Config) (*client.Client, error) {

	url = sanitizeURL(url)

	session, err := client.ConnectNet(context.Background(), url, cfg)
	if err != nil {
		return nil, err
	} else {
		// FIXME: use a better logger and only print such messages in debug mode.
		//logger.Println("Connected to ", baseUrl)
	}

	return session, nil
}

func ConnectAnonymous(url string, realm string, serializer serialize.Serialization, authid string,
	authrole string, keepaliveInterval int) (*client.Client, error) {

	cfg := getAnonymousAuthConfig(realm, serializer, authid, authrole, keepaliveInterval)

	return connect(url, cfg)
}

func ConnectTicket(url string, realm string, serializer serialize.Serialization, authid string, authrole string,
	ticket string, keepaliveInterval int) (*client.Client, error) {

	cfg := getTicketAuthConfig(realm, serializer, authid, authrole, ticket, keepaliveInterval)

	return connect(url, cfg)
}

func ConnectCRA(url string, realm string, serializer serialize.Serialization, authid string, authrole string,
	secret string, keepaliveInterval int) (*client.Client, error) {

	cfg := getCRAAuthConfig(realm, serializer, authid, authrole, secret, keepaliveInterval)

	return connect(url, cfg)
}

func ConnectCryptoSign(url string, realm string, serializer serialize.Serialization, authid string, authrole string,
	privateKey string, keepaliveInterval int) (*client.Client, error) {

	cfg := getCryptosignAuthConfig(realm, serializer, authid, authrole, privateKey, keepaliveInterval)

	return connect(url, cfg)
}

func Subscribe(session *client.Client, topic string, subscribeOptions map[string]string,
	printDetails bool, logSubscribeTime bool, eventReceived chan struct{}) error {
	eventHandler := func(event *wamp.Event) {
		if printDetails {
			argsKWArgs(event.Arguments, event.ArgumentsKw, event.Details)
		} else {
			argsKWArgs(event.Arguments, event.ArgumentsKw, nil)
		}
		if eventReceived != nil {
			eventReceived <- struct{}{}
		}
	}

	var startTime int64
	if logSubscribeTime {
		startTime = time.Now().UnixMilli()
	}

	// Subscribe to topic.
	if err := session.Subscribe(topic, eventHandler, dictToWampDict(subscribeOptions)); err != nil {
		return err
	}
	if logSubscribeTime {
		endTime := time.Now().UnixMilli()
		log.Printf("Subscribed to topic '%s' in %dms\n", topic, endTime-startTime)
	} else {
		log.Printf("Subscribed to topic '%s'\n", topic)
	}
	return nil
}

func actualPublish(session *client.Client, topic string, args wamp.List, kwargs wamp.Dict, logPublishTime bool,
	delayPublish int, publishOptions wamp.Dict) error {
	if delayPublish > 0 {
		time.Sleep(time.Duration(delayPublish) * time.Millisecond)
	}

	var startTime int64
	if logPublishTime {
		startTime = time.Now().UnixMilli()
	}

	// Publish to topic.
	if err := session.Publish(topic, publishOptions, args, kwargs); err != nil {
		return err
	}

	if logPublishTime {
		endTime := time.Now().UnixMilli()
		log.Printf("Published to topic %s in %dms\n", topic, endTime-startTime)
	} else {
		log.Printf("Published to topic '%s'\n", topic)
	}
	return nil
}

func Publish(session *client.Client, topic string, args []string, kwargs map[string]string, publishOptions map[string]string,
	logPublishTime bool, repeatPublish int, delayPublish int, concurrency int) error {
	var startTime int64
	if logPublishTime {
		startTime = time.Now().UnixMilli()
	}

	wp := workerpool.New(concurrency)
	resC := make(chan error, repeatPublish)
	for i := 0; i < repeatPublish; i++ {
		wp.Submit(func() {
			err := actualPublish(session, topic, listToWampList(args), dictToWampDict(kwargs),
				logPublishTime, delayPublish, dictToWampDict(publishOptions))
			resC <- err
		})
	}
	wp.StopWait()

	err := getErrorFromErrorChannel(resC)
	if err != nil {
		return err
	}

	if logPublishTime && repeatPublish > 1 {
		endTime := time.Now().UnixMilli()
		log.Printf("%d calls took %dms\n", repeatPublish, endTime-startTime)
	}
	return nil
}

func Register(session *client.Client, procedure string, command string, delay int, invokeCount int,
	registerOptions map[string]string, logRegisterTime bool) error {

	// If the user has called with --invoke-count
	hasMaxInvokeCount := invokeCount > 0

	if delay > 0 {
		log.Printf("procedure will be registered after %d milliseconds.\n", delay)
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}

	invocationHandler := registerInvocationHandler(session, procedure, command, invokeCount, hasMaxInvokeCount)

	var startTime int64
	if logRegisterTime {
		startTime = time.Now().UnixMilli()
	}
	//Register a procedure
	if err := session.Register(procedure, invocationHandler, dictToWampDict(registerOptions)); err != nil {
		return err
	}
	if logRegisterTime {
		endTime := time.Now().UnixMilli()
		log.Printf("Registered procedure '%s' in %dms\n", procedure, endTime-startTime)
	} else {
		log.Printf("Registered procedure '%s'\n", procedure)
	}

	return nil
}

func actuallyCall(session *client.Client, procedure string, args wamp.List, kwargs wamp.Dict,
	logCallTime bool, delayCall int, callOptions wamp.Dict) (*wamp.Result, error) {
	if delayCall > 0 {
		time.Sleep(time.Duration(delayCall) * time.Millisecond)
	}

	var startTime int64
	if logCallTime {
		startTime = time.Now().UnixMilli()
	}

	var result *wamp.Result
	var err error
	if callOptions["receive_progress"] != nil && callOptions["receive_progress"] == true {
		result, err = session.Call(context.Background(), procedure, callOptions, args, kwargs, func(progress *wamp.Result) {
			progressArgsKWArgs(progress.Arguments, progress.ArgumentsKw)
		})
	} else {
		result, err = session.Call(context.Background(), procedure, callOptions, args, kwargs, nil)
	}
	if err != nil {
		return nil, err
	} else if result != nil && len(result.Arguments) > 0 {
		jsonString, err := json.MarshalIndent(result.Arguments, "", "    ")
		if err != nil {
			return nil, err
		}
		fmt.Println(string(jsonString))
	}

	if logCallTime {
		endTime := time.Now().UnixMilli()
		log.Printf("call took %dms\n", endTime-startTime)
	}
	return result, nil
}

func Call(session *client.Client, procedure string, args []string, kwargs map[string]string,
	logCallTime bool, repeatCount int, delayCall int, concurrency int, callOptions map[string]string) error {
	var startTime int64
	if logCallTime {
		startTime = time.Now().UnixMilli()
	}

	wp := workerpool.New(concurrency)
	resC := make(chan error, repeatCount)

	for i := 0; i < repeatCount; i++ {
		wp.Submit(func() {
			_, err := actuallyCall(session, procedure, listToWampList(args), dictToWampDict(kwargs),
				logCallTime, delayCall, dictToWampDict(callOptions))
			resC <- err
		})
	}
	wp.StopWait()

	err := getErrorFromErrorChannel(resC)
	if err != nil {
		return err
	}

	if logCallTime && repeatCount > 1 {
		endTime := time.Now().UnixMilli()
		log.Printf("%d calls took %dms\n", repeatCount, endTime-startTime)
	}
	return nil
}
