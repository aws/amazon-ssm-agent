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

// Package service is a wrapper for the SSM Message Delivery Service
package service

import (
	reqContext "context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/network"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/amazon-ssm-agent/common/identity"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssmmds"
	"github.com/aws/aws-sdk-go/service/ssmmds/ssmmdsiface"
	"github.com/twinj/uuid"
)

// FailureType is used for failure types.
type FailureType string

const (
	// InternalHandlerException signifies an error while running a plugin.
	InternalHandlerException FailureType = "InternalHandlerException"

	// NoHandlerExists signifies that there is no plugin for a given name.
	NoHandlerExists FailureType = "NoHandlerExists"

	// QuickResponseThreshold is the threshold time - any api response that comes before this (time in seconds) is treated as fast response
	QuickResponseThreshold = 10
)

// Service is an interface to the MDS service.
type Service interface {
	GetMessages(log log.T, instanceID string) (messages *ssmmds.GetMessagesOutput, err error)
	AcknowledgeMessage(log log.T, messageID string) error
	SendReply(log log.T, messageID string, payload string) error
	SendReplyWithInput(log log.T, sendReply *ssmmds.SendReplyInput) error
	FailMessage(log log.T, messageID string, failureType FailureType) error
	DeleteMessage(log log.T, messageID string) error
	LoadFailedReplies(log log.T) []string
	DeleteFailedReply(log log.T, replyId string)
	PersistFailedReply(log log.T, sendReply ssmmds.SendReplyInput) error
	GetFailedReply(log log.T, replyId string) (*ssmmds.SendReplyInput, error)
	Stop()
}

type SendSdkRequest func(req *request.Request) error
type CancelSdkRequest func(trans *http.Transport, req *request.Request)

// sdkService is an service wrapper that delegates to the ssm sdk.
type sdkService struct {
	context          context.T
	sdk              ssmmdsiface.SsmmdsAPI
	tr               *http.Transport
	cancelRequest    reqContext.CancelFunc
	m                sync.Mutex
	sendSdkRequest   SendSdkRequest
	cancelSdkRequest CancelSdkRequest
}

var clientBasedErrorMessages, serverBasedErrorMessages []string

// NewService creates a new MDS service instance.
func NewService(context context.T, connectionTimeout time.Duration) Service {

	config := sdkutil.AwsConfig(context, "ec2messages")
	agentConfig := context.AppConfig()

	if agentConfig.Agent.Region != "" {
		config.Region = &agentConfig.Agent.Region
	}

	if agentConfig.Mds.Endpoint != "" {
		config.Endpoint = &agentConfig.Mds.Endpoint
	}

	// capture Transport so we can use it to cancel requests
	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout:   connectionTimeout,
			KeepAlive: 0,
		}).Dial,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     network.GetDefaultTLSConfig(context.Log(), context.AppConfig()),
	}
	config.HTTPClient = &http.Client{Transport: tr, Timeout: connectionTimeout}

	sess := session.New(config)
	sess.Handlers.Build.PushBack(request.MakeAddToUserAgentHandler(agentConfig.Agent.Name, agentConfig.Agent.Version))

	msgSvc := ssmmds.New(sess)

	//adding server based expected error messages
	serverBasedErrorMessages = make([]string, 0, 2)
	serverBasedErrorMessages = append(serverBasedErrorMessages, "use of closed network connection")
	serverBasedErrorMessages = append(serverBasedErrorMessages, "connection reset by peer")

	//adding client based expected error messages
	clientBasedErrorMessages = make([]string, 0, 1)
	clientBasedErrorMessages = append(clientBasedErrorMessages, "Client.Timeout exceeded while awaiting headers")

	sendMdsSdkRequest := func(req *request.Request) error {
		return req.Send()
	}
	cancelMdsSDKRequest := func(trans *http.Transport, req *request.Request) {
		trans.CancelRequest(req.HTTPRequest)
	}

	return NewMdsSdkService(context, msgSvc, tr, sendMdsSdkRequest, cancelMdsSDKRequest)
}

func NewMdsSdkService(context context.T, msgSvc ssmmdsiface.SsmmdsAPI, tr *http.Transport, sendMdsSdkRequest SendSdkRequest, cancelMdsSDKRequest CancelSdkRequest) Service {
	return &sdkService{context: context, sdk: msgSvc, tr: tr, sendSdkRequest: sendMdsSdkRequest, cancelSdkRequest: cancelMdsSDKRequest}
}

// GetMessages calls the GetMessages MDS API.
func (mds *sdkService) GetMessages(log log.T, instanceID string) (messages *ssmmds.GetMessagesOutput, err error) {
	uuid.SwitchFormat(uuid.CleanHyphen)
	uid := uuid.NewV4().String()
	params := &ssmmds.GetMessagesInput{
		Destination:                aws.String(instanceID), // Required
		MessagesRequestId:          aws.String(uid),        // Required
		VisibilityTimeoutInSeconds: aws.Int64(10),
	}
	log.Debug("Calling GetMessages with params", params)
	requestTime := time.Now()
	req, messages := mds.sdk.GetMessagesRequest(params)
	if requestErr := mds.sendRequest(req); requestErr != nil {
		log.Debug(requestErr)
		if isErrorUnexpected(log, requestErr, requestTime, time.Now()) {
			//GetMessages api responded with unexpected errors - we must return this as error
			err = fmt.Errorf("GetMessages Error: %v", requestErr)
			log.Debug(err)
		}
	} else {
		log.Debug("GetMessages Response", messages)
	}
	return
}

