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

// Package health contains routines that periodically reports health information of the agent
package health

import (
	"math/rand"
	"runtime/debug"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/amazon-ssm-agent/agent/ssm"
	"github.com/aws/amazon-ssm-agent/agent/version"
	"github.com/carlescere/scheduler"
)

type IHealthCheck interface {
	ModuleName() string
	ModuleExecute() (err error)
	ModuleRequestStop(stopType contracts.StopType) (err error)
	GetAgentState() (a AgentState, err error)
}

// HealthCheck encapsulates the logic on configuring, starting and stopping core modules
type HealthCheck struct {
	context               context.T
	healthCheckStopPolicy *sdkutil.StopPolicy
	healthJob             *scheduler.Job
	service               ssm.Service
}

const (
	name = "HealthCheck"
	// AgentName is the name of the current agent.
	AgentName = "amazon-ssm-agent"
)

var healthModule *HealthCheck

// AgentState enumerates active and passive agentMode
type AgentState int32

const (
	//Active would suggest the agent is going to start with full capacity since SSM can be reached
	Active AgentState = 1
	//Passive would suggest that the agent is in Backoff and the health will be checked based on current capacity
	Passive AgentState = 0
)

// NewHealthCheck creates a new health check core module.
// Only one health core module must exist at a time
func NewHealthCheck(context context.T, svc ssm.Service) *HealthCheck {
	if healthModule != nil {
		context.Log().Debug("Health process has already been initialized.")
		return healthModule
	}
	healthContext := context.With("[" + name + "]")
	healthCheckStopPolicy := sdkutil.NewStopPolicy(name, 10)

	healthModule = &HealthCheck{
		context:               healthContext,
		healthCheckStopPolicy: healthCheckStopPolicy,
		service:               svc,
	}
	return healthModule
}

// schedules recurrent updateHealth calls
func (h *HealthCheck) scheduleUpdateHealth() {
	var err error
	if h.healthJob, err = scheduler.Every(h.scheduleInMinutes()).Minutes().Run(h.updateHealth); err != nil {
		h.context.Log().Errorf("unable to schedule health update. %v", err)
	}
	return
}

// updates SSM with the instance health information
func (h *HealthCheck) updateHealth() {
	log := h.context.Log()
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Update health panic: \n%v", r)
			log.Errorf("Stacktrace:\n%s", debug.Stack())
		}
	}()

	log.Infof("%s reporting agent health.", name)

	var err error
	//TODO when will status become inactive?
	// If both ssm config and command is inactive => agent is inactive.
	if _, err = h.service.UpdateInstanceInformation(log, version.Version, "Active", AgentName); err != nil {
		sdkutil.HandleAwsError(log, err, h.healthCheckStopPolicy)
	}

	if !h.healthCheckStopPolicy.IsHealthy() {
		h.service = ssm.NewService(h.context)
		h.healthCheckStopPolicy.ResetErrorCount()
	}

	return
}

// scheduleInMinutes Run Schedule In Minutes
func (h *HealthCheck) scheduleInMinutes() int {
	updateHealthFrequencyMins := 5
	config := h.context.AppConfig()
	log := h.context.Log()
	// Appconstants contain default run-time constants
	constants := h.context.AppConstants()

	if constants.MinHealthFrequencyMinutes <= config.Ssm.HealthFrequencyMinutes && config.Ssm.HealthFrequencyMinutes <= constants.MaxHealthFrequencyMinutes {
		updateHealthFrequencyMins = config.Ssm.HealthFrequencyMinutes
	} else {
		log.Debug("HealthFrequencyMinutes is outside allowable limits. Limiting to 5 minutes default.")
	}
	log.Debugf("%v frequency is every %d minutes.", name, updateHealthFrequencyMins)

	return updateHealthFrequencyMins
}

// ICoreModule implementation

// ModuleName returns the module name
func (h *HealthCheck) ModuleName() string {
	return name
}

// ModuleExecute starts the scheduling of the health check module
func (h *HealthCheck) ModuleExecute() (err error) {
	defer func() {
		if msg := recover(); msg != nil {
			h.context.Log().Errorf("health check ModuleExecute run panic: %v", msg)
		}
	}()
	rand.Seed(time.Now().UTC().UnixNano())
	scheduleInMinutes := h.scheduleInMinutes()

	randomSeconds := rand.Intn(scheduleInMinutes * 60)

	// First call updateHealth once
	go h.updateHealth()

	// Wait randomSeconds and schedule recurrent updateHealth calls
	next := time.Duration(randomSeconds) * time.Second
	go func(h *HealthCheck) {
		select {
		case <-time.After(next):
			go h.scheduleUpdateHealth()
		}
	}(h)

	return
}

// ModuleRequestStop handles the termination of the health check module job
func (h *HealthCheck) ModuleRequestStop(stopType contracts.StopType) (err error) {
	if h.healthJob != nil {
		h.context.Log().Info("stopping update instance health job.")
		h.healthJob.Quit <- true
	}
	return nil
}

//ping sends an empty ping to the health service to identify if the service exists
func (h *HealthCheck) ping() (err error) {
	if h.healthCheckStopPolicy.HasError() {
		h.service = ssm.NewService(h.context)
		h.healthCheckStopPolicy.ResetErrorCount()
	}

	_, err = h.service.UpdateEmptyInstanceInformation(h.context.Log(), version.Version, AgentName)
	if err != nil {
		h.healthCheckStopPolicy.AddErrorCount(1)
	}
	return err
}

// GetAgentState returns the state of the agent. It is the caller's responsibility to log the error
func (h *HealthCheck) GetAgentState() (a AgentState, err error) {
	if err = h.ping(); err != nil {
		return Passive, err
	}
	return Active, err
}
