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
	messageContracts "github.com/aws/amazon-ssm-agent/agent/message/contracts"
	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/aws/aws-sdk-go/service/ssmmds"
	"github.com/twinj/uuid"
)

type offlineService struct {
	TopicPrefix         string
	newCommandDir       string
	submittedCommandDir string
	invalidCommandDir   string
	completeCommandDir  string
}

func NewOfflineService(log log.T, topicPrefix string) (Service, error) {
	uuid.SwitchFormat(uuid.CleanHyphen)
	// Create and harden local document folder if needed
	err := fileutil.MakeDirs(appconfig.LocalCommandRoot)
	if err != nil {
		log.Errorf("Failed to create local command directory %v : %v", appconfig.LocalCommandRoot, err.Error())
		return nil, err
	}
	return &offlineService{TopicPrefix: topicPrefix}, nil
}

func (ols *offlineService) GetMessages(log log.T, instanceID string) (messages *ssmmds.GetMessagesOutput, err error) {
	messages = &ssmmds.GetMessagesOutput{}

	newCommandDir := appconfig.LocalCommandRoot
	submittedCommandDir := appconfig.LocalCommandRootSubmitted
	invalidCommandDir := appconfig.LocalCommandRootInvalid

	// Look for unprocessed locally submitted documents
	var docName, docPath string
	var filenames []string
	if filenames, err = fileutil.GetFileNames(newCommandDir); err != nil {
		log.Debugf("offlineservice: error: %v", err.Error())
		return messages, err
	}
	messages.Messages = make([]*ssmmds.Message, 0, len(filenames))
	for _, filename := range filenames {
		docName = filename
		docPath = filepath.Join(newCommandDir, docName)
		log.Debugf("Found local command document %v | %v", docName, docPath)

		requestUuid := uuid.NewV4().String()
		messages.MessagesRequestId = &requestUuid // TODO:MF: Can this be the same as the commandID?

		commandID := uuid.NewV4().String()
		messageID := fmt.Sprintf("aws.ssm.%v.%v", commandID, instanceID)

		// Parse file
		var content contracts.DocumentContent
		if errContent := jsonutil.UnmarshalFile(docPath, &content); errContent != nil {
			log.Errorf("Error parsing command document %v:\n%v", docName, errContent)
			moveCommandDocument(newCommandDir, invalidCommandDir, docName, commandID)
			continue
		}
		debugContent, _ := jsonutil.Marshal(content)
		log.Debugf("Local command content:\n%v", debugContent)

		// Turn it into a message
		payload := &messageContracts.SendCommandPayload{DocumentContent: content, CommandID: commandID, DocumentName: docName}
		var payloadstr string
		if payloadstr, err = jsonutil.Marshal(payload); err != nil {
			log.Errorf("Error marshalling message for command document %v with message ID %v:\n%v", docName, messageID, err)
			if errMove := moveCommandDocument(newCommandDir, invalidCommandDir, docName, commandID); errMove != nil {
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
		messages.Messages = append(messages.Messages, message)

		// Move to submitted
		if errMove := moveCommandDocument(newCommandDir, submittedCommandDir, docName, commandID); errMove != nil {
			log.Errorf("Command %v was submitted but failed to move to submitted folder: %v", commandID, errMove.Error())
			continue
		}
	}

	debugMessages, _ := jsonutil.Marshal(messages)
	log.Debugf("Local messages:\n%v", debugMessages)
	return messages, nil
}

// TODO:MF: clean up old documents in dstDir?  Or maybe do that in SendReply?  Maybe both
func moveCommandDocument(srcDir string, dstDir string, docName string, commandID string) error {
	// Make directory with appropriate ACL
	if err := fileutil.MakeDirs(dstDir); err != nil {
		return err
	}
	newName := strings.Join([]string{docName, commandID}, ".")
	if success, err := fileutil.MoveAndRenameFile(srcDir, docName, dstDir, newName); !success {
		// Clean up submitted document that failed to move
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

func (ols *offlineService) SendReply(log log.T, messageID string, payload string) error {
	return nil
}

func (ols *offlineService) FailMessage(log log.T, messageID string, failureType FailureType) error {
	return nil
}

func (ols *offlineService) DeleteMessage(log log.T, messageID string) error {
	return nil
}

func (ols *offlineService) Stop() {}
