// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Amazon Software License (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/asl/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package processor contains the methods for update ssm agent.
// It also provides methods for sendReply and updateInstanceInfo
package processor

import (
	"fmt"
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/ssm"
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
)

var ssmSvc ssm.Service
var ssmSvcOnce sync.Once

var newSsmSvc = ssm.NewService

// Use SetInstanceID to load InstanceID with empty string as defaultID
var getInstanceID = platform.SetInstanceID

// UpdateHealthCheck sends the health check information back to the service
func (s *svcManager) UpdateHealthCheck(log log.T, update *UpdateDetail, errorCode string) (err error) {
	var svc ssm.Service
	var instanceID string

	if svc, err = getSsmSvc(log); err != nil {
		return fmt.Errorf("failed to load ssm service %v", err)
	}
	status := prepareHealthStatus(update, errorCode)

	if instanceID, err = getInstanceID(log, ""); err != nil {
		return fmt.Errorf("failed to load instance ID %v", err)
	}

	if _, err = svc.UpdateInstanceInformation(log, instanceID, update.SourceVersion, status); err != nil {
		return
	}

	return nil
}

// getSsmSvc loads ssm service
func getSsmSvc(log log.T) (service ssm.Service, err error) {
	ssmSvcOnce.Do(func() {
		ssmSvc = newSsmSvc(log)
	})

	if ssmSvc == nil {
		return nil, fmt.Errorf("couldn't create ssm service")
	}
	return ssmSvc, nil
}

// prepareHealthStatus prepares health status payload
func prepareHealthStatus(update *UpdateDetail, errorCode string) (result string) {
	switch update.State {
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
		if update.Result == contracts.ResultStatusFailed {
			result = updateFailed
		}
		if update.Result == contracts.ResultStatusSuccess {
			result = updateSucceeded
		}
	case Rollback:
		result = rollingBack
	case RolledBack:
		result = rollBackCompleted
	}

	if len(errorCode) > 0 {
		result = fmt.Sprintf("%v_%v", result, errorCode)
	}

	return result
}
