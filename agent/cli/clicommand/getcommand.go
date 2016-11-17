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

// Package clicommand contains the implementation of all commands for the ssm agent cli
package clicommand

import (
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/cli/cliutil"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
)

const (
	getCommand          = "get-offline-command-invocation"
	getCommandCommandID = "command-id"
	getCommandDetails   = "details"
)

type GetOfflineCommand struct{}

// Execute validates and executes the get-offline-command-invocation cli command
func (GetOfflineCommand) Execute(subcommands []string, parameters map[string][]string) (error, string) {
	validation, commandID, showDetails := validateGetCommandInput(subcommands, parameters)
	// return validation errors if any were found
	if len(validation) > 0 {
		return errors.New(strings.Join(validation, "\n")), ""
	}

	return getCommandStatus(commandID, showDetails)
}

// Help prints help for the get-offline-command-invocation cli command
func (GetOfflineCommand) Help(out io.Writer) {
	fmt.Fprintln(out, "NAME:")
	fmt.Fprintf(out, "    %v\n\n", getCommand)
	fmt.Fprintln(out, "DESCRIPTION")
	fmt.Fprintln(out, "SYNOPSIS")
	fmt.Fprintf(out, "    %v\n", getCommand)
	fmt.Fprintf(out, "    %v\n", cliutil.FormatFlag(getCommandCommandID))
	fmt.Fprintf(out, "    %v\n\n", cliutil.FormatFlag(getCommandDetails))
	fmt.Fprintln(out, "PARAMETERS")
	fmt.Fprintf(out, "    %v (string) Command ID from %v.\n\n", cliutil.FormatFlag(getCommandCommandID), sendCommand)
	fmt.Fprintf(out, "    %v (boolean) true if provided\n\n", cliutil.FormatFlag(getCommandCommandID))
	fmt.Fprintln(out, "EXAMPLES")
	fmt.Fprintf(out, "    This example gets status for a command run by the local amazon-ssm-agent service:\n\n")
	fmt.Fprintf(out, "    Command:\n\n")
	fmt.Fprintf(out, "      %v %v %v 01234567-890a-bcde-f012-34567890abcd\n\n", cliutil.SsmCliName, getCommand, cliutil.FormatFlag(getCommandCommandID))
	fmt.Fprintf(out, "    Output:\n\n")
	fmt.Fprintf(out, "      Completed\n\n")
	fmt.Fprintln(out, "OUTPUT")
	fmt.Fprintf(out, "    Status of command - Pending, In Progress, Complete, or Corrupt\n")
}

// Name is the command name used in the cli
func (GetOfflineCommand) Name() string {
	return getCommand
}

// validateGetCommandInput checks the subcommands and parameters for required values, format, and unsupported values
func validateGetCommandInput(subcommands []string, parameters map[string][]string) (validation []string, commandID string, showDetails bool) {
	validation = make([]string, 0)

	if subcommands != nil && len(subcommands) > 0 {
		validation = append(validation, fmt.Sprintf("%v does not support subcommand %v", getCommand, subcommands), "")
		return validation, "", false // invalid subcommand is an attempt to execute something that really isn't this command, so the rest of the validation is skipped in this case
	}

	// look for required parameters
	if _, exists := parameters[getCommandCommandID]; !exists {
		validation = append(validation, fmt.Sprintf("%v is required", cliutil.FormatFlag(getCommandCommandID)))
	} else if len(parameters[getCommandCommandID]) != 1 {
		validation = append(validation, fmt.Sprintf("expected 1 value for parameter %v",
			cliutil.FormatFlag(getCommandCommandID)))
	} else {
		// must be a 36 character UUID
		commandID = parameters[getCommandCommandID][0]
		if commandIdLen := len(commandID); commandIdLen != 36 {
			validation = append(validation,
				fmt.Sprintf("Invalid length for parameter %v.  Length was %v should be 36",
					cliutil.FormatFlag(getCommandCommandID), commandIdLen))
		}
	}
	_, showDetails = parameters[getCommandDetails]
	if showDetails && len(parameters[getCommandDetails]) > 0 {
		validation = append(validation, fmt.Sprintf("flag %v should not have any values", cliutil.FormatFlag(getCommandDetails)))
	}

	// look for unsupported parameters
	for key, _ := range parameters {
		if key != getCommandCommandID && key != getCommandDetails {
			validation = append(validation, fmt.Sprintf("unknown parameter %v", cliutil.FormatFlag(key)))
		}
	}
	return validation, commandID, showDetails
}

// getCommandStatus looks for the command in the local orchestration folders and returns status and optionally details
func getCommandStatus(commandID string, showDetails bool) (error, string) {
	// Look for file with commandID as name in each orchestration folder
	// If found, return status (or lots of details if showDetails is set)
	if isCommandInState(appconfig.DefaultLocationOfCompleted, commandID) {
		return nil, "Complete"
	}
	if isCommandInState(appconfig.DefaultLocationOfPending, commandID) {
		return nil, "Pending"
	}
	if isCommandInState(appconfig.DefaultLocationOfCurrent, commandID) {
		return nil, "In Progress"
	}
	if isCommandInState(appconfig.DefaultLocationOfCorrupt, commandID) {
		return nil, "Corrupt"
	}

	// If not found, return error
	return fmt.Errorf("No status found for command ID %v", commandID), ""
}

func isCommandInState(stateFolder string, commandID string) bool {
	// TODO:MF: Find a way to get the current instanceID instead of trying all possible folders
	dirs, _ := fileutil.GetDirectoryNames(appconfig.DefaultDataStorePath)

	for _, dir := range dirs {
		potentialFolder := path.Join(appconfig.DefaultDataStorePath,
			dir,
			appconfig.DefaultDocumentRootDirName,
			appconfig.DefaultLocationOfState,
			stateFolder,
			commandID)
		if fileutil.Exists(potentialFolder) {
			return true
		}
	}

	return false
}
