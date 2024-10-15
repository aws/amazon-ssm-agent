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
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/utils"
)

// IProcessorWrapper provides basic methods for processor wrapper
type IProcessorWrapper interface {
	Initialize(outputChan map[contracts.UpstreamServiceName]chan contracts.DocumentResult) error
	GetName() utils.ProcessorName
	GetStartWorker() contracts.DocumentType
	GetTerminateWorker() contracts.DocumentType
	PushToProcessor(contracts.DocumentState) processor.ErrorCode
	Stop()
}
