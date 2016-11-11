package service

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

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
	submittedCommandDir := filepath.Join(newCommandDir, "submitted")
	invalidCommandDir := filepath.Join(newCommandDir, "invalid")

	// Look for unprocessed locally submitted documents
	var docName, docPath string
	var filenames []string
	if filenames, err = fileutil.GetFileNames(newCommandDir); err != nil {
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
			moveCommandDocument(newCommandDir, invalidCommandDir, docName, commandID)
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
		moveCommandDocument(newCommandDir, submittedCommandDir, docName, commandID)
	}

	debugMessages, _ := jsonutil.Marshal(messages)
	log.Debugf("Local messages:\n%v", debugMessages)
	return messages, nil
}

func moveCommandDocument(srcDir string, dstDir string, docName string, commandID string) {
	fileutil.MakeDirs(dstDir)
	newName := strings.Join([]string{docName, commandID}, ".")
	fileutil.RenameFile(srcDir, docName, newName)
	fileutil.MoveFile(newName, srcDir, dstDir)
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
