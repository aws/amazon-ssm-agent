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

// Package coremanager encapsulates the logic for configuring, starting and stopping core plugins
package coremanager

import (
	"path/filepath"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/framework/coreplugins"
	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/rebooter"
)

const (
	rebootPollingInterval = time.Second
	hardStopTimeout       = time.Second * 5
)

// CoreManager encapsulates the logic for configuring, starting and stopping core plugins
type CoreManager struct {
	context     context.T
	corePlugins coreplugins.PluginRegistry
}

// NewCoreManager creates a new core plugin manager.
func NewCoreManager(instanceIdPtr *string, regionPtr *string, log logger.T) (cm *CoreManager, err error) {

	// initialize appconfig
	var config appconfig.SsmagentConfig
	if config, err = appconfig.Config(false); err != nil {
		log.Errorf("Could not load config file: %v", err)
		return
	}

	// initialize region
	if *regionPtr != "" {
		if err = platform.SetRegion(*regionPtr); err != nil {
			log.Errorf("error occured setting the region, %v", err)
			return
		}
	}

	var region string
	if region, err = platform.Region(); err != nil {
		log.Errorf("error fetching the region, %v", err)
		return
	}
	log.Debug("Using region:", region)

	// initialize instance ID
	if *instanceIdPtr != "" {
		if err = platform.SetInstanceID(*instanceIdPtr); err != nil {
			log.Errorf("error occured setting the instance ID, %v", err)
			return
		}
	}

	var instanceId string
	if instanceId, err = platform.InstanceID(); err != nil {
		log.Errorf("error fetching the instanceID, %v", err)
		return
	}
	log.Debug("Using instanceID:", instanceId)

	if err = fileutil.HardenDataFolder(); err != nil {
		log.Errorf("error initializing SSM data folder with hardened ACL, %v", err)
		return
	}

	//Initialize all folders where interim states of executing commands will be stored.
	if !initializeBookkeepingLocations(log, instanceId) {
		log.Error("unable to initialize. Exiting")
		return
	}

	context := context.Default(log, config).With("[instanceID=" + instanceId + "]")
	corePlugins := coreplugins.RegisteredCorePlugins(context)

	return &CoreManager{
		context:     context,
		corePlugins: *corePlugins,
	}, nil
}

// initializeBookkeepingLocations - initializes all folder locations required for bookkeeping
func initializeBookkeepingLocations(log logger.T, instanceID string) bool {

	//TODO: initializations for all state tracking folders of core plugins should be moved inside the corresponding core plugins.

	//Create folders pending, current, completed, corrupt under the location DefaultLogDirPath/<instanceId>
	log.Info("Initializing bookkeeping folders")
	initStatus := true
	folders := []string{
		appconfig.DefaultLocationOfPending,
		appconfig.DefaultLocationOfCurrent,
		appconfig.DefaultLocationOfCompleted,
		appconfig.DefaultLocationOfCorrupt}

	for _, folder := range folders {

		directoryName := filepath.Join(appconfig.DefaultDataStorePath,
			instanceID,
			appconfig.DefaultCommandRootDirName,
			appconfig.DefaultLocationOfState,
			folder)

		err := fileutil.MakeDirs(directoryName)
		if err != nil {
			log.Errorf("Encountered error while creating folders for internal state management. %v", err)
			initStatus = false
			break
		}
	}

	//Create folders for long running plugins
	log.Infof("Initializing bookkeeping folders for long running plugins")
	longRunningPluginsFolderName := filepath.Join(appconfig.DefaultDataStorePath,
		instanceID,
		appconfig.LongRunningPluginsLocation,
		appconfig.LongRunningPluginDataStoreLocation)

	if err := fileutil.MakeDirs(longRunningPluginsFolderName); err != nil {
		log.Error("encountered error while creating folders for internal state management for long running plugins", err)
		initStatus = false
	}

	log.Infof("Initializing healthcheck folders for long running plugins")
	f := filepath.Join(appconfig.DefaultDataStorePath,
		instanceID,
		appconfig.LongRunningPluginsLocation,
		appconfig.LongRunningPluginsHealthCheck)

	if err := fileutil.MakeDirs(f); err != nil {
		log.Error("encountered error while creating folders for health check for long running plugins", err)
		initStatus = false
	}

	return initStatus
}

// Start executes the registered core plugins while watching for reboot request
func (c *CoreManager) Start() {
	go c.watchForReboot()
	c.executeCorePlugins()
}

// Stop requests the core plugins to stop executing
// Stop would be called by the agent and should be treated as hard stop
func (c *CoreManager) Stop() {
	c.stopCorePlugins(contracts.StopTypeHardStop)
}

// executeCorePlugins launches all the core plugins
func (c *CoreManager) executeCorePlugins() {
	var wg sync.WaitGroup
	l := len(c.corePlugins)
	for i := 0; i < l; i++ {
		go func(wgc *sync.WaitGroup, i int) {
			wgc.Add(1)
			defer wgc.Done()

			plugin := c.corePlugins[i]
			var err error
			if err = plugin.Execute(c.context); err != nil {
				c.context.Log().Errorf("error occured trying to start core plugin. Plugin name: %v. Error: %v",
					plugin.Name(),
					err)
			}
		}(&wg, i)
	}
	wg.Wait()
}

// stopCorePlugins requests the core plugins to stop
func (c *CoreManager) stopCorePlugins(stopType contracts.StopType) {
	// use waitgroups in case of softstop to wait for the core plugins to finish their work
	// use timeout for hardstop and return control
	log := c.context.Log()
	log.Infof("core manager stop requested. Stop type: %v", stopType)
	var wg sync.WaitGroup
	l := len(c.corePlugins)
	for i := 0; i < l; i++ {
		go func(wgc *sync.WaitGroup, i int) {
			if stopType == contracts.StopTypeSoftStop {
				wgc.Add(1)
				defer wgc.Done()
			}

			plugin := c.corePlugins[i]
			if err := plugin.RequestStop(stopType); err != nil {
				log.Errorf("Plugin (%v) failed to stop with error: %v",
					plugin.Name(),
					err)
			}

		}(&wg, i)
	}

	// use waitgroups in case of softstop to wait for the core plugins to finish their work
	// use timeout for hardstop and return control
	if stopType == contracts.StopTypeSoftStop {
		wg.Wait()
	} else {
		time.Sleep(hardStopTimeout)
	}
}

// watchForReboot watches for reboot events and request core plugins to stop when necessary
func (c *CoreManager) watchForReboot() {
	log := c.context.Log()
	for {
		// check if there is any pending reboot request
		if rebooter.RebootRequested() {
			// on reboot request, stop core plugins and request agent to initiate reboot.
			c.context.Log().Info("A plugin has requested a reboot.")
			c.stopCorePlugins(contracts.StopTypeSoftStop)
			break
		}

		// wait for a second before checking again
		time.Sleep(rebootPollingInterval)
	}

	log.Info("Processing reboot request...")
	if rebooter.RebootRequested() && !rebooter.RebootInitiated() {
		rebooter.RebootMachine(log)
	}
}
