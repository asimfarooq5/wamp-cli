package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gammazero/nexus/v3/wamp"
	"log"
	"os/exec"
	"strconv"
)

func listToWampList(args []string) wamp.List {
	var arguments wamp.List

	if args == nil {
		return wamp.List{}
	}

	for _, value := range args {

		var mapJson map[string]interface{}
		var mapList []map[string]interface{}

		if number, errNumber := strconv.Atoi(value); errNumber == nil {
			arguments = append(arguments, number)
		} else if float, errFloat := strconv.ParseFloat(value, 64); errFloat == nil {
			arguments = append(arguments, float)
		} else if boolean, errBoolean := strconv.ParseBool(value); errBoolean == nil {
			arguments = append(arguments, boolean)
		} else if errJson := json.Unmarshal([]byte(value), &mapJson); errJson == nil {
			arguments = append(arguments, mapJson)
		} else if errList := json.Unmarshal([]byte(value), &mapList); errList == nil {
			arguments = append(arguments, mapList)
		} else {
			arguments = append(arguments, value)
		}
	}

	return arguments
}

func dictToWampDict(kwargs map[string]string) wamp.Dict {
	var keywordArguments wamp.Dict = make(map[string]interface{})

	for key, value := range kwargs {

		var mapJson map[string]interface{}
		var mapList []map[string]interface{}

		if number, errNumber := strconv.Atoi(value); errNumber == nil {
			keywordArguments[key] = number
		} else if float, errFloat := strconv.ParseFloat(value, 64); errFloat == nil {
			keywordArguments[key] = float
		} else if boolean, errBoolean := strconv.ParseBool(value); errBoolean == nil {
			keywordArguments[key] = boolean
		} else if errJson := json.Unmarshal([]byte(value), &mapJson); errJson == nil {
			keywordArguments[key] = mapJson
		} else if errList := json.Unmarshal([]byte(value), &mapList); errList == nil {
			keywordArguments[key] = mapList
		} else {
			keywordArguments[key] = value
		}
	}
	return keywordArguments
}

func argsKWArgs(args wamp.List, kwArgs wamp.Dict, details wamp.Dict) {
	if details != nil {
		logger.Println(details)
	}

	if len(args) != 0 {
		fmt.Println("args:")
		jsonString, err := json.MarshalIndent(args, "", "    ")
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(string(jsonString))
	}

	if len(kwArgs) != 0 {
		fmt.Println("kwargs:")
		jsonString, err := json.MarshalIndent(kwArgs, "", "    ")
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(string(jsonString))
	}

	if len(args) == 0 && len(kwArgs) == 0 {
		fmt.Println("args: []")
		fmt.Println("kwargs: {}")
	}
}

func shellOut(command string) (error, string, string) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var cmd *exec.Cmd
	cmd = exec.Command("bash", "-c", command)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return err, stdout.String(), stderr.String()
}
