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
)

func main() {
	args := os.Args
	argsLen := len(args) - 1
	if argsLen != 1 {
		panic("Arguments mismatch error.")
	}

	file, err := os.Open(args[1])
	if err != nil {
		panic("Failed to open log file.")
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanBytes)
	for scanner.Scan() {
		os.Stdout.Write(scanner.Bytes())
	}
}
