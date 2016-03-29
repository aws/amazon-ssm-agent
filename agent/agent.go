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

// Package main represents the entry point of the agent.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path"
	"runtime"
	"syscall"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/framework/coremanager"
	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/rebooter"
	"github.com/aws/amazon-ssm-agent/agent/version"
)

func main() {
	Start()
}

var log logger.T
var cpm *coremanager.CoreManager

// Start starts the agent.
func Start() {

	// Setup default parameters.
	instanceIDPtr := flag.String("i", "", "instance id")
	regionPtr := flag.String("r", "", "instance region")
	flag.Parse()

	config, err := appconfig.GetConfig(false)
	if err != nil {
		fmt.Println("Could not load config file: ", err)
		return
	}

	log = logger.GetLogger()
	defer log.Flush()

	log.Infof("Starting Agent: %v", version.String())
	log.Infof("OS: %s, Arch: %s", runtime.GOOS, runtime.GOARCH)
	log.Flush()

	region, err := platform.SetRegion(log, *regionPtr)
	if err != nil {
		log.Error("please specify the region to use.")
		return
	}
	log.Debug("Using region:", region)

	instanceID, err := platform.SetInstanceID(log, *instanceIDPtr)
	if err != nil {
		log.Error("please specify at least one instance id.")
		return
	}

	//Initialize all folders where interim states of executing commands will be stored.
	if !initializeBookkeepingLocations(log, instanceID) {
		log.Error("unable to initialize. Exiting")
		Stop()
	}

	// create a reboot channel to handle reboot request from core plugins
	rebootChan := make(chan int)

	// starting core plugin manager
	cpm = coremanager.NewCoreManager(instanceID, config, log, rebootChan)
	cpm.Start()

	// listen on the channel for reboot requests from the framework/CorePluginManager
	go reboot(<-rebootChan)

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
	log.Info("Stopping agent")
	Stop()
}

//Stop the agent.
func Stop() {
	// stop the core plugin manager
	if cpm != nil {
		cpm.Stop()
	}

	log.Info("Bye.")
	log.Flush()
	log.Close()
	os.Exit(130) // Terminated by user
}

func reboot(code int) {
	log.Info("Processing reboot request...")
	if rebooter.RebootRequested() && !rebooter.RebootInitiated() {
		rebooter.RebootMachine(log)
	}
}

// initializeBookkeepingLocations - initializes all folder locations required for bookkeeping
func initializeBookkeepingLocations(log logger.T, instanceID string) bool {
	//Create folders pending, current, completed, corrupt under the location DefaultDataStorePath/<instanceId>

	log.Info("Initializing bookkeeping folders")
	initStatus := true
	//Parent folder in linux => /var/lib/amazon/ssm
	folders := []string{
		appconfig.DefaultLocationOfPending,
		appconfig.DefaultLocationOfCurrent,
		appconfig.DefaultLocationOfCompleted,
		appconfig.DefaultLocationOfCorrupt}

	for _, folder := range folders {

		directoryName := path.Join(appconfig.DefaultDataStorePath,
			instanceID,
			appconfig.DefaultCommandRootDirName,
			appconfig.DefaultLocationOfState,
			folder)

		err := fileutil.MakeDirs(directoryName)
		if err != nil {
			log.Error("encountered error while creating folders for internal state management", err)
			initStatus = false
			break
		}
	}

	return initStatus
}
