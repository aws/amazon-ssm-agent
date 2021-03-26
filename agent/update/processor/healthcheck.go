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
	"fmt"
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/health"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/ssm"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
)

const (
	// active represents agent is active and running
	active = "Active"
	// updateInitialized represents update has initialized
	updateInitialized = "UpdateInitialized"
	// updateStaged represents installation packages are prepared
	updateStaged = "UpdateStaged"
	// updateInProgress represents target version updating
	updateInProgress = "UpdateInprogress"
	// rollingBack represents target version failed to install, rolling back to source version
	rollingBack = "RollingBack"
	// rollBackCompleted represents rolled-back to the source version
	rollBackCompleted = "RollBackCompleted"
	// updateSucceeded represents update is succeeded
	updateSucceeded = "UpdateSucceeded"
	// updateFailed represents update is failed
	updateFailed = "UpdateFailed"
	// testFailed represents tests fail during update
	testFailed = "TestFailed"
	// noAlarm represents suffix which will be added to unimportant error messages
	noAlarm = "NoAlarm"
)

var ssmSvc ssm.Service
var ssmSvcOnce sync.Once

var newSsmSvc = ssm.NewService

// UpdateHealthCheck sends the health check information back to the service
func (s *svcManager) UpdateHealthCheck(log log.T, update *UpdateDetail, errorCode string) (err error) {
	var svc ssm.Service
	if svc, err = getSsmSvc(s.context); err != nil {
		return fmt.Errorf("Failed to load ssm service, %v", err)
	}
	status := PrepareHealthStatus(update, errorCode, update.TargetVersion)
	if _, err = svc.UpdateInstanceInformation(log, update.SourceVersion, status, health.AgentName); err != nil {
		return
	}

	return nil
}

// getSsmSvc loads ssm service
func getSsmSvc(context context.T) (ssm.Service, error) {
	ssmSvcOnce.Do(func() {
		ssmSvc = newSsmSvc(context)
	})

	if ssmSvc == nil {
		return nil, fmt.Errorf("couldn't create ssm service")
	}
	return ssmSvc, nil
}

// prepareHealthStatus prepares health status payload
func PrepareHealthStatus(updateDetail *UpdateDetail, errorCode string, additionalStatus string) (result string) {
	switch updateDetail.State {
	default:
		result = active
	case NotStarted:
		result = active
	case Initialized:
		result = updateInitialized
	case Staged:
		result = updateStaged
	case Installed:
		result = updateInProgress
	case Completed:
		if updateDetail.Result == contracts.ResultStatusFailed {
			result = updateFailed
		}
		if updateDetail.Result == contracts.ResultStatusSuccess {
			result = updateSucceeded
		}
	case TestExecution:
		if updateDetail.Result == contracts.ResultStatusTestFailure {
			result = testFailed
		}
	case Rollback:
		result = rollingBack
	case RolledBack:
		result = rollBackCompleted
	}

	// please maintain the if condition order.
	if updateDetail.SelfUpdate {
		result = fmt.Sprintf("%v_%v", result, updateconstants.SelfUpdatePrefix)
	}

	if len(errorCode) > 0 {
		result = fmt.Sprintf("%v_%v", result, errorCode)
	}

	if len(additionalStatus) > 0 {
		result = fmt.Sprintf("%v-%v", result, additionalStatus)
	}

	if _, ok := updateconstants.NonAlarmingErrors[updateconstants.ErrorCode(errorCode)]; ok {
		result = fmt.Sprintf("%v-%v", result, noAlarm)
	}

	return result
}
