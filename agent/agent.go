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

// Package main represents the entry point of the agent.
package main

import (
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"syscall"

	"github.com/aws/amazon-ssm-agent/agent/agent"

	"github.com/aws/amazon-ssm-agent/agent/agentlogstocloudwatch/cloudwatchlogspublisher"
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/framework/coremanager"
	"github.com/aws/amazon-ssm-agent/agent/framework/coremodules"
	"github.com/aws/amazon-ssm-agent/agent/health"
	"github.com/aws/amazon-ssm-agent/agent/hibernation"
	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/rebooter"
	"github.com/aws/amazon-ssm-agent/agent/session/utility"
	"github.com/aws/amazon-ssm-agent/agent/ssm"
)

const (
	activationCodeFlag      = "code"
	activationIDFlag        = "id"
	regionFlag              = "region"
	registerFlag            = "register"
	fingerprintFlag         = "fingerprint"
	similarityThresholdFlag = "similarityThreshold"
)

var (
	instanceIDPtr, regionPtr             *string
	activationCode, activationID, region string
	register, clear, force, fpFlag       bool
	similarityThreshold                  int
	registrationFile                     = filepath.Join(appconfig.DefaultDataStorePath, "registration")
)

func start(log logger.T, instanceIDPtr *string, regionPtr *string, shouldCheckHibernation bool) (ssmAgent agent.ISSMAgent, err error) {
	config, err := appconfig.Config(true)
	if err != nil {
		log.Debugf("appconfig could not be loaded - %v", err)
		return
	}
	context := context.Default(log, config)

	//Reset password for default RunAs user if already exists
	sessionUtil := &utility.SessionUtil{}
	if err := sessionUtil.ResetPasswordIfDefaultUserExists(context); err != nil {
		log.Warnf("Reset password failed, %v", err)
	}

	//Initializing the health module to send empty health pings to the service.
	healthModule := health.NewHealthCheck(context, ssm.NewService())
	hibernateState := hibernation.NewHibernateMode(healthModule, context)

	ssmAgent = agent.NewSSMAgent(context, healthModule, hibernateState)
	// Do a health check before starting the agent.
	// Health check would include creating a health module and sending empty health pings to the service.
	// If response is positive, start the agent, else retry and eventually back off (hibernate/passive mode).
	if status, hibernationErr := healthModule.GetAgentState(); shouldCheckHibernation && status == health.Passive {
		//Starting hibernate mode
		context.Log().Info("Entering SSM Agent hibernate - ", hibernationErr)
		go func() {
			hibernateState.ExecuteHibernation()
			err = startAgent(ssmAgent, context, log, instanceIDPtr, regionPtr)
		}()
	} else {
		err = startAgent(ssmAgent, context, log, instanceIDPtr, regionPtr)
	}
	return
}

func startAgent(ssmAgent agent.ISSMAgent, context context.T, log logger.T, instanceIDPtr *string, regionPtr *string) (err error) {
	cloudwatchPublisher := &cloudwatchlogspublisher.CloudWatchPublisher{}
	coreModules := coremodules.RegisteredCoreModules(context)
	reboot := &rebooter.SSMRebooter{}

	var cpm *coremanager.CoreManager
	if cpm, err = coremanager.NewCoreManager(context, *coreModules, cloudwatchPublisher, instanceIDPtr, regionPtr, log, reboot); err != nil {
		log.Errorf("error occurred when starting core manager: %v", err)
		return
	}
	ssmAgent.SetCoreManager(cpm)

	ssmAgent.Start()
	return
}

func blockUntilSignaled(log logger.T) {
	// Below channel will handle all machine initiated shutdown/reboot requests.

	// Set up channel on which to receive signal notifications.
	// We must use a buffered channel or risk missing the signal
	// if we're not ready to receive when the signal is sent.
	c := make(chan os.Signal, 1)

	// Listening for OS signals is a blocking call.
	// Only listen to signals that require us to exit.
	// Otherwise we will continue execution and exit the program.
	signal.Notify(c, os.Interrupt, os.Kill, syscall.SIGTERM)

	s := <-c
	log.Info("Got signal:", s, " value:", s.Signal)
}

// Run as a single process. Used by Unix systems and when running agent from console.
func run(log logger.T) {
	defer func() {
		// recover in case the agent panics
		// this should handle some kind of seg fault errors.
		if msg := recover(); msg != nil {
			log.Errorf("Agent crashed with message %v!", msg)
			log.Errorf("%s: %s", msg, debug.Stack())
		}
	}()

	// run ssm agent
	agent, err := start(log, instanceIDPtr, regionPtr, true)
	if err != nil {
		log.Errorf("error occurred when starting amazon-ssm-agent: %v", err)
		return
	}
	blockUntilSignaled(log)
	agent.Stop()
}