// isErrorUnexpected processes GetMessages errors and determines if its unexpected error
func isErrorUnexpected(log log.T, err error, requestTime, responseTime time.Time) bool {
	//determine the time it took for the api to respond
	timeDiff := responseTime.Sub(requestTime).Seconds()
	if strings.Contains(err.Error(), "request context canceled") {
		log.Debugf("MDS request cancelled in between")
		return false
	}
	//check if response isn't coming too quick & if error is unexpected
	if timeDiff < QuickResponseThreshold {
		//response was too quick - this is unexpected
		return true
	}

	//response wasn't too quick
	//checking if the class of errors are expected
	if isServerBasedError(err.Error()) {
		log.Warnf("Server terminated MDS long poll connection after %v seconds", timeDiff)
		return false
	} else if isClientBasedError(err.Error()) {
		log.Warnf("Client terminated MDS long poll connection after %v seconds due to no response", timeDiff)
		return false
	} else {
		//errors are truly unexpected
		return true
	}
}

// isServerBasedError returns true if and only if the error is server related
func isServerBasedError(message string) bool {
	for _, m := range serverBasedErrorMessages {
		if strings.Contains(message, m) {
			return true
		}
	}
	return false
}

// isClientBasedError returns true if and only if the error is client related
func isClientBasedError(message string) bool {
	for _, m := range clientBasedErrorMessages {
		if strings.Contains(message, m) {
			return true
		}
	}
	return false
}

// AcknowledgeMessage calls AcknowledgeMessage MDS API.
func (mds *sdkService) AcknowledgeMessage(log log.T, messageID string) (err error) {
	params := &ssmmds.AcknowledgeMessageInput{
		MessageId: aws.String(messageID), // Required
	}
	log.Debug("Calling AcknowledgeMessage with params", params)
	req, resp := mds.sdk.AcknowledgeMessageRequest(params)
	if err = mds.sendRequest(req); err != nil {
		err = fmt.Errorf("AcknowledgeMessage Error: %v", err)
		log.Debug(err)
	} else {
		log.Debug("AcknowledgeMessage Response", resp)
	}
	return
}

// SendReplyWithInput calls SendReply MDS API given SendReplyInput object
func (mds *sdkService) SendReplyWithInput(log log.T, sendReply *ssmmds.SendReplyInput) (err error) {
	log.Debug("Calling SendReply with params", sendReply)
	req, resp := mds.sdk.SendReplyRequest(sendReply)
	if err = mds.sendRequest(req); err != nil {
		err = fmt.Errorf("SendReply Error: %v", err)
		log.Debug(err)
	} else {
		log.Info("SendReply Response", resp)
	}
	return
}

// SendReply transforms payload into SendReplyInput object and calls SendReplyWithInput.
func (mds *sdkService) SendReply(log log.T, messageID string, payload string) (err error) {
	uuid.SwitchFormat(uuid.CleanHyphen)
	replyID := uuid.NewV4().String()
	replyInput := ssmmds.SendReplyInput{
		MessageId: aws.String(messageID), // Required
		Payload:   aws.String(payload),   // Required
		ReplyId:   aws.String(replyID),   // Required
	}
	if err = mds.SendReplyWithInput(log, &replyInput); err != nil {
		log.Infof("Saving reply %v to local disk", replyID)
		mds.PersistFailedReply(log, replyInput)
	}
	return
}

// FailMessage calls the FailMessage MDS API.
func (mds *sdkService) FailMessage(log log.T, messageID string, failureType FailureType) (err error) {
	params := &ssmmds.FailMessageInput{
		FailureType: aws.String(string(failureType)), // Required
		MessageId:   aws.String(messageID),           // Required
	}
	log.Debug("Calling FailMessage with params", params)
	req, resp := mds.sdk.FailMessageRequest(params)
	if err = mds.sendRequest(req); err != nil {
		err = fmt.Errorf("FailMessage Error: %v", err)
		log.Debug(err)
	} else {
		log.Debug("FailMessage Response", resp)
	}
	return
}

// DeleteMessage calls the DeleteMessage MDS API.
func (mds *sdkService) DeleteMessage(log log.T, messageID string) (err error) {
	params := &ssmmds.DeleteMessageInput{
		MessageId: aws.String(messageID), // Required
	}
	log.Debug("Calling DeleteMessage with params", params)
	req, resp := mds.sdk.DeleteMessageRequest(params)
	if err = mds.sendRequest(req); err != nil {
		err = fmt.Errorf("DeleteMessage Error: %v", err)
		log.Debug(err)
	} else {
		log.Debug("DeleteMessage Response", resp)
	}
	return
}

