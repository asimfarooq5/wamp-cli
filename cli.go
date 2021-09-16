package main

import (
	"gopkg.in/alecthomas/kingpin.v2"
)
var (
	subscribeCommand = kingpin.Command("subscribe", "subscribe a topic.")
	URLFlagSub   = subscribeCommand.Flag("url", "url").Required().Short('u').String()
	realmFlagSub = subscribeCommand.Flag("realm", "realmSub").Required().Short('r').String()
	topicArgSub  = subscribeCommand.Arg("topic", "topic name").Required().String()

	publishCommand = kingpin.Command("publish", "publishing a topic.")
	urlFlagPub   = publishCommand.Flag("url", "url").Required().Short('u').String()
	realmFlagPub = publishCommand.Flag("realm", "realmSub").Required().Short('r').String()
	topicArgPub  = publishCommand.Arg("topic", "topic name").Required().String()
	argumentsArgPub = publishCommand.Arg("args","give the arguments").Strings()
	kwargsFlagPub   = publishCommand.Flag("kwarg", "give the keyword arguments").Short('k').StringMap()

	registerCommand = kingpin.Command("register", "registering a procedure.")
	urlFlagReg   = registerCommand.Flag("url", "url").Required().Short('u').String()
	realmFlagReg = registerCommand.Flag("realm", "realmSub").Required().Short('r').String()
	topicArgReg  = registerCommand.Arg("procedure", "procedure name").Required().String()

	callCommand = kingpin.Command("call", "calling a procedure.")
	urlFlagCal   = callCommand.Flag("url", "url").Required().Short('u').String()
	realmFlagCal = callCommand.Flag("realm", "realmSub").Required().Short('r').String()
	topicArgCal  = callCommand.Arg("procedure", "procedure name").Required().String()
	argumentsArgCal = callCommand.Arg("args","give the arguments").Strings()
	kwargsFlagCal   = callCommand.Flag("kwarg", "give the keyword arguments").Short('k').StringMap()
)

func main() {
	switch kingpin.Parse() {
		case "subscribe":
			subscribe(*URLFlagSub, *realmFlagSub, *topicArgSub)
		case "publish":
			publish(*urlFlagPub, *realmFlagPub, *topicArgPub, *argumentsArgPub, *kwargsFlagPub)
		case "register":
			register(*urlFlagReg, *realmFlagReg, *topicArgReg)
		case "call":
			call(*urlFlagCal, *realmFlagCal, *topicArgCal, *argumentsArgCal, *kwargsFlagCal)
	}
}