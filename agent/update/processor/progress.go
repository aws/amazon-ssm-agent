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

// Package processor contains the methods for update ssm agent.
// It also provides methods for sendReply and updateInstanceInfo
package processor

import (
	"time"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	logPkg "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
)

// inProgress sets update to inProgressing with given new UpdateState
func (u *updateManager) inProgress(updateDetail *UpdateDetail, log logPkg.T, state UpdateState) (err error) {
	defer func() {
		if err != nil {
			failedUpdateDetail := &UpdateDetail{
				State:         Completed,
				Result:        contracts.ResultStatusFailed,
				TargetVersion: updateDetail.TargetVersion,
				SourceVersion: updateDetail.SourceVersion,
			}
			errorCode := u.subStatus + string(state)
			log.WriteEvent(
				logPkg.AgentUpdateResultMessage,
				failedUpdateDetail.SourceVersion,
				PrepareHealthStatus(failedUpdateDetail, errorCode, failedUpdateDetail.TargetVersion))
			if err = u.svc.UpdateHealthCheck(log, failedUpdateDetail, errorCode); err != nil {
				log.Errorf(err.Error())
			}
		}
	}()
	updateDetail.State = state
	updateDetail.Result = contracts.ResultStatusInProgress

	if updateDetail.HasMessageID() && !updateDetail.SelfUpdate {
		err = u.svc.SendReply(log, updateDetail)
		if err != nil {
			log.Errorf(err.Error())
		}
	}

	if err = u.svc.UpdateHealthCheck(log, updateDetail, ""); err != nil {
		log.Errorf(err.Error())
	}
	return nil
}

func (u *updateManager) reportTestFailure(updateDetail *UpdateDetail, log logPkg.T, testOutput string) {
	updateStatus := &UpdateDetail{
		State:         TestExecution,
		Result:        contracts.ResultStatusTestFailure,
		TargetVersion: updateDetail.TargetVersion,
		SourceVersion: updateDetail.SourceVersion,
	}
	if err := u.svc.UpdateHealthCheck(log, updateStatus, testOutput); err != nil {
		log.Errorf("error while sending test failure metric: %v", err.Error())
	}
}

// succeeded sets update to completed
func (u *updateManager) succeeded(updateDetail *UpdateDetail, log logPkg.T) (err error) {
	updateDetail.State = Completed
	updateDetail.Result = contracts.ResultStatusSuccess
	updateDetail.AppendInfo(
		log,
		"%v updated successfully to %v",
		updateDetail.PackageName,
		updateDetail.TargetVersion)

	log.WriteEvent(
		logPkg.AgentUpdateResultMessage,
		updateDetail.SourceVersion,
		PrepareHealthStatus(updateDetail, "", updateDetail.TargetVersion))
	return u.finalize(u, updateDetail, "")
}

// failed sets update to failed with error messages
func (u *updateManager) failed(updateDetail *UpdateDetail, log logPkg.T, code updateconstants.ErrorCode, errMessage string, noRollbackMessage bool) (err error) {
	updateDetail.State = Completed
	updateDetail.Result = contracts.ResultStatusFailed
	updateDetail.AppendInfo(log, errMessage)
	updateDetail.AppendInfo(
		log,
		"Failed to update %v to %v",
		updateDetail.PackageName,
		updateDetail.TargetVersion)

	// Specify no rollback needed
	if noRollbackMessage {
		updateDetail.AppendInfo(log, "No rollback needed")
	}

	errorCode := u.subStatus + string(code)
	log.WriteEvent(
		logPkg.AgentUpdateResultMessage,
		updateDetail.SourceVersion,
		PrepareHealthStatus(updateDetail, errorCode, updateDetail.TargetVersion))
	return u.finalize(u, updateDetail, errorCode)
}

func (u *updateManager) inactive(updateDetail *UpdateDetail, log logPkg.T, errorWarnCode string) (err error) {
	updateDetail.State = Completed
	updateDetail.Result = contracts.ResultStatusSuccess
	updateDetail.AppendInfo(
		log,
		"%v version %v is deprecated/inactive, update skipped",
		updateDetail.PackageName,
		updateDetail.TargetVersion)
	errorWarnCode = u.subStatus + errorWarnCode
	log.WriteEvent(
		logPkg.AgentUpdateResultMessage,
		updateDetail.SourceVersion,
		PrepareHealthStatus(updateDetail, errorWarnCode, updateDetail.TargetVersion))
	return u.finalize(u, updateDetail, errorWarnCode)
}

func (u *updateManager) skipped(updateDetail *UpdateDetail, log logPkg.T) (err error) {
	updateDetail.State = Completed
	updateDetail.Result = contracts.ResultStatusSuccess
	updateDetail.AppendInfo(
		log,
		"update skipped")
	return u.finalize(u, updateDetail, "")
}

// finalizeUpdateAndSendReply completes the update and sends reply to message service, also uploads to S3 (if any)
func finalizeUpdateAndSendReply(u *updateManager, updateDetail *UpdateDetail, errorCode string) (err error) {
	log := u.Context.Log()
	updateDetail.EndDateTime = time.Now().UTC()

	if !updateDetail.SelfUpdate {
		orchestrationDirectory := getOrchestrationDir(u.Context.Identity(), log, updateDetail)
		var filePath string
		filePath, err = fileutil.AppendToFile(orchestrationDirectory, updateDetail.StdoutFileName, updateDetail.StandardOut)
		if err != nil {
			log.Errorf("Error while appending to file %v", filePath)
		}
		if updateDetail.StandardOut, err = fileutil.ReadAllText(filePath); err != nil {
			log.Errorf("Error reading contents from %v", filePath)
		}

		if filePath, err = fileutil.AppendToFile(orchestrationDirectory, updateDetail.StderrFileName, updateDetail.StandardError); err != nil {
			log.Errorf("Error while appending to file %v", filePath)
		}
		if updateDetail.StandardError, err = fileutil.ReadAllText(filePath); err != nil {
			log.Errorf("Error reading contents from %v", filePath)
		}
		// send reply except for self update, don't send any response back to service side for self update
		if updateDetail.HasMessageID() {
			if err = u.svc.SendReply(log, updateDetail); err != nil {
				log.Errorf(err.Error())
			}

			if err = u.svc.DeleteMessage(log, updateDetail); err != nil {
				log.Errorf(err.Error())
			}
		}

		// upload output to s3 bucket
		log.Debugf("output s3 bucket name is %v", updateDetail.OutputS3BucketName)
		if updateDetail.OutputS3BucketName != "" {
			u.ctxMgr.uploadOutput(log, updateDetail, orchestrationDirectory)
		}
	}

	// update health information
	if err = u.svc.UpdateHealthCheck(log, updateDetail, errorCode); err != nil {
		log.Errorf(err.Error())
	}

	if err = u.clean(u, log, updateDetail); err != nil {
		return err
	}

	return nil
}
