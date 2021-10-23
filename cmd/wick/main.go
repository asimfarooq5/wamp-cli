// MIT License
//
// Copyright (c) 2021 CODEBASE
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package main

import (
	"gopkg.in/alecthomas/kingpin.v2"
	"os"

	"github.com/codebasepk/wick/wamp"
)

var (
	url            = kingpin.Flag("url", "WAMP URL to connect to").
		Default("ws://localhost:8080/ws").String()
	realm          = kingpin.Flag("realm", "The WAMP realm to join").Default("realm1").String()
	authmethod     = kingpin.Flag("authmethod","The authentication method to use").
		Enum("ticket", "wampcra", "cryptosign")
	authid         = kingpin.Flag("authid","The authid to use, if authenticating").String()
	authrole       = kingpin.Flag("authrole","The authrole to use, if authenticating").String()
	secret         = kingpin.Flag("secret", "The secret to use in Challenge-Response Auth.").String()
	privateKey     = kingpin.Flag("private-key", "The ed25519 private key hex for cryptosign").String()
	ticket         = kingpin.Flag("ticket", "The ticket when when ticket authentication").String()
	authExtra      = kingpin.Flag("authextra", "The authentication extras").StringMap()

	subscribe      = kingpin.Command("subscribe", "subscribe a topic.")
	subscribeTopic = subscribe.Arg("topic", "Topic to subscribe to").Required().String()

	publish            = kingpin.Command("publish", "Publish to a topic.")
	publishTopic       = publish.Arg("topic", "topic name").Required().String()
	publishArgs        = publish.Arg("args","give the arguments").Strings()
	publishKeywordArgs = publish.Flag("kwarg", "give the keyword arguments").Short('k').StringMap()

	register          = kingpin.Command("register", "Register a procedure.")
	registerProcedure = register.Arg("procedure", "procedure name").Required().String()
	onInvocationCmd   = register.Arg("command", "Shell command to run and return it's output").String()

	call            = kingpin.Command("call", "Call a procedure.")
	callProcedure   = call.Arg("procedure", "Procedure to call").Required().String()
	callArgs        = call.Arg("args","give the arguments").Strings()
	callKeywordArgs = call.Flag("kwarg", "give the keyword arguments").Short('k').StringMap()
)

func main() {
	cmd := kingpin.Parse()

	switch *authmethod {
	case "cryptosign":
		if *privateKey == "" {
			println("Must provide private key when authmethod is cryptosign")
			os.Exit(1)
		}
	case "ticket":
		if *ticket == "" {
			println("Must provide ticket when authmethod is ticket")
			os.Exit(1)
		}
	case "wampcra":
		if *secret == "" {
			println("Must provide secret when authmethod is wampcra")
			os.Exit(1)
		}
	}

	switch cmd {
	case subscribe.FullCommand():
		wamp.Subscribe(*url, *realm, *subscribeTopic, *authid, *secret)
	case publish.FullCommand():
		wamp.Publish(*url, *realm, *publishTopic, *publishArgs, *publishKeywordArgs, *authid, *secret)
	case register.FullCommand():
		wamp.Register(*url, *realm, *registerProcedure, *onInvocationCmd, *authid, *secret)
	case call.FullCommand():
		wamp.Call(*url, *realm, *callProcedure, *callArgs, *callKeywordArgs,*authid, *secret)
	}
}
