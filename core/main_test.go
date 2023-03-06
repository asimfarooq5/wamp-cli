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

package core_test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/gammazero/nexus/v3/client"
	"github.com/gammazero/nexus/v3/wamp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s-things/wick/core"
	"github.com/s-things/wick/internal/testutil"
)

const (
	testProcedure = "wick.test.procedure"
	testTopic     = "wick.test.topic"
	repeatCount   = 1000
	repeatPublish = 1000
	delay         = 1000
)

func TestRegisterDelay(t *testing.T) {
	rout := testutil.NewTestRouter(t, testutil.TestRealm)
	session := testutil.NewTestClient(t, rout)

	go func() {
		err := core.Register(session, testProcedure, "", delay, 0, nil, false)
		assert.NoError(t, err, fmt.Sprintf("error in registering procedure: %s\n", err))
	}()

	err := session.Unregister(testProcedure)
	assert.Error(t, err, "procedure should register after 1 second")

	time.Sleep(1100 * time.Millisecond)
	err = session.Unregister(testProcedure)
	assert.NoError(t, err, "procedure not even register after delay")
}

func TestRegisterInvokeCount(t *testing.T) {
	invokeCount := 2
	sessionRegister, sessionCall := testutil.ConnectedTestClients(t)

	err := core.Register(sessionRegister, testProcedure, "", 0, invokeCount, nil, false)
	require.NoError(t, err, fmt.Sprintf("error in registering procedure: %s\n", err))

	for i := 0; i < invokeCount; i++ {
		_, err = sessionCall.Call(context.Background(), testProcedure, nil, nil, nil, nil)
		require.NoError(t, err, fmt.Sprintf("error in calling procedure: %s\n", err))
	}
	err = sessionRegister.Unregister(testProcedure)
	require.Error(t, err, "procedure not unregister after invoke-count")
}

func TestRegisterOnInvocationCmd(t *testing.T) {
	sessionRegister, sessionCall := testutil.ConnectedTestClients(t)

	err := core.Register(sessionRegister, testProcedure, "pwd", 0, 0, nil, false)
	require.NoError(t, err, fmt.Sprintf("error in registering procedure: %s\n", err))

	result, err := sessionCall.Call(context.Background(), testProcedure, nil, nil, nil, nil)
	require.NoError(t, err, fmt.Sprintf("error in calling procedure: %s\n", err))

	out, _, _ := core.ShellOut("pwd")
	require.Equal(t, out, result.Arguments[0], "invalid call results")
}

func mockStdout(t *testing.T, mockStdout *os.File) {
	oldStdout := os.Stdout
	t.Cleanup(func() { os.Stdout = oldStdout })
	os.Stdout = mockStdout
}

func TestCallDelayRepeatConcurrency(t *testing.T) {
	sessionRegister, sessionCall := testutil.ConnectedTestClients(t)

	var m sync.Mutex
	iterator := 0
	invocationHandler := func(ctx context.Context, inv *wamp.Invocation) client.InvokeResult {
		m.Lock()
		iterator++
		m.Unlock()
		return client.InvokeResult{Args: wamp.List{wamp.Dict{"foo": "bar"}}}
	}

	err := sessionRegister.Register(testProcedure, invocationHandler, nil)
	require.NoError(t, err, fmt.Sprintf("error in registering procedure: %s\n", err))
	t.Cleanup(func() { sessionRegister.Unregister(testProcedure) })

	t.Run("TestCallDelay", func(t *testing.T) {
		go func() {
			err = core.Call(sessionCall, testProcedure, []string{"Hello", "1"}, nil, core.CallOptions{
				DelayCall: 1000,
			})
			require.NoError(t, err, fmt.Sprintf("error in calling procedure: %s\n", err))
		}()
		m.Lock()
		iter := iterator
		m.Unlock()
		require.Equal(t, 0, iter, "procedure called without delay")
		time.Sleep(1100 * time.Millisecond)

		m.Lock()
		iter = iterator
		m.Unlock()

		require.Equal(t, 1, iter, "procedure not even called after delay")
		iterator = 0
	})

	t.Run("TestCallRepeat", func(t *testing.T) {
		// to avoid logging of call results
		mockStdout(t, os.NewFile(uintptr(syscall.Stdin), os.DevNull))

		err = core.Call(sessionCall, testProcedure, []string{"Hello", "1"}, nil, core.CallOptions{
			RepeatCount: repeatCount,
		})
		require.NoError(t, err, fmt.Sprintf("error in calling procedure: %s\n", err))
		require.Eventually(t, func() bool {
			m.Lock()
			iter := iterator
			m.Unlock()
			require.Equal(t, repeatCount, iter, "procedure not correctly called repeatedly")
			return true
		}, 1*time.Second, 50*time.Millisecond)
	})

}

func TestSubscribe(t *testing.T) {
	rout := testutil.NewTestRouter(t, testutil.TestRealm)

	session := testutil.NewTestClient(t, rout)

	err := core.Subscribe(session, testTopic, nil, false, false, nil)
	require.NoError(t, err, fmt.Sprintf("error in subscribing: %s\n", err))

	err = session.Unsubscribe(testTopic)
	require.NoError(t, err, fmt.Sprintf("error in subscribing: %s\n", err))
}

func TestPublishDelayRepeatConcurrency(t *testing.T) {
	sessionSubscribe, sessionPublish := testutil.ConnectedTestClients(t)

	var m sync.Mutex
	iterator := 0
	eventHandler := func(event *wamp.Event) {
		m.Lock()
		iterator++
		m.Unlock()
	}

	err := sessionSubscribe.Subscribe(testTopic, eventHandler, nil)
	require.NoError(t, err, fmt.Sprintf("error in subscribing topic: %s\n", err))
	t.Cleanup(func() { sessionSubscribe.Unsubscribe(testTopic) })

	t.Run("TestPublishDelay", func(t *testing.T) {
		go func() {
			err = core.Publish(sessionPublish, testTopic, nil, nil, nil, false, 1, 1000, 1)
			require.NoError(t, err, fmt.Sprintf("error in publishing: %s\n", err))
		}()
		m.Lock()
		iter := iterator
		m.Unlock()
		require.Equal(t, 0, iter, "topic published without delay")
		time.Sleep(1100 * time.Millisecond)

		m.Lock()
		iter = iterator
		m.Unlock()
		require.Equal(t, 1, iter, "topic not even published after delay")
		iterator = 0
	})

	t.Run("TestPublishRepeat", func(t *testing.T) {
		err = core.Publish(sessionPublish, testTopic, []string{"Hello", "1"}, nil, nil, false, repeatPublish, 0, 1)
		require.NoError(t, err, fmt.Sprintf("error in publishing topic: %s\n", err))

		require.Eventually(t, func() bool {
			m.Lock()
			iter := iterator
			m.Unlock()
			require.Equal(t, repeatPublish, iter, "topic not correctly publish repeatedly")
			return true
		}, 1*time.Second, 50*time.Millisecond)
	})
}
