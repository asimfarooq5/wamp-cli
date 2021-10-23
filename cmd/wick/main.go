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
	"fmt"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/codebasepk/wick/wamp"
)

var (
	url            = kingpin.Flag("url", "WAMP URL to connect to").Default("ws://localhost:8080/ws").String()
	realm          = kingpin.Flag("realm", "The WAMP realm to join").Default("realm1").String()
	authmethod     = kingpin.Flag("authmethod","The authentication method to use").Enum("anonymous", "ticket", "wampcra", "cryptosign")
	authid         = kingpin.Flag("authid","The authid to use, if authenticating").String()
	authrole       = kingpin.Flag("authrole","The authrole to use, if authenticating").String()
	authSecret     = kingpin.Flag("secret", "The secret to use in CRAuthentication.").String()

	subscribe      = kingpin.Command("subscribe", "subscribe a topic.")
	subscribeTopic = subscribe.Arg("topic", "Topic to subscribe to").Required().String()

	publish            = kingpin.Command("publish", "Publish to a topic.")
	publishTopic       = publish.Arg("topic", "topic name").Required().String()
	publishArgs        = publish.Arg("args","give the arguments").Strings()
	publishKeywordArgs = publish.Flag("kwarg", "give the keyword arguments").Short('k').StringMap()

	register          = kingpin.Command("register", "Register a procedure.")
	registerProcedure = register.Arg("procedure", "procedure name").Required().String()
	bashFlagReg       = register.Flag("bash", "enter bash script").Short('b').Strings()
	shellFlagReg      = register.Flag("shell","enter the shell script").Short('s').Strings()
	pythonFlagReg     = register.Flag("python","enter the python script").Short('p').Strings()
	execFlagReg       = register.Flag("exec","execute any file").Short('e').String()

	call            = kingpin.Command("call", "Call a procedure.")
	callProcedure   = call.Arg("procedure", "Procedure to call").Required().String()
	callArgs        = call.Arg("args","give the arguments").Strings()
	callKeywordArgs = call.Flag("kwarg", "give the keyword arguments").Short('k').StringMap()
)

func main() {
	switch kingpin.Parse() {
		case subscribe.FullCommand():
			wamp.Subscribe(*url, *realm, *subscribeTopic, *authid, *authSecret)
		case publish.FullCommand():
			wamp.Publish(*url, *realm, *publishTopic, *publishArgs, *publishKeywordArgs, *authid, *authSecret)
		case register.FullCommand():
			if *bashFlagReg != nil && *shellFlagReg == nil && *pythonFlagReg == nil && *execFlagReg == "" {
				wamp.Register(*url, *realm, *registerProcedure, *bashFlagReg,"bash",*authid, *authSecret)
			} else if *shellFlagReg != nil && *bashFlagReg == nil && *pythonFlagReg ==nil && *execFlagReg == "" {
				wamp.Register(*url, *realm, *registerProcedure, *shellFlagReg,"sh",*authid, *authSecret)
			} else if *pythonFlagReg != nil && *bashFlagReg == nil && *shellFlagReg == nil && *execFlagReg == "" {
				wamp.Register(*url, *realm, *registerProcedure, *pythonFlagReg,"python3",*authid, *authSecret)
			} else if execFlagReg != nil && *bashFlagReg == nil && *shellFlagReg == nil && *pythonFlagReg ==nil {
				wamp.Register(*url, *realm, *registerProcedure, nil, *execFlagReg,*authid, *authSecret)
			} else if *bashFlagReg == nil && *shellFlagReg == nil && *pythonFlagReg == nil && *execFlagReg == "" {
				wamp.Register(*url, *realm, *registerProcedure, nil,"",*authid, *authSecret)
			}else {
				fmt.Println("Please use one type for running script")
			}
		case call.FullCommand():
			wamp.Call(*url, *realm, *callProcedure, *callArgs, *callKeywordArgs,*authid, *authSecret)
	}
}
