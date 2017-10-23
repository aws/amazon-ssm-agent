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

// Package manager encapsulates everything related to long running plugin manager that starts, stops & configures long running plugins
package manager

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/longrunning/plugin"
	"github.com/aws/amazon-ssm-agent/agent/longrunning/plugin/cloudwatch"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

//this file contains all methods that will be called by LRPM invoker (a worker plugin) when prompted by SSM Config or MDS core modules.

//todo: we are passing m.Context to p.Handler.Start & p.Handler.Stop -> we might want have to change StartPlugin and StopPlugin to accept context directly
//todo: honor the cancel flag for both Start and Stop plugin functions

//StopPlugin stops a given plugin from executing
func (m *Manager) StopPlugin(name string, cancelFlag task.CancelFlag) (err error) {

	//todo: if plugin wasn't even running then stop will have no effect -> for those cases we can return something for a better plugin level status

	lock.Lock()
	defer lock.Unlock()

	log := m.context.Log()
	p, isRegisteredPlugin := m.registeredPlugins[name]
	_, isRunningPlugin := m.runningPlugins[name]

	if isRegisteredPlugin && isRunningPlugin {
		//stop the plugin
		if err = p.Handler.Stop(m.context, cancelFlag); err != nil {
			// check if cloud watch exe process has been terminated manually
			if p.Handler.IsRunning(m.context) {
				log.Errorf("Failed to stop long running plugin - %s because of %s", name, err)
				return
			}
		}
		//remove the entry from the map of running plugins
		delete(m.runningPlugins, name)

		if err = dataStore.Write(m.runningPlugins); err != nil {
			log.Errorf("Failed to update datastore - because of %s", err)
		}

		// Update the config file to "IsEnabled": "false"
		if err = cloudwatch.Instance().Disable(); err != nil {
			log.Errorf("Failed to update config file - because of %s", err)
		}

		return
	}

	log.Debugf("Can't stop %s - since its not even running", name)
	return nil
}

//StartPlugin starts the given plugin with the given configuration
func (m *Manager) StartPlugin(name, configuration string, orchestrationDir string, cancelFlag task.CancelFlag, out iohandler.IOHandler) (err error) {
	lock.Lock()
	defer lock.Unlock()

	log := m.context.Log()
	log.Infof("Starting long running plugin - %s", name)

	//check if the plugin is registered - this is an extra check since ideally we expect invoker to be aware of registered plugins.
	var p plugin.Plugin
	var isRegisteredPlugin bool
	if p, isRegisteredPlugin = m.registeredPlugins[name]; !isRegisteredPlugin {
		err = fmt.Errorf("unable to run %s since it's not even registered", name)
		return
	}

	//set the config path of the long running plugin
	p.Info.Configuration = configuration
	if err = p.Handler.Start(m.context, p.Info.Configuration, orchestrationDir, cancelFlag, out); err != nil {
		log.Errorf("Failed to start long running plugin - %s because of %s", name, err)
		return
	}

	//edit the plugin info
	p.Info.State = plugin.PluginState{
		LastConfigurationModifiedTime: time.Now(),
		IsEnabled:                     true,
	}

	// TODO move persisting out of executing logic
	m.runningPlugins[name] = p.Info
	log.Debugf("Persisting info about %s in datastore", p.Info.Name)

	// TODO separate persist part and actual running part
	if err = dataStore.Write(m.runningPlugins); err != nil {
		err = fmt.Errorf("Failed to persist info about %s in datastore because : %s", p.Info.Name, err.Error())
		log.Errorf(err.Error())
	}

	// Update the config file with new configuration
	var engineConfigurationParser cloudwatch.EngineConfigurationParser
	json.Unmarshal([]byte(p.Info.Configuration), &engineConfigurationParser)
	if err = cloudwatch.Instance().Enable(engineConfigurationParser.EngineConfiguration); err != nil {
		log.Errorf("Failed to update config file - because of %s", err)
	}

	return
}
