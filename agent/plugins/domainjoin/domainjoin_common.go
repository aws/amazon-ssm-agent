// Copyright 2023 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
//

// Package domainjoin implements the domain join plugin.
package domainjoin

import (
	"fmt"
	"regexp"

	"github.com/google/shlex"
)

// directoryIdPattern matches the canonical directory id form: "d-" followed
// by exactly ten lowercase hex characters.
var directoryIdPattern = regexp.MustCompile(`^d-[0-9a-f]{10}$`)

// directoryNamePattern matches DNS-label characters (letters, digits, dot,
// hyphen) and requires an alphanumeric first and last character.
var directoryNamePattern = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?$`)

// isValidDirectoryId returns true when arg matches directoryIdPattern.
func isValidDirectoryId(arg string) bool {
	return directoryIdPattern.MatchString(arg)
}

// isValidDirectoryName returns true when arg matches directoryNamePattern.
func isValidDirectoryName(arg string) bool {
	return directoryNamePattern.MatchString(arg)
}

// Returns true if an injection command is detected in a shell command
func isShellInjection(arg string) bool {
	// The following characters are common shell injection characters
	// Keeping the checks separate for readability
	//
	// The single quote is included because domain-join parameters are
	// single-quote wrapped before the command line is re-parsed; an embedded
	// single quote would terminate that wrapping. The newline and carriage
	// return are included because they would introduce an additional token
	// during that re-parse. None of these characters is valid in a directory
	// id, directory name, OU, hostname, or DNS address.

	injectionCharsArray := []byte{'$', '`', ';', '|', '&', '\'', '\n', '\r'}

	for _, c := range injectionCharsArray {
		for _, s := range arg {
			if byte(s) == c {
				return true
			}
		}
	}

	return false
}

func makeCommandParts(command string) (commandParts []string, err error) {

	if commandParts, err = shlex.Split(command); err != nil {
		return []string{}, fmt.Errorf("Command lexical analysis failed: %v", err)
	}

	return
}
