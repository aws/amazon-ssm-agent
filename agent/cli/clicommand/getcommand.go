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
	"bytes"
	"errors"
	"fmt"
	"path"
	"strings"
	"text/template"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/cli/cliutil"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/common/identity"
)

const (
	getCommand          = "get-offline-command-invocation"
	getCommandCommandID = "command-id"
	getCommandDetails   = "details"
)

const getCommandHelp = `NAME:
    {{.GetCommandName}}

DESCRIPTION
SYNOPSIS
    {{.GetCommandName}}
    {{.CommandIdFlag}}
    {{.DetailsFlag}}

PARAMETERS
    {{.CommandIdFlag}} (string) Command ID from {{.SendCommandName}}.

    {{.DetailsFlag}} (boolean) true if provided.

EXAMPLES
    This example gets status for a command run by the local amazon-ssm-agent service.

    Command:

      {{.SsmCliName}} {{.GetCommandName}} {{.CommandIdFlag}} 01234567-890a-bcde-f012-34567890abcd

    Output:

      Completed

OUTPUT
    Status of command - Pending, In Progress, Complete, or Corrupt
`

type getCommandHelpParams struct {
	SsmCliName      string
	GetCommandName  string
	SendCommandName string
	CommandIdFlag   string
	DetailsFlag     string
}

func init() {
	cliutil.Register(&GetOfflineCommand{})
}

type GetOfflineCommand struct {
	helpText string
}

// Execute validates and executes the get-offline-command-invocation cli command
func (c *GetOfflineCommand) Execute(agentIdentity identity.IAgentIdentity, subcommands []string, parameters map[string][]string) (error, string) {
	validation, commandID, showDetails := c.validateGetCommandInput(subcommands, parameters)
	// return validation errors if any were found
	if len(validation) > 0 {
		return errors.New(strings.Join(validation, "\n")), ""
	}

	instanceID, err := agentIdentity.InstanceID()
	if err != nil {
		return err, ""
	}

	return c.getCommandStatus(instanceID, commandID, showDetails)
}

// Help prints help for the get-offline-command-invocation cli command
func (c *GetOfflineCommand) Help() string {
	if len(c.helpText) == 0 {
		t, _ := template.New("GetOfflineCommandHelp").Parse(getCommandHelp)
		params := getCommandHelpParams{cliutil.SsmCliName, getCommand, sendCommand, cliutil.FormatFlag(getCommandCommandID), cliutil.FormatFlag(getCommandDetails)}
		buf := new(bytes.Buffer)
		t.Execute(buf, params)
		c.helpText = buf.String()
	}
	return c.helpText
}

// Name is the command name used in the cli
func (GetOfflineCommand) Name() string {
	return getCommand
}

// validateGetCommandInput checks the subcommands and parameters for required values, format, and unsupported values
func (GetOfflineCommand) validateGetCommandInput(subcommands []string, parameters map[string][]string) (validation []string, commandID string, showDetails bool) {
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
	for key := range parameters {
		if key != getCommandCommandID && key != getCommandDetails {
			validation = append(validation, fmt.Sprintf("unknown parameter %v", cliutil.FormatFlag(key)))
		}
	}
	return validation, commandID, showDetails
}

// getCommandStatus looks for the command in the local orchestration folders and returns status and optionally details
func (c *GetOfflineCommand) getCommandStatus(instanceID, commandID string, showDetails bool) (error, string) {
	// Look for file with commandID as name in each orchestration folder
	// If found, return status (or lots of details if showDetails is set)
	if c.isCommandCompleted(commandID) {
		return nil, "Complete"
	}
	if c.isCommandInState(instanceID, appconfig.DefaultLocationOfPending, commandID) {
		return nil, "Pending"
	}
	if c.isCommandInState(instanceID, appconfig.DefaultLocationOfCurrent, commandID) {
		return nil, "In Progress"
	}
	if c.isCommandInState(instanceID, appconfig.DefaultLocationOfCorrupt, commandID) {
		return nil, "Corrupt"
	}

	// If not found, return error
	return fmt.Errorf("No status found for command ID %v", commandID), ""
}

func (c *GetOfflineCommand) isCommandCompleted(commandID string) bool {
	return fileutil.Exists(path.Join(appconfig.LocalCommandRootCompleted, commandID))
}

func (GetOfflineCommand) isCommandInState(instanceID, stateFolder, commandID string) bool {
	return fileutil.Exists(path.Join(appconfig.DefaultDataStorePath,
		instanceID,
		appconfig.DefaultDocumentRootDirName,
		appconfig.DefaultLocationOfState,
		stateFolder,
		commandID))
}
