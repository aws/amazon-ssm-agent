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

package context

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/stretchr/testify/assert"
)

func TestCreateContext(t *testing.T) {
	log := log.NewMockLog()
	instanceId := "i-1234567890"
	ssmAppconfig := &appconfig.SsmagentConfig{}

	err := platform.SetInstanceID(instanceId)
	assert.Nil(t, err)

	context, err := NewCoreAgentContext(log, ssmAppconfig)
	assert.Nil(t, err)
	assert.Equal(t, context.Log(), log)
	assert.Equal(t, context.AppConfig(), ssmAppconfig)
	assert.Equal(t, context.AppVariable().InstanceId, instanceId)
}

func TestWithContext(t *testing.T) {
	logger := log.NewMockLog()

	instanceId := "i-1234567890"
	ssmAppconfig := &appconfig.SsmagentConfig{}

	err := platform.SetInstanceID(instanceId)
	assert.Nil(t, err)

	context, err := NewCoreAgentContext(logger, ssmAppconfig)
	assert.Nil(t, err)
	assert.Equal(t, context.Log(), logger)
	assert.Equal(t, context.AppConfig(), ssmAppconfig)
	assert.Equal(t, context.AppVariable().InstanceId, instanceId)

	loggerNew := log.NewMockLog()
	logger.On("WithContext", []string{"test context"}).Return(loggerNew)

	context = context.With("test context")
	assert.Equal(t, context.Log(), loggerNew)
}
