// Copyright 2021 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
	"encoding/json"
	"fmt"
	"log"
	"runtime/debug"
	"sort"
	"sync"
	"text/template"

	"github.com/aws/amazon-ssm-agent/agent/cli/cliutil"
	_ "github.com/aws/amazon-ssm-agent/agent/cli/diagnostics"
	"github.com/aws/amazon-ssm-agent/agent/cli/diagnosticsutil"
)

const (
	getDiagnosticsCommand = "get-diagnostics"
	maxTableWidth         = 80
	maxParallelExecution  = 5
)

// The get-diagnostics was inspired from these two repositories:
// https://github.com/awslabs/aws-support-tools/tree/master/Systems%20Manager/SSMAGENT-TOOLKIT-LINUX
// https://github.com/awslabs/aws-support-tools/tree/master/Systems%20Manager/SSMAgent-Toolkit-Windows

const getDiagnosticsCommandHelp = `NAME:
    {{.GetDiagnosticsCommandName}}
DESCRIPTION
    Dignostics tool would help you to identify possible reasons for the SSM agent not successfully registering the instance to ssm service.
SYNOPSIS
    {{.GetDiagnosticsCommandName}}
    --output
PARAMETERS
    --output table
    This is an optional parameter to create a table format output.
EXAMPLES
    This example will run the diagnostics with JSON output.

    Command:

        {{.SsmCliName}} {{.GetDiagnosticsCommandName}}

    Output:

        {
            "DiagnosticsOutput": [
                {
                    "Check": "EC2 IMDS",
                    "Status": "Success",
                    "Note": "IMDS is accessible and has instance id i-0123456789abcdefa in region us-east-1"
                },
                {
                    "Check": "Hybrid instance registration",
                    "Status": "Skipped",
                    "Note": "Instance does not have hybrid registration"
                },
                {
                    "Check": "Connectivity to ssm endpoint",
                    "Status": "Success",
                    "Note": "ssm.us-east-1.amazonaws.com is reachable"
                },
                {
                    "Check": "Connectivity to ec2messages endpoint",
                    "Status": "Success",
                    "Note": "ec2messages.us-east-1.amazonaws.com is reachable"
                },
                {
                    "Check": "Connectivity to ssmmessages endpoint",
                    "Status": "Success",
                    "Note": "ssmmessages.us-east-1.amazonaws.com is reachable"
                },
                {
                    "Check": "Connectivity to s3 endpoint",
                    "Status": "Success",
                    "Note": "s3.us-east-1.amazonaws.com is reachable"
                },
                {
                    "Check": "Connectivity to kms endpoint",
                    "Status": "Success",
                    "Note": "kms.us-east-1.amazonaws.com is reachable"
                },
                {
                    "Check": "Connectivity to logs endpoint",
                    "Status": "Success",
                    "Note": "logs.us-east-1.amazonaws.com is reachable"
                },
                {
                    "Check": "Connectivity to monitoring endpoint",
                    "Status": "Success",
                    "Note": "monitoring.us-east-1.amazonaws.com is reachable"
                },
                {
                    "Check": "AWS Credentials",
                    "Status": "Success",
                    "Note": "Credentials are for arn:aws:sts::123456789012:assumed-role/SSM-Role/i-0123456789abcdefa and will expire at 2021-09-02 13:24:42 +0000 UTC"
                },
                {
                    "Check": "Agent service",
                    "Status": "Success",
                    "Note": "Agent service is running and is running as expected user"
                },
                {
                    "Check": "Proxy configuration",
                    "Status": "Skipped",
                    "Note": "No proxy configuration detected"
                },
                {
                    "Check": "Windows sysprep image state",
                    "Status": "Success",
                    "Note": "Windows image state value is at desired value IMAGE_STATE_COMPLETE"
                },
                {
                    "Check": "SSM Agent version",
                    "Status": "Success",
                    "Note": "SSM Agent version is 3.0.655.0, latest agent version in us-east-1 is 3.1.192.0"
                }
            ]
        }

    This example will run the diagnostics with table format output.

    Command:

        {{.SsmCliName}} {{.GetDiagnosticsCommandName}} --output table

    Output:
        ┌───────────────────────────────────────┬─────────┬─────────────────────────────────────────────────────────────────────┐
        │ Check                                 │ Status  │ Note                                                                │
        ├───────────────────────────────────────┼─────────┼─────────────────────────────────────────────────────────────────────┤
        │ EC2 IMDS                              │ Success │ IMDS is accessible and has instance id i-0123456789abcdefa in       │
        │                                       │         │ region us-east-1                                                    │
        ├───────────────────────────────────────┼─────────┼─────────────────────────────────────────────────────────────────────┤
        │ Hybrid instance registration          │ Skipped │ Instance does not have hybrid registration                          │
        ├───────────────────────────────────────┼─────────┼─────────────────────────────────────────────────────────────────────┤
        │ Connectivity to ssm endpoint          │ Success │ ssm.us-east-1.amazonaws.com is reachable                            │
        ├───────────────────────────────────────┼─────────┼─────────────────────────────────────────────────────────────────────┤
        │ Connectivity to ec2messages endpoint  │ Success │ ec2messages.us-east-1.amazonaws.com is reachable                    │
        ├───────────────────────────────────────┼─────────┼─────────────────────────────────────────────────────────────────────┤
        │ Connectivity to ssmmessages endpoint  │ Success │ ssmmessages.us-east-1.amazonaws.com is reachable                    │
        ├───────────────────────────────────────┼─────────┼─────────────────────────────────────────────────────────────────────┤
        │ Connectivity to s3 endpoint           │ Success │ s3.us-east-1.amazonaws.com is reachable                             │
        ├───────────────────────────────────────┼─────────┼─────────────────────────────────────────────────────────────────────┤
        │ Connectivity to kms endpoint          │ Success │ kms.us-east-1.amazonaws.com is reachable                            │
        ├───────────────────────────────────────┼─────────┼─────────────────────────────────────────────────────────────────────┤
        │ Connectivity to logs endpoint         │ Success │ logs.us-east-1.amazonaws.com is reachable                           │
        ├───────────────────────────────────────┼─────────┼─────────────────────────────────────────────────────────────────────┤
        │ Connectivity to monitoring endpoint   │ Success │ monitoring.us-east-1.amazonaws.com is reachable                     │
        ├───────────────────────────────────────┼─────────┼─────────────────────────────────────────────────────────────────────┤
        │ AWS Credentials                       │ Success │ Credentials are for                                                 │
        │                                       │         │ arn:aws:sts::123456789012:assumed-role/SSM-Role/i-0123456789abcdefa │
        │                                       │         │ and will expire at 2021-09-02 13:24:42 +0000 UTC                    │
        ├───────────────────────────────────────┼─────────┼─────────────────────────────────────────────────────────────────────┤
        │ Agent service                         │ Success │ Agent service is running and is running as expected user            │
        ├───────────────────────────────────────┼─────────┼─────────────────────────────────────────────────────────────────────┤
        │ Proxy configuration                   │ Skipped │ No proxy configuration detected                                     │
        ├───────────────────────────────────────┼─────────┼─────────────────────────────────────────────────────────────────────┤
        │ Windows sysprep image state           │ Success │ Windows image state value is at desired value IMAGE_STATE_COMPLETE  │
        ├───────────────────────────────────────┼─────────┼─────────────────────────────────────────────────────────────────────┤
        │ SSM Agent version                     │ Success │ SSM Agent version is 3.0.655.0, latest agent version in us-east-1   │
        │                                       │         │ is 3.1.192.0                                                        │
        └───────────────────────────────────────┴─────────┴─────────────────────────────────────────────────────────────────────┘
OUTPUT
    Provide the status and a note of each check.
`

