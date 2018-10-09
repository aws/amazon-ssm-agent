// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package agent represents the core SSM agent object
package agent

import (
	"runtime"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/framework/coremanager"
	"github.com/aws/amazon-ssm-agent/agent/health"
	"github.com/aws/amazon-ssm-agent/agent/hibernation"
	"github.com/aws/amazon-ssm-agent/agent/version"
)

type ISSMAgent interface {
	SetCoreManager(cm coremanager.ICoreManager)
	SetContext(c context.T)
	Start()
	Stop()
	Hibernate()
}

// SSMAgent encapsulates the core functionality of the agent
type SSMAgent struct {
	context        context.T
	coreManager    coremanager.ICoreManager
	healthModule   health.IHealthCheck
	hibernateState hibernation.IHibernate
}

// NewSSMAgent creates and returns and object of type SSMAgent interface
func NewSSMAgent(c context.T, hm health.IHealthCheck, hs hibernation.IHibernate) ISSMAgent {
	return &SSMAgent{
		context:        c,
		healthModule:   hm,
		hibernateState: hs,
	}
}

// SetCoreManager sets the coreManager for the agent, initializing the agent doesn't include the core manager
// This enables hibernation check before initializing the coreManager
// This is needed so we can check for hibernation before we start creating the agent dependencies
func (agent *SSMAgent) SetCoreManager(cm coremanager.ICoreManager) {
	if cm != nil {
		agent.coreManager = cm
	}
}

func (agent *SSMAgent) SetContext(c context.T) {
	if c != nil {
		agent.context = c
	}
}

// Start the core manager
func (agent *SSMAgent) Start() {
	log := agent.context.Log()

	log.Infof("Starting Agent: %v", version.String())
	log.Infof("OS: %s, Arch: %s", runtime.GOOS, runtime.GOARCH)
	log.Flush()

	if agent.coreManager == nil {
		log.Errorf("Agent's core manager can't be nil")
		return
	}

	agent.coreManager.Start()
}

// Hibernate checks if the agent should hibernate when it can't reach the service
func (agent *SSMAgent) Hibernate() {
	if status, err := agent.healthModule.GetAgentState(); status == health.Passive {
		//Starting hibernate mode
		agent.context.Log().Info("Entering SSM Agent hibernate - ", err)
		agent.hibernateState.ExecuteHibernation()
	}
}

// Stop the core manager
func (agent *SSMAgent) Stop() {
	log := agent.context.Log()
	log.Info("Stopping agent")
	log.Flush()

	if agent.coreManager == nil {
		return
	}

	agent.coreManager.Stop()
	log.Info("Bye.")
	log.Flush()
}
