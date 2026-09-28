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

// Package testutils represents the common logic needed for agent tests
package testutils

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/health"
	ssmMock "github.com/aws/amazon-ssm-agent/agent/ssm/mocks"
)

func NewHealthCheck(context context.T) (healthModule *health.HealthCheck, ssmServiceMock *ssmMock.Service) {
	serviceMock := new(ssmMock.Service)
	// Create a new Healthcheck module
	healthModule = health.NewHealthCheck(context, serviceMock)
	return healthModule, serviceMock
}
