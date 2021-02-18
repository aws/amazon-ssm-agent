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

// Parts of this file are automatically updated and should not be edited.

// Package rip contains AWS services regional endpoints.
package rip

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/stretchr/testify/assert"
)

func TestGetMgsEndpointForUnknownRegion(t *testing.T) {
	region := "unknown-region"
	expected := MgsServiceName + "." + region + ".amazonaws.com"

	contextMock := context.NewMockDefault()
	endpoint := GetDefaultServiceEndpoint(contextMock, region, MgsServiceName)

	assert.Equal(t, expected, endpoint)
}

func TestGetMgsEndpointForCnRegion(t *testing.T) {
	region := "cn-north-1"
	expected := MgsServiceName + "." + region + ".amazonaws.com.cn"

	contextMock := context.NewMockDefault()
	endpoint := GetDefaultServiceEndpoint(contextMock, region, MgsServiceName)

	assert.Equal(t, expected, endpoint)
}

func TestGetMgsEndpointForKnownAwsRegion(t *testing.T) {
	contextMock := &context.Mock{}
	contextMock.On("AppConfig").Return(appconfig.SsmagentConfig{})
	endpoint := GetMgsEndpoint(contextMock, "us-east-1")

	expected := MgsServiceName + ".us-east-1.amazonaws.com"

	assert.Equal(t, expected, endpoint)
}

func TestGetMgsEndpointForKnownAwsCnRegion(t *testing.T) {
	contextMock := &context.Mock{}
	contextMock.On("AppConfig").Return(appconfig.SsmagentConfig{})
	endpoint := GetMgsEndpoint(contextMock, "cn-northwest-1")

	expected := MgsServiceName + ".cn-northwest-1.amazonaws.com.cn"

	assert.Equal(t, expected, endpoint)
}
