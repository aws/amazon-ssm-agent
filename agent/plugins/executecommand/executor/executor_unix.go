// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License..

// +build darwin freebsd linux netbsd openbsd

// Package executor implements the document and script related functionality for executecommand
package executor

func populateCommand(filename string) (commandName string, commandArgs []string) {
	var shellArgs = []string{"-c"}
	commandName = filename
	commandArgs = shellArgs
	return
}

func appendArgs(shellArgs, args []string, file string) (commandArgs []string) {
	commandArgs = shellArgs
	for _, arg := range args {
		commandArgs = append(commandArgs, arg)
	}
	return
}
