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
	"testing"

	"github.com/gammazero/nexus/v3/wamp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s-things/wick/core"
)

const (
	privateKeyHex = "b99067e6e271ae300f3f5d9809fa09288e96f2bcef8dd54b7aabeb4e579d37ef"
)

func TestPrivateHexToKeyPair(t *testing.T) {
	publicKey, privateKey, err := core.GetKeyPair(privateKeyHex)
	require.NoError(t, err)
	assert.NotNil(t, publicKey, "public key is nil")
	assert.NotNil(t, privateKey, "private key is nil")
}

func TestArgsKWArgs(t *testing.T) {
	for _, data := range []struct {
		args           wamp.List
		kwargs         wamp.Dict
		details        wamp.Dict
		expectedResult string
	}{
		{wamp.List{"test", 1, true, "1.0"}, wamp.Dict{}, nil, `args:
[
    "test",
    1,
    true,
    "1.0"
]`},
		{wamp.List{}, wamp.Dict{"key": "value", "key2": 1, "key3": false}, nil, `kwargs:
{
    "key": "value",
    "key2": 1,
    "key3": false
}`},
		{wamp.List{"test", 1, true, "1.0"}, wamp.Dict{"key": "value", "key2": 1, "key3": false}, nil, `args:
[
    "test",
    1,
    true,
    "1.0"
]kwargs:
{
    "key": "value",
    "key2": 1,
    "key3": false
}`},
		{wamp.List{"test", 1, true, "1.0"}, wamp.Dict{"key": "value", "key2": 1, "key3": false},
			wamp.Dict{"details": "wamp details"}, `details:{
    "details": "wamp details"
}
args:
[
    "test",
    1,
    true,
    "1.0"
]kwargs:
{
    "key": "value",
    "key2": 1,
    "key3": false
}`},
		{wamp.List{}, wamp.Dict{}, wamp.Dict{"details": "wamp details"}, `details:{
    "details": "wamp details"
}
`},
		{wamp.List{}, wamp.Dict{}, nil, `args: []
kwargs: {}`},
	} {
		outputString, err := core.ArgsKWArgs(data.args, data.kwargs, data.details)
		require.NoError(t, err)
		assert.Equal(t, outputString, data.expectedResult)
	}
}

func TestProgressArgsKWArgs(t *testing.T) {
	for _, data := range []struct {
		args           wamp.List
		kwargs         wamp.Dict
		expectedResult string
	}{
		{
			wamp.List{"test", 1, true, "1.0"},
			wamp.Dict{},
			`args: ["test",1,true,"1.0"]`},
		{
			wamp.List{},
			wamp.Dict{"key": "value", "key2": 1, "key3": false},
			`kwargs: {"key":"value","key2":1,"key3":false}`},
		{
			wamp.List{"test", 1, true, "1.0"},
			wamp.Dict{"key": "value", "key2": 1, "key3": false},
			`args: ["test",1,true,"1.0"]kwargs: {"key":"value","key2":1,"key3":false}`},
		{wamp.List{}, wamp.Dict{}, `args: [] kwargs: {}`},
	} {
		outputString, err := core.ProgressArgsKWArgs(data.args, data.kwargs)
		require.NoError(t, err)
		assert.Equal(t, outputString, data.expectedResult)
	}
}

func TestUrlSanitization(t *testing.T) {
	for _, data := range []struct {
		url          string
		sanitizedUrl string
	}{
		{"rs://localhost:8080/", "tcp://localhost:8080/"},
		{"rss://localhost:8080/", "tcp://localhost:8080/"},
	} {
		url := core.SanitizeURL(data.url)
		assert.Equal(t, data.sanitizedUrl, url, "url sanitization failed")
	}
}

func TestListToWampList(t *testing.T) {
	inputList := []string{
		"string", "1", "1.1", "true",
		`"123"`, `'123'`, `"true"`,
		// JSON array, object, array of objects
		`["group_1","group_2", 1.1, true]`,
		`{"firstKey":"value", "secondKey":2.1}`,
		`[{"firstKey":"value", "secondKey":2.1}, {"firstKey":"value", "secondKey":2.1}]`,
	}

	expectedList := wamp.List{
		"string", 1, 1.1, true,
		"123", "123", "true",
		// converted from JSON
		[]interface{}{"group_1", "group_2", 1.1, true},
		map[string]interface{}{"firstKey": "value", "secondKey": 2.1},
		[]map[string]interface{}{
			{"firstKey": "value", "secondKey": 2.1},
			{"firstKey": "value", "secondKey": 2.1},
		},
	}

	wampList := core.ListToWampList(inputList)
	assert.Equal(t, expectedList, wampList, "error in list conversion")
}

func TestDictToWampDict(t *testing.T) {
	inputDict := map[string]string{
		"string": "string", "int": "1", "float": "1.1", "bool": "true",
		"stringNumber": `""123"`, "stringFloat": `'1.23'`, "stringBool": `"true"`,
		"list":     `["group_1","group_2", 1.1, true]`,
		"json":     `{"firstKey":"value", "secondKey":2.2}`,
		"jsonList": `[{"firstKey":"value", "secondKey":2.2}, {"firstKey":"value", "secondKey":2.2}]`,
	}

	expectedDict := wamp.Dict{
		"string": "string", "int": 1, "float": 1.1, "bool": true,
		"stringNumber": `"123`, "stringFloat": "1.23", "stringBool": "true",
		"list": []interface{}{
			"group_1", "group_2", 1.1, true,
		},
		"json": map[string]interface{}{"firstKey": "value", "secondKey": 2.2},
		"jsonList": []map[string]interface{}{
			{"firstKey": "value", "secondKey": 2.2},
			{"firstKey": "value", "secondKey": 2.2},
		},
	}

	wampDict := core.DictToWampDict(inputDict)
	assert.Equal(t, expectedDict, wampDict, "error in dict conversion")
}
