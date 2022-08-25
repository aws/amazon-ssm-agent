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

// Package process wraps up the os.Process interface and also provides os-specific process lookup functions
package proc

import (
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/identity"
	"github.com/aws/amazon-ssm-agent/common/runtimeconfig"
)

var (
	getAppConfig                         = appconfig.Config
	newAgentIdentity                     = identity.NewAgentIdentity
	runtimeConfigClientCreator           = runtimeconfig.NewIdentityRuntimeConfigClient
	defaultAgentIdentitySelectorCreator  = identity.NewDefaultAgentIdentitySelector
	runtimeConfigIdentitySelectorCreator = identity.NewRuntimeConfigIdentitySelector
)

// OSProcess is an abstracted interface of os.Process
type OSProcess interface {
	//generic ssm visible fields
	Pid() int
	StartTime() time.Time
	//kill the attached child process
	Kill() error
	//wait for the child to finish, if parent dies halfway, the child process is detached and becomes orphan
	//On Unix system, orphan is not be killed by systemd or upstart by default
	//On Windows service controller, stop service does not kill orphan by default
	//TODO confirm MSI Installer does not kill the child process
	Wait() error
}

// impl of OSProcess with os.Process embed
type WorkerProcess struct {
	*exec.Cmd
	startTime time.Time
}

func (p *WorkerProcess) Pid() int {
	return p.Cmd.Process.Pid
}

func (p *WorkerProcess) StartTime() time.Time {
	return p.startTime
}

// TODO use the kill functions provided in executes package
func (p *WorkerProcess) Kill() error {
	return p.Cmd.Process.Kill()
}

func (p *WorkerProcess) Wait() error {
	return p.Cmd.Wait()
}

// start a child process, with the resources attached to its parent
func StartProcess(name string, argv []string) (OSProcess, error) {
	//TODO connect stdin and stdout to avoid seelog error
	cmd := exec.Command(name, argv...)
	prepareProcess(cmd)
	err := cmd.Start()
	p := WorkerProcess{
		cmd,
		time.Now().UTC(),
	}

	return &p, err
}

// TODO figure out why sometimes argv does not contain program name
func parseArgv(argv []string) (channelName string, err error) {
	if len(argv) == 1 {
		if argv[0] == appconfig.DefaultDocumentWorker || argv[0] == appconfig.DefaultSessionWorker {
			return "", errors.New("insufficient argument number")
		}
		return argv[0], nil
	} else if len(argv) == 2 {
		if argv[0] == appconfig.DefaultDocumentWorker || argv[0] == appconfig.DefaultSessionWorker {
			return argv[1], nil
		}
		return argv[0], nil
	} else if len(argv) == 3 {
		if argv[0] == appconfig.DefaultDocumentWorker || argv[0] == appconfig.DefaultSessionWorker {
			return argv[1], nil
		}
	}
	return "", fmt.Errorf("executable argument number mismatch: %v", len(argv))
}

func InitializeWorkerDependencies(log log.T, args []string) (*appconfig.SsmagentConfig, identity.IAgentIdentity, string, error) {
	config, err := getAppConfig(true)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to initialize config: %v", err)
	}
	channelName, err := parseArgv(args)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to parse args: %v", err)
	}
	var selector identity.IAgentIdentitySelector
	var agentIdentity identity.IAgentIdentity
	runtimeConfigClientHandler := runtimeConfigClientCreator()
	if ok, err := runtimeConfigClientHandler.ConfigExists(); ok && err == nil {
		log.Info("picking up runtime config identity selector")
		selector = runtimeConfigIdentitySelectorCreator(log)
	} else {
		log.Info("picking up default identity selector")
		selector = defaultAgentIdentitySelectorCreator(log)
	}
	agentIdentity, err = newAgentIdentity(log, &config, selector)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to get identity: %v", err)
	}

	return &config, agentIdentity, channelName, nil
}
