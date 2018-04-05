// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/ssm"
	"github.com/aws/aws-sdk-go/aws"
)

var log logger.T

func init() {
	log = logger.DefaultLogger()
	defer log.Flush()
}

type consoleConfig struct {
	Instances map[string]string
}

func main() {
	defer log.Flush()
	commandPtr := flag.String("c", "", "a command")
	scriptFilePtr := flag.String("f", "", "a script file")
	dirPtr := flag.String("d", "", "working directory")
	bucketNamePtr := flag.String("b", "", "bucket name")
	keyPrefixPtr := flag.String("k", "", "bucket key prefix")
	cancelPtr := flag.Bool("cancel", false, "cancel command on key press")
	typePtr := flag.String("type", "", "instance type (windows, ubuntu or aml)")
	instanceIDPtr := flag.String("i", "", "instance id")
	regionPtr := flag.String("r", "us-east-1", "instance region")
	flag.Parse()
	var timeout int64 = 10000
	timeoutPtr := &timeout
	var err error
	err = platform.SetRegion(*regionPtr)
	if err != nil {
		log.Error("please specify the region to use.")
		return
	}

	if *commandPtr == "" && *scriptFilePtr == "" {
		fmt.Println("No commands specified (use either -c or -f).")
		flag.Usage()
		return
	}
	if *keyPrefixPtr == "" {
		keyPrefixPtr = nil
	}
	if *bucketNamePtr == "" {
		bucketNamePtr = nil
		if keyPrefixPtr != nil {
			defaultBucket := "ec2configservice-ssm-logs"
			bucketNamePtr = &defaultBucket
		}
	}

	var cc consoleConfig
	err = jsonutil.UnmarshalFile("integration-cli.json", &cc)
	if err != nil {
		log.Error("error parsing consoleConfig ", err)
		return
	}

	// specific instance is provided use only that
	if *instanceIDPtr != "" {
		cc.Instances = make(map[string]string)
		if *typePtr != "" {
			cc.Instances[*instanceIDPtr] = *typePtr
		} else {
			cc.Instances[*instanceIDPtr] = "aml"
		}
	} else {
		// other wise select or filter from the consoleConfig file list
		if *typePtr != "" {
			for instanceID, instanceType := range cc.Instances {
				if instanceType != *typePtr {
					delete(cc.Instances, instanceID)
				}
			}
			if len(cc.Instances) == 0 {
				log.Error("no instances of type ", *typePtr)
				return
			}
		}
	}

	ssmSvc := ssm.NewService()
	if ssmSvc == nil {
		log.Error("couldn't create ssm service.")
		return
	}

	var instanceIDs []string
	// first get windows instances (bug in SSM if first instance is not win)
	for instanceID, instanceType := range cc.Instances {
		if instanceType == "windows" {
			instanceIDs = append(instanceIDs, instanceID)
		}
	}
	// then get rest of the instances
	for instanceID, instanceType := range cc.Instances {
		if instanceType != "windows" {
			instanceIDs = append(instanceIDs, instanceID)
		}
	}
	docName := "AWS-BETA-RunShellScript"
	if runtime.GOOS == "windows" {
		docName = "AWS-RunPowerShellScript"
	}

	var commands []string
	if *commandPtr != "" {
		commands = []string{*commandPtr}
	} else {
		f, err := os.Open(*scriptFilePtr)
		if err != nil {
			log.Error(err)
			return
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			commands = append(commands, scanner.Text())
		}
	}

	for i := 0; i < len(commands); i++ {
		// escape backslashes
		commands[i] = strings.Replace(commands[i], `\`, `\\`, -1)

		// escape double quotes
		commands[i] = strings.Replace(commands[i], `"`, `\"`, -1)

		// escape single quotes
		//		commands[i] = strings.Replace(commands[i], `'`, `\'`, -1)
	}

	parameters := map[string][]*string{
		"commands":         aws.StringSlice(commands),
		"workingDirectory": aws.StringSlice([]string{*dirPtr}),
	}

	log.Infof("Sending command %v", parameters)

	cmd, err := ssmSvc.SendCommand(log, docName, instanceIDs, parameters, timeoutPtr, bucketNamePtr, keyPrefixPtr)
	if cmd == nil || cmd.Command == nil || cmd.Command.CommandId == nil {
		log.Error("command was not created. Aborting!")
		return
	}

	if *cancelPtr {
		log.Info("Press any key to cancel command")
		var b = make([]byte, 1)
		os.Stdin.Read(b)
		log.Info("Canceling command ")
		ssmSvc.CancelCommand(log, *cmd.Command.CommandId, instanceIDs)
	}

	log.Info("================== Looping for results ================")
	log.Flush()
	time.Sleep(1000 * time.Millisecond)
	for {
		done := true
	inst:
		for instanceID, instanceType := range cc.Instances {
			descr := fmt.Sprintf("%v [%v]", instanceID, instanceType)
			out, err := ssmSvc.ListCommandInvocations(log, instanceID, *cmd.Command.CommandId)
			if err != nil {
				continue
			}
			for _, inv := range out.CommandInvocations {
				if *inv.Status == "Pending" {
					log.Infof("Instance %v is in status %v; waiting some more", descr, *inv.Status)
					done = false
					continue inst
				}

				data, err := json.Marshal(inv)
				if err != nil {
					log.Error(err)
					continue
				}
				log.Debug(jsonutil.Indent(string(data)))

				for _, cp := range inv.CommandPlugins {
					if cp.Output == nil {
						log.Errorf("Output Nil for %v", descr)
						continue
					}

					var o interface{}
					err := json.Unmarshal([]byte(*cp.Output), &o)
					if err != nil {
						log.Errorf("error parsing %v\n err=%v", *cp.Output, err)
					}
					log.Info(descr, " : ", prettyPrint(o, 0))
				}
			}
		}
		if done {
			break
		}
		time.Sleep(3000 * time.Millisecond)
	}

	//	c.Wait()*/
}

func indented(s string, indentLevel int) (res string) {
	for i := 0; i < indentLevel; i++ {
		res += "  "
	}
	return res + s
}

func prettyPrint(input interface{}, indentLevel int) (res string) {
	switch input := input.(type) {
	case string:
		return input

	case []interface{}:
		if len(input) == 0 {
			return "[]"
		}

		//		res = "[\n" + indented("", indentLevel+1)
		for _, v := range input {
			res += prettyPrint(v, indentLevel+1) + "\n"
		}
		//		res += indented("]", indentLevel)
		return res

	case map[string]interface{}:
		if len(input) == 0 {
			return "{}"
		}
		//		res = "{\n"
		res = "\n"
		for k, v := range input {
			res += indented("", indentLevel+1)
			prettyV := prettyPrint(v, indentLevel+1)
			if k == "Stdout" {
				prettyV = outText(prettyV)
			}
			if k == "Stderr" {
				prettyV = errText(prettyV)
			}
			if k == "Error" {
				prettyV = errText(prettyV)
			}
			res += fmt.Sprintf("%v : %v\n", k, prettyV)
		}
		//		res += indented("}", indentLevel)
		return res

	case nil:
		return ""

	default:
		log.Errorf("unexpected object %v of type %T", input, input)
		return ""
	}
}

//Colors In Terminal
const (
	//Clr0 Colors In Terminal
	Clr0 = "\x1b[30;1m"
	ClrR = "\x1b[31;1m"
	ClrG = "\x1b[32;1m"
	ClrY = "\x1b[33;1m"
	ClrB = "\x1b[34;1m"
	ClrM = "\x1b[35;1m"
	ClrC = "\x1b[36;1m"
	ClrW = "\x1b[37;1m"
	ClrN = "\x1b[0m"
)

func outText(text string) (res string) {
	if runtime.GOOS == "linux" {
		return fmt.Sprintf("%s%s%s\n", ClrB, text, ClrN)
	}
	return text
}

func errText(text string) string {
	if runtime.GOOS == "linux" {
		return fmt.Sprintf("%s%s%s\n", ClrR, text, ClrN)
	}
	return text
}