type getDiagnosticsHelpParams struct {
	SsmCliName                string
	GetDiagnosticsCommandName string
}

func init() {
	cliutil.Register(&GetDiagnosticsCommand{
		"",
		sync.Mutex{},
		make(chan bool, maxParallelExecution),
		sync.WaitGroup{},
		map[string]diagnosticsutil.DiagnosticOutput{},
	})
}

type GetDiagnosticsCommand struct {
	helpText          string
	writeOutputLock   sync.Mutex
	backPressureValve chan bool
	waitGroup         sync.WaitGroup
	outputMap         map[string]diagnosticsutil.DiagnosticOutput
}

// all supported parameters
var supportedParameters = map[string]bool{"output": true}

// Output parameters values
var jsonOutput = "json"
var tableOutput = "table"
var allOutputFormats = []string{jsonOutput, tableOutput}

type jsonOutputStruct struct {
	DiagnosticsOutput []diagnosticsutil.DiagnosticOutput
}

func validateParameters(parameters map[string][]string) error {
	for key, _ := range parameters {
		if !supportedParameters[key] {
			return fmt.Errorf("'%s' is not a supported parameter for %s", key, getDiagnosticsCommand)
		}
	}

	return nil
}

func getOutputFormat(parameters map[string][]string) (string, error) {
	outputList, ok := parameters["output"]
	if !ok || len(outputList) == 0 {
		// default to JSON output
		return jsonOutput, nil
	}

	if len(outputList) > 1 {
		return "", fmt.Errorf("Output parameter only supports one of the following values: %s", allOutputFormats)
	}

	for _, outputType := range allOutputFormats {
		if outputType == outputList[0] {
			return outputType, nil
		}
	}

	return "", fmt.Errorf("'%s' is not a supported output parameter value", outputList[0])
}

