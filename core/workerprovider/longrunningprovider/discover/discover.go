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

// Package discover finds worker configs for the core agent.
package discover

import (
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/core/workerprovider/longrunningprovider/model"
)

// IDiscover is the interface for starting/terminating/watching worker
type IDiscover interface {
	FindWorkerConfigs() map[string]*model.WorkerConfig
}

// WorkerDiscover contains list of running workers, it starts/terminates/watches workers
type WorkerDiscover struct {
	log log.T
}

// NewWorkerDiscover returns worker discover
func NewWorkerDiscover(log log.T) *WorkerDiscover {
	return &WorkerDiscover{
		log: log,
	}
}

// FindWorkerConfigs finds the available worker configs
func (discover *WorkerDiscover) FindWorkerConfigs() map[string]*model.WorkerConfig {
	ssmAgentWorker := model.WorkerConfig{
		Name:       model.SSMAgentWorkerName,
		BinaryName: model.SSMAgentWorkerBinaryName,
		Path:       appconfig.DefaultSSMAgentWorker,
		Args:       []string{},
	}

	availableWorkers := make(map[string]*model.WorkerConfig)
	availableWorkers[ssmAgentWorker.Name] = &ssmAgentWorker

	return availableWorkers
}
