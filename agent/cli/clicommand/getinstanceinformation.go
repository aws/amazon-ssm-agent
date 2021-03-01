// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
	"strings"
	"text/template"

	"github.com/aws/amazon-ssm-agent/agent/cli/cliutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/version"
	"github.com/aws/amazon-ssm-agent/common/identity"
)

const (
	getInstanceInformationCommand = "get-instance-information"
)

const getInstanceInformationCommandHelp = `NAME:
EXAMPLES
    This example returns basic information about the instance this agent is running on,
    including AWS region name, instance id and release version of this CLI.

    Note: release version of this CLI should match the release version of the SSM agent,
    since in normal case, CLI and agent are compiled from same source files; in rare
    case, like updating the agent without updating the CLI, the release vesion returned
    is the CLI version, not the agent version.

    Command:

      {{.SsmCliName}} {{.GetInstanceInformationCommandName}}

    Output:
      {
        "region" : "us-west-2",
        "instance-id" : "i-12345678",
        "release-version" : "1.0.0"
      }

OUTPUT
    Instance information containing region, instance ID and version in JSON format
`

type getInstanceInformationHelpParams struct {
	SsmCliName                        string
	GetInstanceInformationCommandName string
}

func init() {
	cliutil.Register(&GetInstanceInformationCommand{})
}

type GetInstanceInformationCommand struct {
	helpText string
}

// Execute validates and executes the get-instance-information cli command
func (c *GetInstanceInformationCommand) Execute(agentIdentity identity.IAgentIdentity, subcommands []string, parameters map[string][]string) (error, string) {
	validation := c.validateGetInstanceInformationCommandInput(subcommands, parameters)
	// return validation errors if any were found
	if len(validation) > 0 {
		return errors.New(strings.Join(validation, "\n")), ""
	}

	information := make(map[string]string)
	if region, err := agentIdentity.Region(); err != nil {
		return err, ""
	} else {
		information["region"] = region
	}

	if instanceId, err := agentIdentity.InstanceID(); err != nil {
		return err, ""
	} else {
		information["instance-id"] = instanceId
	}

	information["release-version"] = version.Version

	result, _ := jsonutil.Marshal(information)
	return nil, result
}

// Help prints help for the get-instance-information cli command
func (c *GetInstanceInformationCommand) Help() string {
	if len(c.helpText) == 0 {
		t, _ := template.New("GetInstanceInformationCommandHelp").Parse(getInstanceInformationCommandHelp)
		params := getInstanceInformationHelpParams{cliutil.SsmCliName, getInstanceInformationCommand}
		buf := new(bytes.Buffer)
		t.Execute(buf, params)
		c.helpText = buf.String()
	}
	return c.helpText
}

// Name is the command name used in the cli
func (GetInstanceInformationCommand) Name() string {
	return getInstanceInformationCommand
}

// validateGetInstanceInformationCommandInput checks the subcommands and parameters for required values, format, and unsupported values
func (GetInstanceInformationCommand) validateGetInstanceInformationCommandInput(subcommands []string, parameters map[string][]string) []string {
	validation := make([]string, 0)
	if subcommands != nil && len(subcommands) > 0 {
		validation = append(validation, fmt.Sprintf("%v does not support subcommand %v", getInstanceInformationCommand, subcommands), "")
		return validation // invalid subcommand is an attempt to execute something that really isn't this command, so the rest of the validation is skipped in this case
	}

	// look for unsupported parameters
	for key, _ := range parameters {
		validation = append(validation, fmt.Sprintf("unknown parameter %v", cliutil.FormatFlag(key)))
	}
	return validation
}
