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

// Package cli represents the entry point of the ssm agent cli.
package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/cli/cliutil"
	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/identity"
	"github.com/twinj/uuid"
)

// Assign to variable to be able to mock function
var newAgentIdentity = identity.NewAgentIdentity

// TODO:MF: make errors more like ssm-cli: error: <arg type>: <error>?
// RunCommand parses and executes a single command line, please refer the aws cli exit code
// https://docs.aws.amazon.com/cli/latest/topic/return-codes.html
func RunCommand(args []string, out io.Writer) (exitCode int) {

	uuid.SwitchFormat(uuid.CleanHyphen)
	if len(args) < 2 {
		displayUsage(out)
		// Customer doesn't provide enough arguments
		return cliutil.CLI_PARSE_FAIL_EXITCODE
	}
	err, _, command, subcommands, parameters := parseCommand(args)
	if err != nil {
		displayUsage(out)
		fmt.Fprintln(out, err.Error())
		// Exit with 2 if parseCommand error occurs
		return cliutil.CLI_PARSE_FAIL_EXITCODE
	}
	if cmd, exists := cliutil.CliCommands[command]; exists {
		if cliutil.IsHelp(subcommands, parameters) {
			fmt.Fprint(out, cmd.Help())
		} else {
			log := logger.NewSilentMockLog()
			config := appconfig.DefaultConfig()
			selector := identity.NewDefaultAgentIdentitySelector(log)
			agentIdentity, err := newAgentIdentity(log, &config, selector)
			if err != nil {
				fmt.Fprintf(out, "Failed to load agent identity: %v", err)
				return cliutil.CLI_NO_IDENTITY_EXITCODE
			}

			cmdErr, result := cmd.Execute(agentIdentity, subcommands, parameters)
			if cmdErr != nil {
				displayUsage(out)
				fmt.Fprintln(out, cmdErr.Error())
				// Exit 255 if command failed
				return cliutil.CLI_COMMAND_FAIL_EXITCODE
			} else {
				fmt.Fprintln(out, result)
			}
		}
	} else if command == cliutil.HelpFlag {
		displayHelp(out)
	} else {
		displayUsage(out)
		fmt.Fprintf(out, "\nInvalid command %v.  The following commands are supported:\n\n", command)
		displayValidCommands(out)
		// Customer input command is invalid
		return cliutil.CLI_PARSE_FAIL_EXITCODE
	}
	return cliutil.CLI_SUCCESS_EXITCODE
}

// parseCommand turns the command line arguments into a command name and a map of flag names and values
// args format should be ssm-cli [options] <command> <subcommand> [<subcommand> ...] [parameters]
func parseCommand(args []string) (err error, options []string, command string, subcommands []string, parameters map[string][]string) {
	// TODO:MF: aws cli is case-sensitive on things other than parameter value, I propose we be case-insensitive on all non-values

	argCount := len(args)
	pos := 1

	// Options
	options = make([]string, 0)
	for _, val := range args[pos:] {
		if !cliutil.IsFlag(val) {
			break
		}
		options = append(options, cliutil.GetFlag(val))
		pos++
	}

	// Command
	if pos >= argCount {
		err = errors.New("command is required")
		return
	}
	command = strings.ToLower(args[pos])
	pos++

	// Subcommands
	if pos >= argCount {
		return
	}
	subcommands = make([]string, 0)
	for _, val := range args[pos:] {
		if cliutil.IsFlag(val) {
			break
		}
		subcommands = append(options, strings.ToLower(val))
		pos++
	}

	// Parameters
	if pos >= argCount {
		return
	}
	parameters = make(map[string][]string)
	var parameterName string
	for _, val := range args[2:] {
		if cliutil.IsFlag(val) {
			parameterName = cliutil.GetFlag(val)
			if parameterName == "" {
				// aws cli doesn't valid this
				err = fmt.Errorf("input contains parameter with no name")
				return
			}
			if _, exists := parameters[parameterName]; exists {
				// aws cli doesn't valid this
				err = fmt.Errorf("duplicate parameter %v", parameterName)
				return
			}
			parameters[parameterName] = make([]string, 0)
		} else {
			parameters[parameterName] = append(parameters[parameterName], val)
		}
	}
	return
}
