// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
//

package service

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/stretchr/testify/assert"
)

var testServiceOutput = "[{\"Name\": \"AJRouter\", \"DisplayName\": \"AllJoyn Router Service\", \"Status\": \"Stopped\", \"DependentServices\": \"\", \"ServicesDependedOn\": \"\", \"ServiceType\": \"Win32ShareProcess\", \"StartType\": \"\"},{\"Name\": \"ALG\", \"DisplayName\": \"Application Layer Gateway Service\", \"Status\": \"Stopped\", \"DependentServices\": \"\", \"ServicesDependedOn\": \"BrokerInfrastructure\", \"ServiceType\": \"Win32OwnProcess\", \"StartType\": \"\"}]"

var testServiceOutputData = []model.ServiceData{
	{
		Name:               "AJRouter",
		DisplayName:        "AllJoyn Router Service",
		Status:             "Stopped",
		DependentServices:  "",
		ServicesDependedOn: "",
		ServiceType:        "Win32ShareProcess",
		StartType:          "",
	},
	{
		Name:               "ALG",
		DisplayName:        "Application Layer Gateway Service",
		Status:             "Stopped",
		DependentServices:  "",
		ServicesDependedOn: "BrokerInfrastructure",
		ServiceType:        "Win32OwnProcess",
		StartType:          "",
	},
}

func testExecuteCommand(command string, args ...string) ([]byte, error) {
	return []byte(testServiceOutput), nil
}

func TestServiceData(t *testing.T) {

	contextMock := context.NewMockDefault()
	cmdExecutor = testExecuteCommand

	data, err := collectServiceData(contextMock, model.Config{})

	assert.Nil(t, err)
	assert.Equal(t, data, testServiceOutputData)
}
