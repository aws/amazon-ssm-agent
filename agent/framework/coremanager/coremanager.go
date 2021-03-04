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

// Package coremanager encapsulates the logic for configuring, starting and stopping core modules
package coremanager

import (
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/agentlogstocloudwatch/cloudwatchlogspublisher"
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/framework/coremodules"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/plugin"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/rebooter"
)

const (
	rebootPollingInterval = time.Second
	hardStopTimeout       = time.Second * 5
)

type ICoreManager interface {
	// Start executes the registered core modules
	Start()
	// Stop requests the core modules to stop executing
	Stop()
}

// CoreManager encapsulates the logic for configuring, starting and stopping core modules
type CoreManager struct {
	context             context.T
	coreModules         coremodules.ModuleRegistry
	cloudwatchPublisher *cloudwatchlogspublisher.CloudWatchPublisher
	rebooter            rebooter.IRebootType
}

// NewCoreManager creates a new core module manager.
func NewCoreManager(context context.T, mr coremodules.ModuleRegistry, cwp *cloudwatchlogspublisher.CloudWatchPublisher, rbt rebooter.IRebootType) (cm *CoreManager, err error) {
	log := context.Log()
	shortInstanceId, err := context.Identity().ShortInstanceID()
	if err != nil {
		log.Errorf("error fetching the ShortInstanceID: %v", err)
		return nil, err
	}

	if err = fileutil.HardenDataFolder(); err != nil {
		log.Errorf("error initializing SSM data folder with hardened ACL, %v", err)
		return
	}

	//Initialize all folders where interim states of executing commands will be stored.
	if !initializeBookkeepingLocations(log, shortInstanceId) {
		log.Error("unable to initialize. Exiting")
		return
	}

	// Initialize the client diagnostics
	cwp.Init()
	context = context.With("[instanceID=" + shortInstanceId + "]")
	runpluginutil.SSMPluginRegistry = plugin.RegisteredWorkerPlugins(context)

	return &CoreManager{
		context:             context,
		coreModules:         mr,
		cloudwatchPublisher: cwp,
		rebooter:            rbt,
	}, nil
}

// initializeBookkeepingLocations - initializes all folder locations required for bookkeeping
func initializeBookkeepingLocations(log logger.T, shortInstanceID string) bool {

	//TODO: initializations for all state tracking folders of core modules should be moved inside the corresponding core modules.

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
			shortInstanceID,
			appconfig.DefaultDocumentRootDirName,
			appconfig.DefaultLocationOfState,
			folder)
		//legacy dir, unused
		if folder == appconfig.DefaultLocationOfCompleted {
			log.Info("removing the completed state files")
			fileutil.DeleteDirectory(directoryName)
		}
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
		shortInstanceID,
		appconfig.LongRunningPluginsLocation,
		appconfig.LongRunningPluginDataStoreLocation)

	if err := fileutil.MakeDirs(longRunningPluginsFolderName); err != nil {
		log.Error("encountered error while creating folders for internal state management for long running plugins", err)
		initStatus = false
	}

	log.Infof("Initializing replies folder for MDS reply requests that couldn't reach the service")
	replies := filepath.Join(appconfig.DefaultDataStorePath,
		shortInstanceID,
		appconfig.RepliesRootDirName)

	if err := fileutil.MakeDirs(replies); err != nil {
		log.Error("encountered error while creating folders for MDS replies", err)
		initStatus = false
	}

	log.Infof("Initializing healthcheck folders for long running plugins")
	f := filepath.Join(appconfig.DefaultDataStorePath,
		shortInstanceID,
		appconfig.LongRunningPluginsLocation,
		appconfig.LongRunningPluginsHealthCheck)

	if err := fileutil.MakeDirs(f); err != nil {
		log.Error("encountered error while creating folders for health check for long running plugins", err)
		initStatus = false
	}

	//Create folders for inventory plugin
	log.Infof("Initializing locations for inventory plugin")
	inventoryLocation := filepath.Join(appconfig.DefaultDataStorePath,
		shortInstanceID,
		appconfig.InventoryRootDirName)

	if err := fileutil.MakeDirs(inventoryLocation); err != nil {
		log.Error("encountered error while creating folders for inventory plugin", err)
		initStatus = false
	}

	log.Infof("Initializing default location for custom inventory")
	customInventoryLocation := filepath.Join(appconfig.DefaultDataStorePath,
		shortInstanceID,
		appconfig.InventoryRootDirName,
		appconfig.CustomInventoryRootDirName)

	if err := fileutil.MakeDirs(customInventoryLocation); err != nil {
		log.Error("encountered error while creating folders for custom inventory", err)
		initStatus = false
	}

	log.Infof("Initializing default location for file inventory")
	fileInventoryLocation := filepath.Join(appconfig.DefaultDataStorePath,
		shortInstanceID,
		appconfig.InventoryRootDirName,
		appconfig.FileInventoryRootDirName)

	if err := fileutil.MakeDirs(fileInventoryLocation); err != nil {
		log.Error("encountered error while creating folders for file inventory", err)
		initStatus = false
	}

	log.Infof("Initializing default location for role inventory")
	roleInventoryLocation := filepath.Join(appconfig.DefaultDataStorePath,
		shortInstanceID,
		appconfig.InventoryRootDirName,
		appconfig.RoleInventoryRootDirName)

	if err := fileutil.MakeDirs(roleInventoryLocation); err != nil {
		log.Error("encountered error while creating folders for role inventory", err)
		initStatus = false
	}

	return initStatus
}

