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

	"github.com/aws/amazon-ssm-agent/agent/cli/cliutil"
	"github.com/twinj/uuid"
)

// TODO:MF: make errors more like ssm-cli: error: <arg type>: <error>?
// RunCommand parses and executes a single command line
func RunCommand(args []string, out io.Writer) {
	uuid.SwitchFormat(uuid.CleanHyphen)
	if len(args) < 2 {
		displayUsage(out)
		return
	}
	err, _, command, subcommands, parameters := parseCommand(args)
	if err != nil {
		displayUsage(out)
		fmt.Fprintln(out, err.Error())
		return
	}
	if cmd, exists := cliutil.CliCommands[command]; exists {
		if cliutil.IsHelp(subcommands, parameters) {
			cmd.Help(out)
		} else {
			cmdErr, result := cmd.Execute(subcommands, parameters)
			if cmdErr != nil {
				displayUsage(out)
				fmt.Fprintln(out, cmdErr.Error())
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
	}
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
