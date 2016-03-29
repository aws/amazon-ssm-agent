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

// Package coremanager encapsulates the logic for configuring, starting and stopping core plugins
package coremanager

import (
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/coreplugins"
	"github.com/aws/amazon-ssm-agent/agent/log"
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
	rebootChan  chan int
}

// NewCoreManager creates a new core plugin manager.
func NewCoreManager(instanceID string, config appconfig.T, log log.T, rebootChan chan int) *CoreManager {
	context := context.Default(log, config).With("[instanceID=" + instanceID + "]")
	corePlugins := coreplugins.RegisteredCorePlugins(context)

	return &CoreManager{
		context:     context,
		corePlugins: *corePlugins,
		rebootChan:  rebootChan,
	}
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
	for {
		// check if there is any pending reboot request
		if rebooter.RebootRequested() {
			// on reboot request, stop core plugins and request agent to initiate reboot.
			c.context.Log().Info("A plugin has requested a reboot.")
			c.stopCorePlugins(contracts.StopTypeSoftStop)
			c.rebootChan <- 0
			break
		}

		// wait for a second before checking again
		time.Sleep(rebootPollingInterval)
	}
}
