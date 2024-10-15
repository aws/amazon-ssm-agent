// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package tests represents stress and integration tests of the agent
package tests

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/ec2/ec2detector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type ec2DetectorTestSuite struct {
	suite.Suite
}

func (suite *ec2DetectorTestSuite) SetupTest() {}

func (suite *ec2DetectorTestSuite) TearDownSuite() {}

// TestIsEC2Instance verifies that the test is running on ec2 instance (assumes that tests are running on ec2)
func (suite *ec2DetectorTestSuite) TestIsEC2Instance() {
	detector := ec2detector.New(appconfig.SsmagentConfig{})
	assert.True(suite.T(), detector.IsEC2Instance(), "Expected ec2detector to detect ec2 instance but failed to do so")
}

func TestEC2DetectorIntegTestSuite(t *testing.T) {
	suite.Run(t, new(ec2DetectorTestSuite))
}
