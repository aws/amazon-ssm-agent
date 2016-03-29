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

// Package health contains routines that periodically reports health information of the agent
package health

import (
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/amazon-ssm-agent/agent/ssm"
	"github.com/aws/amazon-ssm-agent/agent/version"
	"github.com/carlescere/scheduler"
)

// HealthCheck encapsulates the logic on configuring, starting and stopping core plugins
type HealthCheck struct {
	contracts.ICorePlugin
	context               context.T
	healthCheckStopPolicy *sdkutil.StopPolicy
	instanceID            string
	healthJob             *scheduler.Job
}

const (
	name = "HealthCheck"
)

// NewHealthCheck creates a new health check core plugin.
func NewHealthCheck(context context.T) *HealthCheck {
	healthContext := context.With("[" + name + "]")
	healthCheckStopPolicy := sdkutil.NewStopPolicy(name, 10)

	var instanceID string
	var err error
	if instanceID = platform.InstanceID(); instanceID == "" {
		err = fmt.Errorf("failed to get instance id")
		healthContext.Log().Error(err)
		return nil
	}

	return &HealthCheck{
		context:               healthContext,
		healthCheckStopPolicy: healthCheckStopPolicy,
		instanceID:            instanceID,
	}
}

// updates SSM with the instance health information
func (h *HealthCheck) updateHealth() {
	log := h.context.Log()
	log.Infof("%s reporting agent health.", name)
	if h.healthCheckStopPolicy != nil {
		if h.healthCheckStopPolicy.IsHealthy() == false {
			if err := h.RequestStop(contracts.StopTypeSoftStop); err != nil {
				log.Errorf("failed to stop %v plugin. %v", name, err)
			}
		}
	}

	var err error
	var svc ssm.Service
	if svc = ssm.NewService(log); svc == nil {
		err = fmt.Errorf("unable to create ssm service")
		sdkutil.HandleAwsError(log, err, h.healthCheckStopPolicy)
		return
	}

	// TODO, when will status become inactive?
	// If both ssm config and command is inactive => agent is inactive.
	if _, err = svc.UpdateInstanceInformation(log, h.instanceID, version.Version, "Active"); err != nil {
		sdkutil.HandleAwsError(log, err, h.healthCheckStopPolicy)
	}
	return
}

// CorePlugin Run Schedule In Minutes
func (h *HealthCheck) scheduleInMinutes() int {
	updateHealthFrequencyMins := 5
	config := h.context.AppConfig()
	log := h.context.Log()

	if 4 < config.Ssm.HealthFrequencyMinutes || config.Ssm.HealthFrequencyMinutes < 61 {
		updateHealthFrequencyMins = config.Ssm.HealthFrequencyMinutes
	} else {
		log.Debug("HealthFrequencyMinutes is outside allowable limits. Limiting to 5 minutes default.")
	}
	log.Debugf("%v frequency is every %d minutes.", name, updateHealthFrequencyMins)

	return updateHealthFrequencyMins
}

// ICorePlugin implementation

// Name returns the Plugin Name
func (h *HealthCheck) Name() string {
	return name
}

// Execute starts the scheduling of the health check plugin
func (h *HealthCheck) Execute(context context.T) (err error) {
	if h.healthJob, err = scheduler.Every(h.scheduleInMinutes()).Minutes().Run(h.updateHealth); err != nil {
		context.Log().Errorf("unable to schedule health update. %v", err)
	}
	return
}

// RequestStop handles the termination of the health check plugin job
func (h *HealthCheck) RequestStop(stopType contracts.StopType) (err error) {
	if h.healthJob != nil {
		h.context.Log().Info("stopping update instance health job.")
		h.healthJob.Quit <- true
	}
	return nil
}
