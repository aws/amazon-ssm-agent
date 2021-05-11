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

// cloudwatchlogspublisher is responsible for pulling logs from the log queue and publishing them to cloudwatch

package cloudwatchlogspublisher

import (
	"errors"
	"testing"

	cloudwatchlogspublisher_mock "github.com/aws/amazon-ssm-agent/agent/agentlogstocloudwatch/cloudwatchlogspublisher/mock"
	"github.com/aws/amazon-ssm-agent/agent/agentlogstocloudwatch/cloudwatchlogsqueue"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/cihub/seelog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var serviceMock = cloudwatchlogspublisher_mock.NewServiceMockDefault(logMock)

func TestCreateLogGroupError(t *testing.T) {
	serviceMock := cloudwatchlogspublisher_mock.NewServiceMockDefault(logMock)
	serviceMock.On("IsLogGroupPresent", mock.AnythingOfType("string")).Return(false, &cloudwatchlogs.LogGroup{})
	serviceMock.On("CreateLogGroup", mock.AnythingOfType("string")).Return(errors.New("Log Group Creation Service Error"))

	cwPublisher := CloudWatchPublisher{
		context:               contextMock,
		cloudWatchLogsService: serviceMock,
	}

	err := cwPublisher.createLogGroupAndStream("GroupDoesNotExist", "Stream")
	assert.Error(t, err, "Error Expected When Log Group Creation Errors")

}

func TestCreateLogStreamError(t *testing.T) {
	serviceMock := cloudwatchlogspublisher_mock.NewServiceMockDefault(logMock)
	serviceMock.On("CreateLogGroup", mock.AnythingOfType("string")).Return(nil)
	serviceMock.On("CreateLogStream", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(errors.New("Log Stream Creation Service Error"))

	cwPublisher := CloudWatchPublisher{
		context:               contextMock,
		cloudWatchLogsService: serviceMock,
	}

	err := cwPublisher.createLogGroupAndStream("Group", "StreamDoesNotExist")
	assert.Error(t, err, "Error Expected When Log Stream Creation Errors")

}

func TestGetSharingConfigurations(t *testing.T) {

	xmlArgs := make(map[string]string)
	xmlArgs["log-group"] = "LogGroup"
	xmlArgs["log-sharing-enabled"] = "true"
	xmlArgs["sharing-destination"] = "KeyID::Key::Group::Stream"
	initArgs := seelog.CustomReceiverInitArgs{
		XmlCustomAttrs: xmlArgs,
	}
	cloudwatchlogsqueue.DestroyCloudWatchDataInstance()
	cloudwatchlogsqueue.CreateCloudWatchDataInstance(initArgs)
	sharingConfigs := getSharingConfigurations()
	assert.NotNil(t, sharingConfigs, "Parsing Valid Configurations should not result in nil")

	assert.Equal(t, "KeyID", sharingConfigs.accessKeyId, "Access Key Id incorrect")
	assert.Equal(t, "Key", sharingConfigs.secretAccessKey, "Secret Access Key incorrect")
	assert.Equal(t, "Group", sharingConfigs.logGroup, "Log Group incorrect")
	assert.Equal(t, "Stream", sharingConfigs.logStream, "Log Stream incorrect")

}

func TestGetSharingConfigurationsIncorrect(t *testing.T) {
	xmlArgs := make(map[string]string)
	xmlArgs["log-group"] = "LogGroup"
	xmlArgs["log-sharing-enabled"] = "true"
	xmlArgs["sharing-destination"] = "KeyID:Key::Group::Stream"
	initArgs := seelog.CustomReceiverInitArgs{
		XmlCustomAttrs: xmlArgs,
	}
	cloudwatchlogsqueue.DestroyCloudWatchDataInstance()
	cloudwatchlogsqueue.CreateCloudWatchDataInstance(initArgs)
	sharingConfigs := getSharingConfigurations()
	assert.Nil(t, sharingConfigs, "Configurations should be nil as incorrectly formatted")
}
