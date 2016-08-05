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
	"runtime"
	"syscall"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/framework/coremanager"
	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/version"
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

func start(log logger.T, instanceIDPtr *string, regionPtr *string) (cpm *coremanager.CoreManager, err error) {
	log.Infof("Starting Agent: %v", version.String())
	log.Infof("OS: %s, Arch: %s", runtime.GOOS, runtime.GOARCH)
	log.Flush()

	if cpm, err = coremanager.NewCoreManager(instanceIDPtr, regionPtr, log); err != nil {
		log.Errorf("error occured when starting core manager: %v", err)
		return
	}
	cpm.Start()
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

func stop(log logger.T, cpm *coremanager.CoreManager) {
	log.Info("Stopping agent")
	log.Flush()
	cpm.Stop()
	log.Info("Bye.")
	log.Flush()
}

// Run as a single process. Used by Unix systems and when running agent from console.
func run(log logger.T) {
	// run core manager
	cpm, err := start(log, instanceIDPtr, regionPtr)
	if err != nil {
		log.Errorf("error occured when starting amazon-ssm-agent: %v", err)
		return
	}
	blockUntilSignaled(log)
	stop(log, cpm)
}
