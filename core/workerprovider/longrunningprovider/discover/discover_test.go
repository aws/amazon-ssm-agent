// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package discover implements session shell plugin with interactive commands.
package discover

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/core/workerprovider/longrunningprovider/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type DiscoveryTestSuite struct {
	suite.Suite
	mockLog  log.T
	discover *WorkerDiscover
}

func (suite *DiscoveryTestSuite) SetupTest() {
	mockLog := log.NewMockLog()
	suite.mockLog = mockLog
	suite.discover = &WorkerDiscover{}
}

// Execute the test suite
func TestDiscoveryTestSuite(t *testing.T) {
	suite.Run(t, new(DiscoveryTestSuite))
}

func (suite *DiscoveryTestSuite) TestInitWorkers_Successful() {

	results := suite.discover.FindWorkerConfigs()
	assert.NotNil(suite.T(), results)

	agentWorker, ok := results[model.SSMAgentWorkerName]

	assert.Equal(suite.T(), ok, true)
	assert.Equal(suite.T(), agentWorker.Name, model.SSMAgentWorkerName)
}