// LoadFailedReplies loads SendReplyInput objects from local replies folder on disk
func (mds *sdkService) LoadFailedReplies(log log.T) []string {
	log.Debug("Checking Replies folder for failed sent replies")
	shortInstanceID, _ := mds.context.Identity().ShortInstanceID()
	absoluteDirPath := path.Join(appconfig.DefaultDataStorePath,
		shortInstanceID,
		appconfig.RepliesRootDirName)

	files, err := fileutil.GetFileNames(absoluteDirPath)
	if err != nil {
		log.Errorf("encountered error %v while listing replies in %v", err, absoluteDirPath)
	}
	return files
}

// DeleteFailedReply deletes failed reply from local replies folder on disk
func (mds *sdkService) DeleteFailedReply(log log.T, fileName string) {
	absoluteFileName := getFailedReplyLocation(mds.context.Identity(), fileName)
	if fileutil.Exists(absoluteFileName) {
		err := fileutil.DeleteFile(absoluteFileName)
		if err != nil {
			log.Errorf("encountered error %v while deleting file %v", err, absoluteFileName)
		} else {
			log.Debugf("successfully deleted file %v", absoluteFileName)
		}
	}
}

// PersistFailedReply saves SendReplyInput object to local replies folder on disk
func (mds *sdkService) PersistFailedReply(log log.T, sendReply ssmmds.SendReplyInput) (err error) {
	content, err := jsonutil.Marshal(sendReply)
	if err != nil {
		log.Errorf("encountered error with message %v while marshalling %v to string", err, sendReply)
	} else {
		files, _ := fileutil.GetFileNames(GetFailedReplyDirectory(mds.context.Identity()))
		for _, file := range files {
			if strings.HasPrefix(file, *sendReply.ReplyId) {
				log.Debugf("Reply %v already saved in file %v, skipping", *sendReply.ReplyId, file)
				return
			}
		}
		t := time.Now().UTC()
		fileName := fmt.Sprintf("%v_%v", *sendReply.ReplyId, t.Format("2006-01-02T15-04-05"))
		absoluteFileName := getFailedReplyLocation(mds.context.Identity(), fileName)

		log.Tracef("persisting reply %v in file %v", jsonutil.Indent(content), absoluteFileName)
		if s, err := fileutil.WriteIntoFileWithPermissions(absoluteFileName, jsonutil.Indent(content), os.FileMode(int(appconfig.ReadWriteAccess))); s && err == nil {
			log.Debugf("successfully persisted reply in %v", absoluteFileName)
		} else {
			log.Debugf("persisting reply in %v failed with error %v", absoluteFileName, err)
		}
	}
	return err
}

// GetFailedReply load SendReplyInput object from replies folder given the reply id of the object
func (mds *sdkService) GetFailedReply(log log.T, fileName string) (*ssmmds.SendReplyInput, error) {
	absoluteFileName := getFailedReplyLocation(mds.context.Identity(), fileName)

	var sendReply ssmmds.SendReplyInput
	err := jsonutil.UnmarshalFile(absoluteFileName, &sendReply)
	if err != nil {
		log.Errorf("encountered error with message %v while reading reply input from file - %v", err, absoluteFileName)
	} else {
		//logging reply as read from the file
		jsonString, err := jsonutil.Marshal(sendReply)
		if err != nil {
			log.Errorf("encountered error with message %v while marshalling %v to string", err, sendReply)
		} else {
			log.Tracef("Send reply input read from file-system - %v", jsonutil.Indent(jsonString))
		}
	}
	return &sendReply, err
}

// Stop stops this service so that any blocked calls wake up.
func (mds *sdkService) Stop() {
	mds.m.Lock()
	defer mds.m.Unlock()
	if mds.cancelRequest != nil {
		// cancel the underlying http request to wake up the last call
		mds.cancelRequest()
	}
	mds.context.Log().Infof("Stopped Mds service")
}

// sendRequest wraps req.Send() so that it can keep track of the executing request
func (mds *sdkService) sendRequest(req *request.Request) error {
	mds.storeRequest(req)
	defer mds.clearRequest()
	return mds.sendSdkRequest(req)
}

func (mds *sdkService) storeRequest(req *request.Request) {
	mds.m.Lock()
	defer mds.m.Unlock()
	if req == nil {
		mds.cancelRequest = nil
		return
	}
	var cancel reqContext.CancelFunc
	ctx, cancel := reqContext.WithCancel(reqContext.Background())
	req.SetContext(ctx)
	mds.cancelRequest = cancel
}

func (mds *sdkService) clearRequest() {
	mds.storeRequest(nil)
}

// getFailedReplyLocation returns path to reply file
func getFailedReplyLocation(identity identity.IAgentIdentity, fileName string) string {
	return path.Join(GetFailedReplyDirectory(identity), fileName)
}

// GetFailedReplyDirectory returns path to replies folder
func GetFailedReplyDirectory(identity identity.IAgentIdentity) string {
	shortInstanceID, _ := identity.ShortInstanceID()
	return path.Join(appconfig.DefaultDataStorePath,
		shortInstanceID,
		appconfig.RepliesRootDirName)
}
