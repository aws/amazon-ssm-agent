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

package service

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/stretchr/testify/assert"
)

var testService = []model.ServiceData{
	{
		Name:               "BrokerInfrastructure",
		DisplayName:        "Background Tasks Infrastructure Service",
		Status:             "Running",
		DependentServices:  "embeddedmode",
		ServicesDependedOn: "DcomLaunch RpcSs RpcEptMapper",
		ServiceType:        "Win32ShareProcess",
		StartType:          "",
	},
	{
		Name:               "embeddedmode",
		DisplayName:        "Embedded Mode",
		Status:             "Stopped",
		DependentServices:  "",
		ServicesDependedOn: "BrokerInfrastructure",
		ServiceType:        "Win32ShareProcess",
		StartType:          "",
	},
}

func testCollectServiceData(context context.T, config model.Config) (data []model.ServiceData, err error) {
	return testService, nil
}

func TestGatherer(t *testing.T) {
	contextMock := context.NewMockDefault()
	gatherer := Gatherer(contextMock)
	collectData = testCollectServiceData
	item, err := gatherer.Run(contextMock, model.Config{})
	assert.Nil(t, err)
	assert.Equal(t, 1, len(item))
	assert.Equal(t, GathererName, item[0].Name)
	assert.Equal(t, SchemaVersionOfServiceGatherer, item[0].SchemaVersion)
	assert.Equal(t, testService, item[0].Content)
}
