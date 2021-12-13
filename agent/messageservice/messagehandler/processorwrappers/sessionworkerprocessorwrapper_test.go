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
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/twinj/uuid"
)

type SessionWorkerProcessorWrapperTestSuite struct {
	suite.Suite
}

//Execute the test suite
func TestSessionWorkerProcessorWrapperTestSuite(t *testing.T) {
	suite.Run(t, new(SessionWorkerProcessorWrapperTestSuite))
}

func (suite *SessionWorkerProcessorWrapperTestSuite) TestBuildAgentTaskCompleteWhenPluginIdIsEmpty() {
	pluginResults := make(map[string]*contracts.PluginResult)
	pluginResult := contracts.PluginResult{
		PluginName: "Standard_Stream",
		Status:     contracts.ResultStatusInProgress,
	}
	pluginResults["Standard_Stream"] = &pluginResult
	messageId := uuid.NewV4()
	result := contracts.DocumentResult{
		Status:          contracts.ResultStatusInProgress,
		PluginResults:   pluginResults,
		LastPlugin:      "",
		MessageID:       messageId.String(),
		AssociationID:   "",
		NPlugins:        1,
		DocumentName:    "documentName",
		DocumentVersion: "1",
	}

	ctx := context.NewMockDefault()
	workerConfigs := utils.LoadProcessorWorkerConfig(ctx)
	var processorWrap *SessionWorkerProcessorWrapper
	for _, config := range workerConfigs {
		if config.WorkerName == utils.SessionWorkerName {
			processorWrap = NewSessionWorkerProcessorWrapper(ctx, config).(*SessionWorkerProcessorWrapper)
			break
		}
	}
	resultChan := make(chan contracts.DocumentResult)
	documentResultChan := make(chan contracts.DocumentResult)
	outputMap := make(map[contracts.UpstreamServiceName]chan contracts.DocumentResult)
	outputMap[contracts.MessageGatewayService] = resultChan
	go processorWrap.listenSessionReply(documentResultChan, outputMap)
	documentResultChan <- result
	select {
	case <-resultChan:
		assert.Fail(suite.T(), "should not receive message")
	case <-time.After(100 * time.Millisecond):
		assert.True(suite.T(), true, "channel should not receive message")
	}
}
