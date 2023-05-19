// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package app represents the core SSM agent object
package app

import (
	"runtime"
	"time"

	agentcontracts "github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/version"
	"github.com/aws/amazon-ssm-agent/core/app/context"
	"github.com/aws/amazon-ssm-agent/core/app/credentialrefresher"
	reboot "github.com/aws/amazon-ssm-agent/core/app/reboot/model"
	"github.com/aws/amazon-ssm-agent/core/app/registrar"
	"github.com/aws/amazon-ssm-agent/core/app/selfupdate"
	"github.com/aws/amazon-ssm-agent/core/ipc/messagebus"
	"github.com/aws/amazon-ssm-agent/core/workerprovider/longrunningprovider"
)

type CoreAgent interface {
	Start(statusChan *agentcontracts.StatusComm) error
	Stop()
}

// SSMCoreAgent encapsulates the core functionality of the agent
type SSMCoreAgent struct {
	context        context.ICoreAgentContext
	container      longrunningprovider.IContainer
	selfupdate     selfupdate.ISelfUpdate
	credsRefresher credentialrefresher.ICredentialRefresher
	registrar      registrar.IRetryableRegistrar
}

// NewSSMCoreAgent creates and returns and object of type CoreAgent interface
func NewSSMCoreAgent(context context.ICoreAgentContext, messageBus messagebus.IMessageBus) CoreAgent {
	coreAgent := &SSMCoreAgent{
		context:        context,
		container:      longrunningprovider.NewWorkerContainer(context, messageBus),
		selfupdate:     selfupdate.NewSelfUpdater(context),
		credsRefresher: credentialrefresher.NewCredentialRefresher(context),
	}

	if registrar := registrar.NewRetryableRegistrar(context); registrar != nil {
		coreAgent.registrar = registrar
	}

	return coreAgent
}

// Start the core manager
func (agent *SSMCoreAgent) Start(statusChan *agentcontracts.StatusComm) error {
	log := agent.context.Log()

	log.Infof("amazon-ssm-agent - %v", version.String())
	log.Infof("OS: %s, Arch: %s", runtime.GOOS, runtime.GOARCH)
	log.Info("Starting Core Agent")

	if agent.registrar != nil {
		log.Info("registrar detected. Attempting registration")
		if err := agent.registrar.Start(); err != nil {
			return err
		}
	}

	if err := agent.credsRefresher.Start(); err != nil {
		return err
	}

	credentialsReadyChan := agent.credsRefresher.GetCredentialsReadyChan()
	select {
	case <-credentialsReadyChan:
		log.Debug("Agent core module started after receiving credentials")
		close(credentialsReadyChan)
		agent.container.Start()
		go agent.container.Monitor()
		agent.selfupdate.Start()
		// removing the below wait time will cause the agent worker to run orphaned when
		// agent is stopped immediately after start
		time.Sleep(3 * time.Second)
		break
	case <-statusChan.TerminationChan:
		log.Info("Received stop/termination signal from main routine")
		break
	}
	statusChan.DoneChan <- struct{}{}
	return nil
}

// Stop the core manager
func (agent *SSMCoreAgent) Stop() {
	log := agent.context.Log()
	log.Info("Stopping Core Agent")
	log.Flush()

	agent.selfupdate.Stop()
	agent.container.Stop(reboot.StopTypeHardStop)
	agent.credsRefresher.Stop()
	if agent.registrar != nil {
		agent.registrar.Stop()
	}

	log.Info("Bye.")
	log.Flush()
}
