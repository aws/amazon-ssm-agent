// +build darwin freebsd linux netbsd openbsd

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

// Package rundaemon implements rundaemon plugin and its configuration
package rundaemon

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

// Plugin is the type for the configureDaemon plugin.
type Plugin struct {
	iohandler.PluginConfig
	// ExeLocation is the location directory for a particular daemon
	ExeLocation string
	// Name is the name of the daemon
	Name string
	// CommandLine is the command line to launch the daemon (On Windows, ame of executable or a powershell script)
	CommandLine string
}

// IsRunning checks if the daemon is alive
func (p *Plugin) IsRunning(context context.T) bool {
	log := context.Log()
	log.Infof("IsRunning check for daemon %v", p.Name)
	return false // TODO:DAEMON check to see if process is alive (false for now to force regular restarts and see the logs
}

// Start starts the daemon
func (p *Plugin) Start(context context.T, configuration string, orchestrationDir string, cancelFlag task.CancelFlag, out iohandler.IOHandler) error {
	log := context.Log()
	log.Infof("Starting %v Command: %v Config: %v", p.Name, p.CommandLine, configuration)
	return nil
}

// Stop stops the daemon
func (p *Plugin) Stop(context context.T, cancelFlag task.CancelFlag) error {
	log := context.Log()
	log.Infof("Stopping %v", p.Name)
	return nil
}
