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

// Package metrics is responsible for pulling logs from the log queue and publishing them to cloudwatch

package metrics

import (
	"testing"

	contextmocks "github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/stretchr/testify/assert"
)

func TestCreateCloudwatchService(t *testing.T) {
	context := contextmocks.NewMockDefault()
	service := NewCloudWatchService(context)

	assert.NotNil(t, service)
	assert.NotNil(t, service.service)
	assert.Equal(t, "https://monitoring.us-east-1.amazonaws.com", service.service.Endpoint)
}
