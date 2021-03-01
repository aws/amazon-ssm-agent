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
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/cli/cliutil"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/identity"
	"github.com/twinj/uuid"
)

const (
	sendCommand        = "send-offline-command"
	sendCommandContent = "content"
)

const sendCommandHelp = `NAME:
    {{.SendCommandName}}

DESCRIPTION
SYNOPSIS
    {{.SendCommandName}}
    {{.ContentFlag}}

PARAMETERS
    {{.ContentFlag}} (string) JSON or URL to command document.
    A valid command document is a configuration document with all parameters filled in.
    For information about writing a configuration document, see Configuration Document in the SSM API Reference.

EXAMPLES
    This example runs a command in a document in S3.

    Command:

      {{.SsmCliName}} {{.SendCommandName}} {{.ContentFlag}} https://s3.amazonaws.com/bucketname/keypath/filename.json

    Output:

      Successfully submitted with command id 01234567-890a-bcde-f012-34567890abcd

OUTPUT
    Success message with command id or failure message - failure usually happens because you are not admin or provided invalid JSON
`

type sendCommandHelpParams struct {
	SsmCliName      string
	SendCommandName string
	ContentFlag     string
}

func init() {
	cliutil.Register(&SendOfflineCommand{})
}

type SendOfflineCommand struct {
	helpText string
}

// Execute validates and executes the send-offline-command cli command
func (c *SendOfflineCommand) Execute(agentIdentity identity.IAgentIdentity, subcommands []string, parameters map[string][]string) (error, string) {
	validation := c.validateSendCommandInput(subcommands, parameters)
	// return validation errors if any were found
	if len(validation) > 0 {
		return errors.New(strings.Join(validation, "\n")), ""
	}

	if err, content := c.loadContent(agentIdentity, parameters[sendCommandContent][0]); err != nil {
		return err, ""
	} else if err := c.validateContent(content); err != nil {
		return err, ""
	} else if contentString, err := jsonutil.Marshal(content); err != nil {
		return err, ""
	} else if err, documentName := c.submitCommandDocument(contentString); err != nil {
		return err, ""
	} else {
		return nil, c.waitForSubmitStatus(documentName)
	}
}

// Help prints help for the send-offline-command cli command
func (c *SendOfflineCommand) Help() string {
	if len(c.helpText) == 0 {
		t, _ := template.New("SendOfflineCommandHelp").Parse(sendCommandHelp)
		params := sendCommandHelpParams{cliutil.SsmCliName, sendCommand, cliutil.FormatFlag(sendCommandContent)}
		buf := new(bytes.Buffer)
		t.Execute(buf, params)
		c.helpText = buf.String()
	}
	return c.helpText
}

// Name is the command name used in the cli
func (SendOfflineCommand) Name() string {
	return sendCommand
}

// validateSendCommandInput checks the subcommands and parameters for required values, format, and unsupported values
func (SendOfflineCommand) validateSendCommandInput(subcommands []string, parameters map[string][]string) []string {
	validation := make([]string, 0)
	if subcommands != nil && len(subcommands) > 0 {
		validation = append(validation, fmt.Sprintf("%v does not support subcommand %v", sendCommand, subcommands), "")
		return validation // invalid subcommand is an attempt to execute something that really isn't this command, so the rest of the validation is skipped in this case
	}

	// look for required parameters
	if _, exists := parameters[sendCommandContent]; !exists {
		validation = append(validation, fmt.Sprintf("%v is required", cliutil.FormatFlag(sendCommandContent)))
	} else if len(parameters[sendCommandContent]) != 1 {
		validation = append(validation, fmt.Sprintf("expected 1 value for parameter %v", cliutil.FormatFlag(sendCommandContent)))
	} else {
		// must be valid json or a valid URI
		val := parameters[sendCommandContent][0]
		if !cliutil.ValidJson(val) && !cliutil.ValidUrl(val) {
			validation = append(validation, fmt.Sprintf("%v value must be valid json or a URL", cliutil.FormatFlag(sendCommandContent)))
		}
	}

	// look for unsupported parameters
	for key := range parameters {
		if key != sendCommandContent {
			validation = append(validation, fmt.Sprintf("unknown parameter %v", cliutil.FormatFlag(key)))
		}
	}
	return validation
}

