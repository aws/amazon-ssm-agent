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
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
)

// inProgress sets update to inProgressing with given new UpdateState
func (u *updateManager) inProgress(context *UpdateContext, log log.T, state UpdateState) (err error) {
	update := context.Current
	update.State = state
	update.Result = contracts.ResultStatusInProgress

	// resolve context location base on the UpdateRoot
	contextLocation := updateutil.UpdateContextFilePath(update.UpdateRoot)
	if err = u.ctxMgr.saveUpdateContext(log, context, contextLocation); err != nil {
		return err
	}

	if update.HasMessageID() {
		err = u.svc.SendReply(log, update)
		if err != nil {
			log.Errorf(err.Error())
		}
	}

	if err = u.svc.UpdateHealthCheck(log, update, ""); err != nil {
		log.Errorf(err.Error())
	}

	return nil
}

// succeeded sets update to completed
func (u *updateManager) succeeded(context *UpdateContext, log log.T) (err error) {
	update := context.Current
	update.State = Completed
	update.Result = contracts.ResultStatusSuccess
	update.AppendInfo(
		log,
		"%v updated successfully to %v",
		update.PackageName,
		update.TargetVersion)

	return u.finalizeUpdateAndSendReply(log, context, "")
}

// failed sets update to failed with error messages
func (u *updateManager) failed(context *UpdateContext, log log.T, code updateutil.ErrorCode, errMessage string, noRollbackMessage bool) (err error) {
	update := context.Current
	update.State = Completed
	update.Result = contracts.ResultStatusFailed
	update.AppendInfo(log, errMessage)
	update.AppendInfo(
		log,
		"Failed to update %v to %v",
		update.PackageName,
		update.TargetVersion)

	// Specify no rollback needed
	if noRollbackMessage {
		update.AppendInfo(log, "No rollback needed")
	}

	return u.finalizeUpdateAndSendReply(log, context, string(code))
}

// finalizeUpdateAndSendReply completes the update and sends reply to message service, also uploads to S3 (if any)
func (u *updateManager) finalizeUpdateAndSendReply(log log.T, context *UpdateContext, errorCode string) (err error) {
	update := context.Current
	update.EndDateTime = time.Now().UTC()
	// resolve context location base on the UpdateRoot
	contextLocation := updateutil.UpdateContextFilePath(update.UpdateRoot)
	if err = u.ctxMgr.saveUpdateContext(log, context, contextLocation); err != nil {
		return err
	}

	orchestrationDirectory := getOrchestrationDir(log, update)
	filePath, err := fileutil.AppendToFile(orchestrationDirectory, update.StdoutFileName, update.StandardOut)
	if err != nil {
		log.Errorf("Error while appending to file %v", filePath)
	}
	if update.StandardOut, err = fileutil.ReadAllText(filePath); err != nil {
		log.Errorf("Error reading contents from %v", filePath)
	}

	if filePath, err = fileutil.AppendToFile(orchestrationDirectory, update.StderrFileName, update.StandardError); err != nil {
		log.Errorf("Error while appending to file %v", filePath)
	}
	if update.StandardError, err = fileutil.ReadAllText(filePath); err != nil {
		log.Errorf("Error reading contents from %v", filePath)
	}
	// send reply
	if update.HasMessageID() {
		if err = u.svc.SendReply(log, update); err != nil {
			log.Errorf(err.Error())
		}

		if err = u.svc.DeleteMessage(log, update); err != nil {
			log.Errorf(err.Error())
		}
	}

	// update health information
	if err = u.svc.UpdateHealthCheck(log, update, errorCode); err != nil {
		log.Errorf(err.Error())
	}

	// upload output to s3 bucket
	log.Debugf("output s3 bucket name is %v", update.OutputS3BucketName)
	if update.OutputS3BucketName != "" {
		u.ctxMgr.uploadOutput(log, context, orchestrationDirectory)
	}

	context.cleanUpdate()
	if err = u.ctxMgr.saveUpdateContext(log, context, contextLocation); err != nil {
		return err
	}

	return nil
}
