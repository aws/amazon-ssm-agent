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

// Package cliutil contains helper functions for cli and clicommand
package cliutil

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/aws/amazon-ssm-agent/common/identity"
)

const (
	HelpFlag                  = "help"
	SsmCliName                = "ssm-cli"
	CLI_PARSE_FAIL_EXITCODE   = 2
	CLI_NO_IDENTITY_EXITCODE  = 3
	CLI_COMMAND_FAIL_EXITCODE = 255
	CLI_SUCCESS_EXITCODE      = 0
)

const (
	flagPrefix = "--"
)

// CliCommands is the set of support commands
var CliCommands map[string]CliCommand

// CliCommand defines the interface for all commands the cli can execute
type CliCommand interface {
	Execute(agentIdentity identity.IAgentIdentity, subcommands []string, parameters map[string][]string) (error, string)
	Help() string
	Name() string
}

// init creates the map of commands - all imported commands will add themselves to the map
func init() {
	CliCommands = make(map[string]CliCommand)
}

// Register
func Register(command CliCommand) {
	CliCommands[command.Name()] = command
}

// FormatFlag returns a parameter name formatted as a command line flag
func FormatFlag(flagName string) string {
	return fmt.Sprintf("%v%v", flagPrefix, flagName)
}

// IsFlag returns true if val is a flag
func IsFlag(val string) bool {
	return strings.HasPrefix(val, flagPrefix)
}

// GetFlag returns the flag name if val is a flag, or empty if it is not
func GetFlag(val string) string {
	if strings.HasPrefix(val, flagPrefix) {
		return strings.ToLower(strings.TrimLeft(val, flagPrefix))
	}
	return ""
}

// IsHelp determines if a subcommand or flag is a request for help
func IsHelp(subcommands []string, parameters map[string][]string) bool {
	for _, val := range subcommands {
		if val == HelpFlag {
			return true
		}
	}
	if _, exists := parameters[HelpFlag]; exists {
		return true
	}
	return false
}

// ValidJson determines if a string is valid Json
func ValidJson(s string) bool {
	var js map[string]interface{}
	return json.Unmarshal([]byte(s), &js) == nil
}

// ValidUrl determines if a string is a valid URL
func ValidUrl(s string) bool {
	if strings.HasPrefix(strings.ToLower(s), "file://") {
		return true
	}
	if _, errUrl := url.ParseRequestURI(s); errUrl == nil {
		return true
	}
	return false
}
