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
	_ "embed" // nolint:gci
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/gammazero/nexus/v3/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	main "github.com/s-things/wick/cmd/wick"
	"github.com/s-things/wick/core"
	"github.com/s-things/wick/internal/testutil" // nolint:gci
)

var (
	//go:embed wick.yaml.in
	sampleConfig []byte
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

func commandOutput(t *testing.T, cmd []string) string {
	rescueStdout := os.Stdout
	r, w, err := os.Pipe()
	assert.NoError(t, err)
	os.Stdout = w

	err = main.Run(cmd)
	require.NoError(t, err)

	w.Close()
	out, err := io.ReadAll(r)
	assert.NoError(t, err)
	os.Stdout = rescueStdout

	return string(out)
}

func TestKeyGen(t *testing.T) {
	t.Run("TestPrintKeys", func(t *testing.T) {
		out := commandOutput(t, []string{"keygen"})
		keys := strings.Split(out, "\n")

		for index, value := range []string{"Public Key: ", "Private Key: "} {
			key := strings.TrimPrefix(keys[index], value)
			keyRaw, err := hex.DecodeString(key)
			require.NoError(t, err)
			require.Len(t, keyRaw, 32)
		}
	})

	t.Run("TestSaveToFile", func(t *testing.T) {
		err := main.Run([]string{"keygen", "-O"})
		require.NoError(t, err)

		for _, fileName := range []string{"key", "key.pub"} {
			stat, err := os.Stat(fileName)
			require.NoError(t, err)
			t.Cleanup(func() { os.Remove(stat.Name()) })

			data, err := os.ReadFile(fileName)
			require.NoError(t, err)
			rawData, err := hex.DecodeString(string(data))
			require.NoError(t, err)
			require.Len(t, rawData, 32)
		}
	})
}

func TestComposeInit(t *testing.T) {
	err := main.Run([]string{"compose", "init"})
	require.NoError(t, err)

	t.Cleanup(func() { os.Remove("wick.yaml") })

	actualConfig, err := os.ReadFile("wick.yaml")
	require.NoError(t, err)
	require.Equal(t, sampleConfig, actualConfig)

	t.Run("TestFileAlreadyExists", func(t *testing.T) {
		//append more text in file
		f, err := os.OpenFile("wick.yaml", os.O_APPEND|os.O_WRONLY, 0644)
		require.NoError(t, err)
		appendStr := `
  - name: publish to a topic
    type: publish
    topic: foo.bar.tick
`
		_, err = f.WriteString(appendStr)
		require.NoError(t, err)
		err = f.Close()
		require.NoError(t, err)

		out := commandOutput(t, []string{"compose", "init"})
		require.Equal(t, "file 'wick.yaml' already exists", out)

		// ensure file is not overwritten
		config, err := os.ReadFile("wick.yaml")
		require.NoError(t, err)
		require.Equal(t, append(sampleConfig, []byte(appendStr)...), config)
	})

}
