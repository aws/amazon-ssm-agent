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
// either express or implied. See the License for the specific language governing`
// permissions and limitations under the License.

// Package mgsinteractor will be responsible for interacting with MGS
package mgsinteractor

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/interactor/mgsinteractor/replytypes"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/utils"
	"github.com/aws/amazon-ssm-agent/common/identity"
	"github.com/carlescere/scheduler"
	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/websocket"
	"github.com/twinj/uuid"
)

const (
	failedReplyProcessingLimit = 50
	updateSuffix               = "update"
)

var (
	writeIntoFile = fileutil.WriteIntoFileWithPermissions
	getFileNames  = fileutil.GetFileNames
)

// loadFailedReplies loads failed replies from local mgs replies folder on disk
func (mgs *MGSInteractor) loadFailedReplies(log log.T) []string {
	log.Debug("Checking MGS Replies folder for failed sent replies")
	absoluteDirPath := getFailedReplyDirectory(mgs.context.Identity())
	files, err := getFileNames(absoluteDirPath)
	if err != nil {
		log.Errorf("encountered error %v while listing mgs replies in %v", err, absoluteDirPath)
	}
	return files
}

// deleteFailedReply deletes failed mgs replies from local replies folder on disk
func (mgs *MGSInteractor) deleteFailedReply(log log.T, fileName string) {
	mgs.handledUpdateReplies.LoadAndDelete(fileName)

	absoluteFileName := getFailedReplyLocation(mgs.context.Identity(), fileName)
	if fileutil.Exists(absoluteFileName) {
		err := fileutil.DeleteFile(absoluteFileName)
		if err != nil {
			log.Errorf("encountered error %v while deleting file %v", err, absoluteFileName)
		} else {
			log.Debugf("successfully deleted file %v", absoluteFileName)
		}
	}
}

// sendFailedReplies loads replies from local disk and send it again to the service, if it fails no action is needed
func (mgs *MGSInteractor) sendFailedReplies() {
	log := mgs.context.Log()
	log.Debug("send failed reply thread started")
	defer func() {
		log.Debug("send failed reply thread done")
		if msg := recover(); msg != nil {
			log.Errorf("sendFailedReplies panicked: %v", msg)
			log.Errorf("Stacktrace:\n%s", debug.Stack())
		}
	}()

	log.Debug("Checking if there are document replies that failed to reach the service, and retry sending them")
	unfilteredReplies := mgs.loadFailedReplies(log)
	replies := mgs.filterReplies(unfilteredReplies)

	// this check denotes that either the list failed replies failed or have no values
	if len(replies) == 0 {
		log.Debugf("No failed document replies found")
		return
	}
	replyProcessingLimit := failedReplyProcessingLimit
	log.Info("Found document replies that need to be sent to the service")
	for _, reply := range replies {
		log.Debug("Loading reply ", reply)
		docPersistData, err := mgs.getFailedReply(log, reply)
		if err != nil {
			log.Errorf("encountered error with message %v while reading reply input from file - %v", err, reply)
			continue
		}
		// sending it at least once after the first failure
		if utils.IsValidReplyRequest(reply, contracts.MessageGatewayService) == false && docPersistData.RetryNumber > 1 {
			log.Debug("Reply is old, document execution must have timed out. Deleting the reply")
			mgs.deleteFailedReply(log, reply)
			continue
		}
		replyUUID, err := uuid.Parse(docPersistData.ReplyId)
		if err != nil {
			log.Errorf("error while parsing reply uuid %v", err)
			continue
		}

		replyObject, err := replytypes.GetReplyTypeObject(mgs.context, docPersistData.AgentResult, replyUUID, docPersistData.RetryNumber) // initializes reply object
		if err != nil {
			log.Errorf("error while constructing reply object %v", err)
			continue
		}
		agentReplyContract := &agentReplyLocalContract{
			documentResult: replyObject,
			backupFile:     reply,
			retryNumber:    docPersistData.RetryNumber,
		}
		// added to reduce the load on the reply thread
		if !mgs.isChannelOpenForAgentJobMsgs() {
			break
		}
		mgs.sendReplyProp.reply <- agentReplyContract
		replyProcessingLimit--
		if replyProcessingLimit == 0 {
			log.Infof("failed reply processing ended")
			break
		}
	}
}

