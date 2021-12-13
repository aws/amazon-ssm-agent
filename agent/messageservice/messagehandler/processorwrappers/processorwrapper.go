// Copyright 2021 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package processorwrappers implements different processor wrappers to handle the processors which launches
// document worker and session worker for now
package processorwrappers

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/utils"
)

var processorWrapperDelegateMap map[utils.WorkerName]func(context.T, *utils.ProcessorWorkerConfig) IProcessorWrapper

func init() {
	if processorWrapperDelegateMap == nil {
		processorWrapperDelegateMap = make(map[utils.WorkerName]func(context.T, *utils.ProcessorWorkerConfig) IProcessorWrapper)
	}
	processorWrapperDelegateMap[utils.DocumentWorkerName] = NewCommandWorkerProcessorWrapper
	processorWrapperDelegateMap[utils.SessionWorkerName] = NewSessionWorkerProcessorWrapper
}

// GetProcessorWrapperDelegateMap returns preloaded map with worker name and processor wrapper creation function pointer as its key and value
func GetProcessorWrapperDelegateMap() map[utils.WorkerName]func(context.T, *utils.ProcessorWorkerConfig) IProcessorWrapper {
	return processorWrapperDelegateMap
}
