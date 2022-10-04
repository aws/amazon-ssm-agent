// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// bootstrap package contains logic for agent initialization
package bootstrap

import (
	"fmt"
	"runtime/debug"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/identity"
	"github.com/aws/amazon-ssm-agent/core/app/context"
	"github.com/aws/amazon-ssm-agent/core/workerprovider/longrunningprovider/datastore/filesystem"
)

const (
	bootstrapRetryInterval = 2
	defaultFileCreateMode  = 0750
)

var newAgentIdentitySelector = identity.NewDefaultAgentIdentitySelector
var newAgentIdentity = identity.NewAgentIdentity

// IBootstrap is the interface for initializing the system for core agent
type IBootstrap interface {
	Init() (context.ICoreAgentContext, error)
}

// Bootstrap is the implementation for initializing the system for core agent
type Bootstrap struct {
	log        logger.T
	fileSystem filesystem.IFileSystem
}

// NewBootstrap returns a new instance for bootstrap
func NewBootstrap(log log.T, fileSystem filesystem.IFileSystem) IBootstrap {
	return &Bootstrap{
		log:        log,
		fileSystem: fileSystem,
	}
}

// Init initialize the system for core agent
func (bs *Bootstrap) Init() (context.ICoreAgentContext, error) {
	logger := bs.log
	defer func() {
		if msg := recover(); msg != nil {
			logger.Errorf("bootstrap init run panic: %v", msg)
			logger.Errorf("%s: %s", msg, debug.Stack())
		}
	}()

	config, err := appconfig.Config(true)
	if err != nil {
		return nil, fmt.Errorf("app config could not be loaded - %v", err)
	}

	selector := newAgentIdentitySelector(logger)
	agentIdentity, err := newAgentIdentity(logger, &config, selector)
	if err != nil {
		return nil, logger.Errorf("failed to get identity: %v", err)
	}

	instanceId, err := agentIdentity.InstanceID()
	if err != nil {
		return nil, logger.Errorf("error fetching the instanceID, %v", err)
	}
	logger.Debugf("Using instanceID: '%s'", instanceId)

	err = bs.createIPCFolder()
	if err != nil {
		return nil, logger.Errorf("failed to create IPC folder, %v", err)
	}

	bs.updateSSMUserShellProperties(logger)
	for i := 0; i < 3; i++ {
		ctx, err := context.NewCoreAgentContext(logger, &config, agentIdentity)
		if err == nil {
			return ctx, nil
		}
		time.Sleep(bootstrapRetryInterval * time.Second)
	}

	return nil, logger.Errorf("context could not be loaded - %v", err)
}

func (bs *Bootstrap) createIfNotExist(dir string) (err error) {
	if _, err = bs.fileSystem.Stat(dir); bs.fileSystem.IsNotExist(err) {
		//configure it to be not accessible by others
		err = bs.fileSystem.MkdirAll(dir, defaultFileCreateMode)
	}
	return
}
