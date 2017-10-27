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
	"sync"

	"path/filepath"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/longrunning/plugin"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

var (
	lock sync.RWMutex
)

// ensurePluginsAreRunning ensures all running plugins are actually running.
func (m *Manager) ensurePluginsAreRunning() {

	log := m.context.Log()

	lock.RLock()
	defer lock.RUnlock()

	if len(m.runningPlugins) > 0 {
		for n := range m.runningPlugins {
			p, isRegistered := m.registeredPlugins[n]
			if isRegistered && !p.Handler.IsRunning(m.context) {
				log.Infof("Starting %s since it wasn't running before")
				//todo: we arent using task pools anymore -> change the following implementation
				m.startPlugin.Submit(m.context.Log(), n, func(cancelFlag task.CancelFlag) {
					instanceID, _ := platform.InstanceID()
					orchestrationRootDir := filepath.Join(
						appconfig.DefaultDataStorePath,
						instanceID,
						appconfig.DefaultDocumentRootDirName,
						m.context.AppConfig().Agent.OrchestrationRootDir)
					orchestrationDir := fileutil.BuildPath(orchestrationRootDir)

					ioConfig := contracts.IOConfiguration{
						OrchestrationDirectory: orchestrationDir,
						OutputS3BucketName:     "",
						OutputS3KeyPrefix:      "",
					}
					out := iohandler.NewDefaultIOHandler(log, ioConfig)
					defer out.Close(log)
					out.Init(log, p.Info.Name)
					p.Handler.Start(m.context, p.Info.Configuration, "", cancelFlag, out)
					out.Close(log)
				})
			}
		}
	} else {
		log.Infof("There are no long running plugins currently getting executed - skipping their healthcheck")
	}
}

// stopLifeCycleManagementJob stops periodic health checks of long running plugins
func (m *Manager) stopLifeCycleManagementJob() {
	if m.managingLifeCycleJob != nil {
		m.managingLifeCycleJob.Quit <- true
	}
}

// RegisteredPlugins loads all registered long running plugins in memory
func RegisteredPlugins(context context.T) map[string]plugin.Plugin {
	return plugin.RegisteredPlugins(context)
}