func (mgs *MGSInteractor) isSendFailedReplyJobScheduled() bool {
	mgs.mutex.Lock()
	defer mgs.mutex.Unlock()
	return mgs.sendReplyProp.sendFailedReplyJob != nil
}

func (mgs *MGSInteractor) startSendFailedReplyJob() {
	var err error
	log := mgs.context.Log()
	mgs.mutex.Lock()
	defer mgs.mutex.Unlock()
	if mgs.sendReplyProp.sendFailedReplyJob == nil {
		if mgs.sendReplyProp.sendFailedReplyJob, err = scheduler.Every(utils.SendFailedReplyFrequencyMinutes).Minutes().Run(mgs.sendFailedReplies); err != nil {
			log.Errorf("unable to schedule send failed reply job. %v", err)
		}
	}
}

// startUpdateReplyFileWatcher starts up a file watcher that awaits update replies.
func (mgs *MGSInteractor) startUpdateReplyFileWatcher() {
	log := mgs.context.Log()
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Errorf("Error initializing the update reply watcher: %v", err)
		return
	}
	defer func() {
		if err := recover(); err != nil {
			log.Errorf("Update reply watcher panic: %v", err)
		}
		err := watcher.Close()
		if err != nil {
			log.Errorf("Error closing the update reply watcher: %v", err)
		}
		log.Infof("Update reply watcher closed")
	}()

	err = watcher.Add(getFailedReplyDirectory(mgs.context.Identity()))
	if err != nil {
		log.Errorf("Error adding directory to watcher: %v", err)
	}

	log.Debug("Starting MGS update reply file watcher")
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				log.Debug("Update file watcher closed")
				return
			}
			if mgs.isUpdateWriteEvent(event) {
				err := mgs.sendUpdateReply(filepath.Base(event.Name))
				if err != nil {
					log.Errorf("Error while sending update reply: %v", err)
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				log.Debug("Update file watcher closed")
				return
			}
			log.Errorf("Error in MGS update reply file watcher: %v", err)
		case <-mgs.updateWatcherDone:
			return
		}
	}
}

// stopUpdateReplyFileWatcher initiates update reply file watcher close down.
func (mgs *MGSInteractor) stopUpdateReplyFileWatcher() {
	mgs.updateWatcherDone <- true
}

// sendUpdateReply reads the update reply from the replies directory and pushes it onto the reply channel.
func (mgs *MGSInteractor) sendUpdateReply(fileName string) (err error) {
	log := mgs.context.Log()
	log.Infof("Reading MGS update reply: %v", fileName)
	if _, ok := mgs.handledUpdateReplies.Load(fileName); ok {
		return
	}
	mgs.handledUpdateReplies.Store(fileName, true)

	docPersistData, err := mgs.getFailedReply(log, fileName)
	if err != nil {
		return fmt.Errorf("error while getting stored reply: %v", err)
	}
	replyUUID, err := uuid.Parse(docPersistData.ReplyId)
	if err != nil {
		return fmt.Errorf("error while parsing UUID: %v", err)
	}

	replyObject, err := replytypes.GetReplyTypeObject(mgs.context, docPersistData.AgentResult, replyUUID, docPersistData.RetryNumber)
	if err != nil {
		return fmt.Errorf("error while getting reply type object: %v", err)
	}
	agentReplyContract := &agentReplyLocalContract{
		documentResult: replyObject,
		backupFile:     fileName,
		retryNumber:    docPersistData.RetryNumber,
	}
	if mgs.isChannelOpenForAgentJobMsgs() {
		mgs.sendReplyProp.reply <- agentReplyContract
	}
	return err
}

func (mgs *MGSInteractor) closeSendFailedReplyJob() {
	mgs.mutex.Lock()
	defer mgs.mutex.Unlock()
	if mgs.sendReplyProp.sendFailedReplyJob != nil {
		mgs.sendReplyProp.sendFailedReplyJob.Quit <- true
	}
}

