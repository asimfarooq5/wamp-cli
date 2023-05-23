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

package main_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/gammazero/nexus/v3/client"
	"github.com/stretchr/testify/require"

	main "github.com/s-things/wick/cmd/wick"
	"github.com/s-things/wick/core"
	"github.com/s-things/wick/internal/testutil"
)

func TestJoin(t *testing.T) {
	t.Run("TestValidation", func(t *testing.T) {
		for _, value := range []string{"parallel", "concurrency"} {
			err := main.Run([]string{"join", fmt.Sprintf("--%s", value), "0"})
			require.EqualError(t, err, fmt.Sprintf("%s must be greater than zero", value))
		}

		// test interactive join validation
		err := main.Run([]string{"join", "--time"})
		require.EqualError(t, err, "time is allowed for non-interactive join only")

		err = main.Run([]string{"join", "--concurrency", "2"})
		require.EqualError(t, err, "concurrency is allowed for non-interactive join only")

		err = main.Run([]string{"join", "--parallel", "2"})
		require.EqualError(t, err, "parallel is allowed for non-interactive join only")
	})

	t.Run("TestNonInteractive", func(t *testing.T) {
		rout := testutil.NewTestRouter(t, testutil.TestRealm)
		url := startWsServer(t, rout)
		testSession := testutil.NewTestClient(t, rout)
		m := sync.Mutex{}
		main.MockConnectSession(t, func(clientInfo *core.ClientInfo, keepalive int) (*client.Client, error) {
			m.Lock()
			sess := testSession
			m.Unlock()
			return sess, nil
		})
		doneChan := make(chan bool, 1)
		go func() {
			err := main.Run([]string{"join", "--url", url, "--non-interactive"})
			require.NoError(t, err)
			doneChan <- true
		}()

		m.Lock()
		// close session to exit join command
		err := testSession.Close()
		m.Unlock()
		require.NoError(t, err)
		<-doneChan
	})

}
