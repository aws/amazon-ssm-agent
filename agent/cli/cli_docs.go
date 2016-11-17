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

// Package cli contains the implementation of the ssm agent cli
package cli

import (
	"fmt"
	"io"
	"sort"

	"github.com/aws/amazon-ssm-agent/agent/cli/cliutil"
)

// TODO:MF: consider changing these to use templates
// displayUsage prints cli usage info to the console
func displayUsage(out io.Writer) {
	fmt.Fprintf(out, "usage: %v [options] <command> [subcommand1 subcommand2...] [parameters]\n", cliutil.SsmCliName)
	fmt.Fprint(out, "To see help text, you can run:\n\n")
	fmt.Fprintf(out, "  %v %v\n", cliutil.SsmCliName, cliutil.HelpFlag)
	fmt.Fprintf(out, "  %v <command> %v\n", cliutil.SsmCliName, cliutil.HelpFlag)
	fmt.Fprintf(out, "  %v <command> <subcommand> %v\n", cliutil.SsmCliName, cliutil.HelpFlag)
}

// displayValidCommands prints a list of valid cli commands to the console
func displayValidCommands(out io.Writer) {
	commands := make([]string, 0, len(cliutil.CliCommands))
	for command := range cliutil.CliCommands {
		commands = append(commands, command)
	}
	sort.Strings(commands)
	for _, command := range commands {
		fmt.Fprintf(out, "%v\n", command)
	}
}

// displayHelp shows help for the ssm sli
func displayHelp(out io.Writer) {
	fmt.Fprintf(out, "%v\n", cliutil.SsmCliName)
	fmt.Fprintf(out, "Submit commands directly to the local amazon-ssm-agent service.\n")
	fmt.Fprintf(out, "You will need to have admin rights to run most commands.\n\n")
	fmt.Fprintf(out, "Available commands:\n")
	displayValidCommands(out)
}