// getFailedReply load documentResultPersistData object from replies folder given the message id of the object
func (mgs *MGSInteractor) getFailedReply(log log.T, fileName string) (*AgentResultLocalStoreData, error) {
	var sendReply AgentResultLocalStoreData
	absoluteFileName := getFailedReplyLocation(mgs.context.Identity(), fileName)
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

// getFailedReplyLocation returns path to reply file
func getFailedReplyLocation(identity identity.IAgentIdentity, fileName string) string {
	return path.Join(getFailedReplyDirectory(identity), fileName)
}

// persistResult saves agent message in the local disk
func (mgs *MGSInteractor) persistResult(replyBytes AgentResultLocalStoreData) (err error) {
	log := mgs.context.Log()
	log.Debugf("persisting result %+v", replyBytes)
	content, err := jsonutil.Marshal(replyBytes)
	if err != nil {
		log.Errorf("encountered error with message %v while marshalling %v to string", err)
	} else {
		files, _ := getFileNames(getFailedReplyDirectory(mgs.context.Identity()))
		persistTime := time.Now().UTC()
		fileName := fmt.Sprintf("%v_%v", persistTime.Format("2006-01-02T15-04-05"), replyBytes.ReplyId) //changing the format a bit from MDS replies to support proper sorting
		for fileIndex := len(files) - 1; fileIndex >= 0; fileIndex-- {
			file := files[fileIndex]
			if strings.HasSuffix(file, replyBytes.ReplyId) {
				log.Debugf("updating the file %v as reply %v is already present", replyBytes.ReplyId, file)
				fileName = file
				break
			}
		}
		absoluteFileName := getFailedReplyLocation(mgs.context.Identity(), fileName)
		log.Tracef("persisting reply %v in file %v", jsonutil.Indent(content), absoluteFileName)
		if s, err := writeIntoFile(absoluteFileName, jsonutil.Indent(content), os.FileMode(appconfig.ReadWriteAccess)); s && err == nil {
			log.Debugf("successfully persisted reply in %v", absoluteFileName)
		} else {
			log.Debugf("persisting reply in %v failed with error %v", absoluteFileName, err)
		}
	}
	return err
}

// getFailedReplyDirectory returns path to mgs replies folder
func getFailedReplyDirectory(identity identity.IAgentIdentity) string {
	shortInstanceID, _ := identity.ShortInstanceID()
	return path.Join(appconfig.DefaultDataStorePath,
		shortInstanceID,
		appconfig.RepliesMGSRootDirName)
}

// processReply processes the reply received from the reply queue
func (mgs *MGSInteractor) processReply(result *agentReplyLocalContract) {
	// send reply
	replyAckChan := make(chan bool, 1)
	docResult := result.documentResult
	agentMessageUUID := docResult.GetMessageUUID().String()
	log := mgs.context.Log()
	mgs.sendReplyProp.replyAckChan.Store(agentMessageUUID, replyAckChan)
	totalNoOfRetries := docResult.GetNumberOfContinuousRetries()
	log.Debugf("started reply processing - %v", agentMessageUUID)
	defer log.Debugf("ended reply processing - %v", agentMessageUUID)
	log.Tracef("reply received for processing %+v", result)

externalLoop:
	// currently, continuous retry is applicable only for agent_complete messages
	for retryNo := 0; retryNo < totalNoOfRetries; retryNo++ {
		// increment retries count
		docResult.IncrementRetries()
		err := mgs.sendReplyToMGS(docResult)
		persist := AgentResultLocalStoreData{
			AgentResult: docResult.GetResult(),
			ReplyId:     docResult.GetMessageUUID().String(),
			RetryNumber: docResult.GetRetryNumber(),
		}
		if mgs.isTempError(err) { // do not retry or wait when we see these errors
			log.Debugf("skipping wait after send reply due to the following temporary error %v", err)
			mgs.persistResult(persist)
			break
		}
		if err != nil {
			log.Errorf("error while sending reply %v to MGS - %v ", agentMessageUUID, err)
		}
		select {
		case <-time.After(time.Duration(docResult.GetBackOffSecond()) * time.Second):
			if docResult.ShouldPersistData() && ((retryNo + 1) == totalNoOfRetries) {
				log.Warnf("no ack received while sending reply %v", agentMessageUUID)
				persist.RetryNumber = docResult.GetRetryNumber()
				mgs.persistResult(persist)
			}
		case <-replyAckChan:
			log.Debugf("received reply ack id %v", agentMessageUUID)
			if result.backupFile != "" {
				mgs.deleteFailedReply(log, result.backupFile)
			}
			break externalLoop
		}
	}
	mgs.sendReplyProp.replyAckChan.Delete(agentMessageUUID)
}

// startReplyProcessingQueue starts the reply goroutine threads when the reply is received and sends it to MGS
func (mgs *MGSInteractor) startReplyProcessingQueue() {
	replyThreadCount := 0
	logger := mgs.context.Log()
	logger.Debugf("started reply processing queue")
	defer func() {
		logger.Debugf("ended reply processing queue")
		if r := recover(); r != nil {
			logger.Errorf("reply queue handler panic: \n%v", r)
			logger.Errorf("Stacktrace:\n%s", debug.Stack())
			time.Sleep(2 * time.Second)
			go mgs.startReplyProcessingQueue()
		}
	}()
exitLoopLabel:
	for {
		// If there are too many reply threads currently running, wait for any of them to free up
		if replyThreadCount >= mgs.sendReplyProp.replyQueueLimit {
			logger.Debug("maximum reply threads are running right now. Waiting for one of them to end")
			<-mgs.sendReplyProp.replyThreadDone
			logger.Debug("one of the reply thread completed. proceeding to the next reply")
			replyThreadCount--
		}

		select {
		case res, ok := <-mgs.sendReplyProp.reply:
			if !ok {
				logger.Info("Reply queue has been closed")
				break exitLoopLabel
			}
			commandId := res.documentResult.GetResult().MessageID
			logger.Debugf("Got reply msg Id %s for %v %v, starting reply thread", res.documentResult.GetMessageUUID().String(), res.documentResult.GetName(), commandId)
			replyThreadCount++
			go func(resLocalContract *agentReplyLocalContract) {
				defer func() {
					if r := recover(); r != nil {
						logger.Errorf("reply processing queue panic: \n%v", r)
						logger.Errorf("Stacktrace:\n%s", debug.Stack())
					}
				}()
				defer mgs.resultProcessingDone()
				mgs.processReply(resLocalContract)
			}(res)
		case <-mgs.sendReplyProp.replyThreadDone:
			logger.Debug("reply sending done")
			replyThreadCount--
		}
	}

	// Wait for all replies to complete
	for replyThreadCount != 0 {
		<-mgs.sendReplyProp.replyThreadDone
		logger.Debug("reply completed")
		replyThreadCount--
	}
	mgs.sendReplyProp.allReplyClosed <- struct{}{}
	logger.Info("all replies done")
}

// resultProcessingDone pushes to replyThreadDone chan to tell the reply queue
// that the reply processing has been done
func (mgs *MGSInteractor) resultProcessingDone() {
	logger := mgs.context.Log()
	logger.Debugf("result processing done")
	mgs.sendReplyProp.replyThreadDone <- struct{}{}
}

// sendReplyToMGS send replies to MGS
func (mgs *MGSInteractor) sendReplyToMGS(result replytypes.IReplyType) error {
	log := mgs.context.Log()
	var err error
	agentMessage, err := result.ConvertToAgentMessage()
	if err != nil {
		return fmt.Errorf("error while converting to agent message: %v", err)
	}
	msg, err := agentMessage.Serialize(log)
	if err != nil {
		return fmt.Errorf("error while serializing agent message: %v", err)
	}
	if mgs.controlChannel != nil {
		if err = mgs.controlChannel.SendMessage(log, msg, websocket.BinaryMessage); err != nil {
			err = fmt.Errorf("error while sending agent reply message, ID [%v], err: %v", result.GetMessageUUID().String(), err) // modify
		} else {
			log.Infof("successfully sent reply message id: %s", result.GetMessageUUID().String()) //modify
		}
		return err
	}
	return fmt.Errorf("control channel is not open")
}

// isUpdateWriteEvent returns if a file event is a write to a file with the suffix "update."
func (mgs *MGSInteractor) isUpdateWriteEvent(event fsnotify.Event) bool {
	return event.Op&fsnotify.Write == fsnotify.Write && strings.HasSuffix(event.Name, updateSuffix)
}

// filterReplies filters out replies that are meant for the update file watcher or already handled.
func (mgs *MGSInteractor) filterReplies(unfilteredReplies []string) (replies []string) {
	for i := range unfilteredReplies {
		if !strings.HasSuffix(unfilteredReplies[i], updateSuffix) {
			replies = append(replies, unfilteredReplies[i])
		} else if _, ok := mgs.handledUpdateReplies.Load(unfilteredReplies[i]); !ok {
			mgs.handledUpdateReplies.Store(unfilteredReplies[i], true)
			replies = append(replies, unfilteredReplies[i])
		}
	}
	return replies
}

func (mgs *MGSInteractor) isTempError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "ws not initialized still")
}
