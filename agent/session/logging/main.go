// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package logging reads byte data from the log file and prints it on the console.
package main

import (
	"bufio"
	"os"
	"strconv"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log/ssmlog"
	"github.com/aws/amazon-ssm-agent/agent/session/logging/console"
)

const totalArguments = 2
const defaultSessionLoggerContextName = "[ssm-session-logger]"

func main() {
	args := os.Args
	argsLen := len(args) - 1

	logger := initializeLogger()
	log := logger.Log()

	// We need two arguments here.
	// First one is the name of the log file to read from.
	// Second one tells us whether to enable virtual terminal processing for newer versions of Windows.
	if argsLen != totalArguments {
		log.Error("Invalid number of arguments received while initializing session logger.")
		return
	}

	file, err := os.Open(args[1])
	if err != nil {
		log.Errorf("Failed to open log file %s", args[1])
		return
	}
	defer file.Close()

	enableVirtualTerminalProcessingForWindows, err := strconv.ParseBool(args[2])
	if err != nil {
		log.Errorf("Invalid argument type received while initializing session logger %s", args[2])
		return
	}

	if enableVirtualTerminalProcessingForWindows {
		if err := console.InitDisplayMode(); err != nil {
			log.Errorf("Encountered an error while initializing console: %v", err)
			return
		}
	}

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanBytes)
	for scanner.Scan() {
		os.Stdout.Write(scanner.Bytes())
	}
}

// initializeLogger initializes context logging for ssm session logger.
func initializeLogger() context.T {
	// initialize a light weight logger, use the default seelog config logger
	logger := ssmlog.SSMLogger(false)

	// initialize appconfig, use default config
	config := appconfig.DefaultConfig()

	return context.Default(logger, config).With(defaultSessionLoggerContextName)
}