// loadContent loads raw json or json obtained from a URL into DocumentContent
func (SendOfflineCommand) loadContent(agentIdentity identity.IAgentIdentity, rawContent string) (error, contracts.DocumentContent) {
	var content contracts.DocumentContent
	if cliutil.ValidJson(rawContent) {
		err := json.Unmarshal([]byte(rawContent), &content)
		return err, content
	}
	var url = rawContent
	// TODO:MF: Write a URI loader utility - artifact really doesn't do that job
	if strings.HasPrefix(strings.ToLower(url), "file://") {
		url = url[7:]
	}

	context := context.Default(log.NewSilentMockLog(), appconfig.DefaultConfig(), agentIdentity)
	input := &artifact.DownloadInput{SourceURL: url}
	if output, err := artifact.Download(context, *input); err != nil {
		return err, content
	} else {
		err = jsonutil.UnmarshalFile(output.LocalFilePath, &content)
		// TODO:MF: ideally we'd delete the file if we downloaded it - but it might've been a local file and we don't have a good way to tell
		return err, content
	}
}

//validateContent checks to see that content has at least one runtimeConfig for 1.2 or mainSteps for 2.0 and no unbound parameters
func (SendOfflineCommand) validateContent(content contracts.DocumentContent) error {
	switch content.SchemaVersion {
	case "1.2":
		if len(content.RuntimeConfig) == 0 {
			return fmt.Errorf("runtimeConfig cannot be empty")
		}
	case "2.0", "2.0.1", "2.0.2", "2.0.3", "2.2":
		if len(content.MainSteps) == 0 {
			return fmt.Errorf("mainSteps cannot be empty")
		}
	default:
		return fmt.Errorf("unsupported schema version %v", content.SchemaVersion)

	}

	return nil
}

// submitCommandDocument
func (SendOfflineCommand) submitCommandDocument(content string) (error, string) {
	documentName := uuid.NewV4().String()
	documentPath := filepath.Join(appconfig.LocalCommandRoot, documentName)

	if err := fileutil.MakeDirs(appconfig.LocalCommandRoot); err != nil {
		return errors.New("failed to submit command"), ""
	} else if err := fileutil.WriteAllText(documentPath, content); err != nil {
		return err, ""
	}
	return nil, documentName
}

// waitForSubmitStatus
func (c *SendOfflineCommand) waitForSubmitStatus(documentName string) string {
	for i := 0; i < 10; i++ {
		if processed, commandId := c.isDocumentProcessed(documentName, appconfig.LocalCommandRootSubmitted); processed {
			return fmt.Sprintf("successfully submitted with command id: %v", commandId)
		}
		if processed, _ := c.isDocumentProcessed(documentName, appconfig.LocalCommandRootInvalid); processed {
			return "failed to submit document: document was invalid"
		}
		time.Sleep(500 * time.Millisecond)
	}
	documentPath := filepath.Join(appconfig.LocalCommandRoot, documentName)
	fileutil.DeleteFile(documentPath)
	if processed, commandId := c.isDocumentProcessed(documentName, appconfig.LocalCommandRootSubmitted); processed {
		return fmt.Sprintf("successfully submitted with command id: %v", commandId)
	}
	if processed, _ := c.isDocumentProcessed(documentName, appconfig.LocalCommandRootInvalid); processed {
		return "failed to submit document: document was invalid"
	}
	return "failed to submit document: timed out"
}

// isDocumentProcessed checks for a document in the processed folder and returns the command id suffix
func (SendOfflineCommand) isDocumentProcessed(documentName string, folder string) (bool, string) {
	files, _ := fileutil.GetFileNames(folder)
	for _, file := range files {
		if strings.HasPrefix(file, documentName) && strings.Contains(file, ".") {
			return true, file[strings.LastIndex(file, ".")+1:]
		}
	}
	return false, ""
}
