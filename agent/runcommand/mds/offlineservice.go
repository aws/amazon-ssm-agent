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

// Package service is a wrapper for the SSM Message Delivery Service and Offline Command Service
package service

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"errors"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/runcommand/contracts"
	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/aws/aws-sdk-go/service/ssmmds"
	"github.com/twinj/uuid"
)

type offlineService struct {
	TopicPrefix         string
	newCommandDir       string
	submittedCommandDir string
	commandResultDir    string
	invalidCommandDir   string
}

// NewOfflineService initializes a service that looks for work in a local command folder
func NewOfflineService(log log.T, topicPrefix string) (Service, error) {
	uuid.SwitchFormat(uuid.CleanHyphen)
	// Create and harden local document folder if needed
	err := fileutil.MakeDirs(appconfig.LocalCommandRoot)
	if err != nil {
		log.Errorf("Failed to create local command directory %v : %v", appconfig.LocalCommandRoot, err.Error())
		return nil, err
	}
	err = fileutil.MakeDirs(appconfig.LocalCommandRootCompleted)
	return &offlineService{
		TopicPrefix:         topicPrefix,
		newCommandDir:       appconfig.LocalCommandRoot,
		submittedCommandDir: appconfig.LocalCommandRootSubmitted,
		invalidCommandDir:   appconfig.LocalCommandRootInvalid,
		commandResultDir:    appconfig.LocalCommandRootCompleted,
	}, err
}

// GetMessages looks for new local command documents on the filesystem and parses them into messages
func (ols *offlineService) GetMessages(log log.T, instanceID string) (messages *ssmmds.GetMessagesOutput, err error) {
	messages = &ssmmds.GetMessagesOutput{}

	// Look for unprocessed locally submitted documents
	var docName, docPath string
	var filenames []string
	if filenames, err = fileutil.GetFileNames(ols.newCommandDir); err != nil {
		log.Debugf("offlineservice: error: %v", err.Error())
		return messages, err
	}
	messages.Messages = make([]*ssmmds.Message, 0, len(filenames))
	for _, filename := range filenames {
		docName = filename
		docPath = filepath.Join(ols.newCommandDir, docName)
		log.Debugf("Found local command document %v | %v", docName, docPath)

		requestUuid := uuid.NewV4().String()
		messages.MessagesRequestId = &requestUuid // TODO:MF: Can this be the same as the commandID?

		commandID := uuid.NewV4().String()
		messageID := fmt.Sprintf("aws.ssm.%v.%v", commandID, instanceID)

		// Parse file
		var content contracts.DocumentContent
		if errContent := jsonutil.UnmarshalFile(docPath, &content); errContent != nil {
			log.Errorf("Error parsing command document %v:\n%v", docName, errContent)
			if errMove := moveCommandDocument(ols.newCommandDir, ols.invalidCommandDir, docName, commandID); errMove != nil {
				log.Errorf("Command %v was invalid but failed to move to invalid folder: %v", commandID, errMove.Error())
			}
			continue
		}
		debugContent, _ := jsonutil.Marshal(content)
		log.Debugf("Local command content:\n%v", debugContent)

		// Turn it into a message
		payload := &messageContracts.SendCommandPayload{DocumentContent: content, CommandID: commandID, DocumentName: docName}
		var payloadstr string
		if payloadstr, err = jsonutil.Marshal(payload); err != nil {
			log.Errorf("Error marshalling message for command document %v with message ID %v:\n%v", docName, messageID, err)
			if errMove := moveCommandDocument(ols.newCommandDir, ols.invalidCommandDir, docName, commandID); errMove != nil {
				log.Errorf("Command %v was invalid but failed to move to invalid folder: %v", commandID, errMove.Error())
			}
			continue
		}
		created := times.ToIso8601UTC(time.Now())
		topic := fmt.Sprintf("%v.%v", ols.TopicPrefix, docName)
		message := &ssmmds.Message{
			CreatedDate: &created,
			Destination: &instanceID,
			MessageId:   &messageID,
			Payload:     &payloadstr,
			Topic:       &topic,
		}
		// Move to submitted
		if errMove := moveCommandDocument(ols.newCommandDir, ols.submittedCommandDir, docName, commandID); errMove != nil {
			log.Errorf("Command %v was valid but failed to move to submitted folder: %v", commandID, errMove.Error())
			continue // If doc failed to move, we will not return this message - we don't want to reprocess it or make it impossible to know which command ID it was given
		}

		messages.Messages = append(messages.Messages, message)
	}

	return messages, nil
}

// TODO:MF: clean up old documents in dstDir?  Or maybe do that in SendReply?  Maybe both
// moveCommandDocument moves a command into its final destination and attaches the command ID file extension
func moveCommandDocument(srcDir string, dstDir string, docName string, commandID string) error {
	// Make directory with appropriate ACL
	if err := fileutil.MakeDirs(dstDir); err != nil {
		// This will fail if there is a file in the directory with the same name as
		// the directory we're trying to create and the directory doesn't exist yet (see os.MakedirAll in path.go)
		return err
	}
	newName := strings.Join([]string{docName, commandID}, ".")
	if success, err := fileutil.MoveAndRenameFile(srcDir, docName, dstDir, newName); !success {
		// Clean up submitted document if we failed to move it (we don't want to keep trying to process it)
		defer fileutil.DeleteFile(filepath.Join(srcDir, docName))
		if err != nil {
			return err
		} else {
			return errors.New("failed to move file")
		}
	}
	return nil
}

func (ols *offlineService) AcknowledgeMessage(log log.T, messageID string) error {
	return nil
}

//offline service bookkeeps the command output to specified disk location
func (ols *offlineService) SendReply(log log.T, messageID string, payload string) error {
	commandID, err := messageContracts.GetCommandID(messageID)
	if err != nil {
		log.Errorf("failed to parse messageID: %v", err)
		return nil
	}
	if err := fileutil.WriteAllText(filepath.Join(ols.commandResultDir, commandID), payload); err != nil {
		log.Errorf("failed to write command %v result: %v", commandID, err)
	}
	return nil
}

func (ols *offlineService) FailMessage(log log.T, messageID string, failureType FailureType) error {
	return nil
}

func (ols *offlineService) DeleteMessage(log log.T, messageID string) error {
	return nil
}

func (ols *offlineService) Stop() {}

func (ols *offlineService) LoadFailedReplies(log log.T) []string {
	return nil
}

func (ols *offlineService) DeleteFailedReply(log log.T, replyId string) {}

func (ols *offlineService) PersistFailedReply(log log.T, sendReply ssmmds.SendReplyInput) error {
	return nil
}

func (ols *offlineService) GetFailedReply(log log.T, replyId string) (*ssmmds.SendReplyInput, error) {
	return nil, nil
}

func (ols *offlineService) SendReplyWithInput(log log.T, sendReply *ssmmds.SendReplyInput) error {
	return nil
}
