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

package artifact

import (
	"path/filepath"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type ArtifactTestSuite struct {
	suite.Suite
	contextMock *context.Mock
	logMock     *log.Mock
	artifact    *Artifact
	appConfig   appconfig.SsmagentConfig
}

// SetupTest will initialized the object for each test case before test function execution
func (suite *ArtifactTestSuite) SetupTest() {
	suite.contextMock = context.NewMockDefault()
	suite.logMock = log.NewMockLog()
	suite.appConfig = appconfig.SsmagentConfig{}
	suite.artifact = NewSelfUpdateArtifact(suite.logMock, suite.appConfig)
}

func (suite *ArtifactTestSuite) TestMd5HashValue() {
	path := filepath.Join("testdata", "checksum_hash.txt")
	content, err := suite.artifact.md5HashValue(path)
	assert.Nil(suite.T(), err)
	assert.NotEqual(suite.T(), len(content), 0)
}

func (suite *ArtifactTestSuite) TestSha256HashValue() {
	path := filepath.Join("testdata", "checksum_hash.txt")
	content, err := suite.artifact.sha256HashValue(path)
	assert.Nil(suite.T(), err)
	assert.NotEqual(suite.T(), len(content), 0)
}

// Execute the test suite
func TestSelfUpdateTestSuite(t *testing.T) {
	suite.Run(t, new(ArtifactTestSuite))
}