// Start executes the registered core modules while watching for reboot request
func (c *CoreManager) Start() {
	go c.watchForReboot()
	c.executeCoreModules()
}

// Stop requests the core modules to stop executing
// Stop would be called by the agent and should be treated as hard stop
func (c *CoreManager) Stop() {
	c.stopCoreModules(contracts.StopTypeHardStop)
}

// executeCoreModules launches all the core modules
func (c *CoreManager) executeCoreModules() {
	l := len(c.coreModules)
	for i := 0; i < l; i++ {
		go func(i int) {
			defer func() {
				if r := recover(); r != nil {
					c.context.Log().Errorf("Execute core modules panic: %v", r)
					c.context.Log().Errorf("Stacktrace:\n%s", debug.Stack())
				}
			}()
			module := c.coreModules[i]
			var err error
			if err = module.ModuleExecute(); err != nil {
				c.context.Log().Errorf("error occurred trying to start core module. Plugin name: %v. Error: %v",
					module.ModuleName(),
					err)
			}
		}(i)
	}
}

// stopCoreModules requests the core modules to stop
func (c *CoreManager) stopCoreModules(stopType contracts.StopType) {
	// use waitgroups in case of softstop to wait for the core modules to finish their work
	// use timeout for hardstop and return control
	log := c.context.Log()
	log.Infof("core manager stop requested. Stop type: %v", stopType)
	var wg sync.WaitGroup
	l := len(c.coreModules)
	for i := 0; i < l; i++ {
		go func(wgc *sync.WaitGroup, i int) {
			defer func() {
				if r := recover(); r != nil {
					log.Errorf("Core module stop request panic: %v", r)
					log.Errorf("Stacktrace:\n%s", debug.Stack())
				}
			}()
			if stopType == contracts.StopTypeSoftStop {
				wgc.Add(1)
				defer wgc.Done()
			}

			module := c.coreModules[i]
			if err := module.ModuleRequestStop(stopType); err != nil {
				log.Errorf("Plugin (%v) failed to stop with error: %v",
					module.ModuleName(),
					err)
			}

		}(&wg, i)
	}

	// use waitgroups in case of softstop to wait for the core modules to finish their work
	// use timeout for hardstop and return control
	if stopType == contracts.StopTypeSoftStop {
		wg.Wait()
	} else {
		time.Sleep(hardStopTimeout)
	}
}

// watchForReboot watches for reboot events and request core modules to stop when necessary
func (c *CoreManager) watchForReboot() {
	log := c.context.Log()

	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Watch for reboot panic: %v", r)
			log.Errorf("Stacktrace:\n%s", debug.Stack())
		}
	}()

	ch := c.rebooter.GetChannel()
	// blocking receive
	val := <-ch
	log.Info("A plugin has requested a reboot.")
	if val == rebooter.RebootRequestTypeReboot {
		log.Info("Processing reboot request...")
		c.stopCoreModules(contracts.StopTypeSoftStop)
		c.rebooter.RebootMachine(log)
	} else {
		log.Error("reboot type not supported yet")
	}

}