func (c *GetDiagnosticsCommand) appendOutput(output diagnosticsutil.DiagnosticOutput) {
	c.writeOutputLock.Lock()
	defer c.writeOutputLock.Unlock()
	c.outputMap[output.Check] = output
}

func (c *GetDiagnosticsCommand) executeQuery(query diagnosticsutil.DiagnosticQuery) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("ssm-cli get-diagnostics panic during %s check: %v\n", query.GetName(), r)
			log.Printf("Stacktrace:\n%s\n", debug.Stack())
		}
	}()
	defer c.waitGroup.Done()
	c.appendOutput(query.Execute())
	<-c.backPressureValve
}

// Execute validates and executes the get-diagnostics cli command
func (c *GetDiagnosticsCommand) Execute(subcommands []string, parameters map[string][]string) (err error, output string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("ssm-cli get-diagnostics panic: %v\n", r)
			log.Printf("Stacktrace:\n%s\n", debug.Stack())
			err, _ = r.(error)
		}
	}()

	// validate parameters
	err = validateParameters(parameters)
	if err != nil {
		return err, ""
	}

	// detect output type
	outputFormat, err := getOutputFormat(parameters)
	if err != nil {
		return err, ""
	}

	// Check if cli is running as admin/root
	err = diagnosticsutil.IsRunningElevatedPermissions()
	if err != nil {
		return nil, err.Error()
	}

	// Set proxy configuration to make sure we use the same configuration as agent
	diagnosticsutil.AssumeAgentEnvironmentProxy()

	// Sort queries base on priority
	sort.SliceStable(diagnosticsutil.DiagnosticQueries, func(i, j int) bool {
		return diagnosticsutil.DiagnosticQueries[i].GetPriority() < diagnosticsutil.DiagnosticQueries[j].GetPriority()
	})

	for _, query := range diagnosticsutil.DiagnosticQueries {
		c.backPressureValve <- true
		c.waitGroup.Add(1)
		go c.executeQuery(query)
	}

	c.waitGroup.Wait()

	var outputList []diagnosticsutil.DiagnosticOutput
	for _, query := range diagnosticsutil.DiagnosticQueries {
		// Only reason output would not be in the map is if check panic'ed, would print stacktrace in executeQuery func
		if outputObj, ok := c.outputMap[query.GetName()]; ok {
			outputList = append(outputList, outputObj)
		}
	}

	switch outputFormat {
	case jsonOutput:
		outputBytes, err := json.MarshalIndent(jsonOutputStruct{outputList}, "", "  ")
		return err, string(outputBytes)
	case tableOutput:
		t := diagnosticsutil.NewTableFormatter(maxTableWidth, outputList)
		return nil, t.String()
	default:
		// Should never happen because of validation above
		return fmt.Errorf("Output format not defined"), ""
	}
}

// Help prints help for the get-instance-information cli command
func (c *GetDiagnosticsCommand) Help() string {
	if len(c.helpText) == 0 {
		t, _ := template.New("GetDiagnosticsCommandHelp").Parse(getDiagnosticsCommandHelp)
		params := getDiagnosticsHelpParams{cliutil.SsmCliName, getDiagnosticsCommand}
		buf := new(bytes.Buffer)
		t.Execute(buf, params)
		c.helpText = buf.String()
	}
	return c.helpText
}

// Name is the command name used in the cli
func (*GetDiagnosticsCommand) Name() string {
	return getDiagnosticsCommand
}
