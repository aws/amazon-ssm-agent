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

// Package executer allows execute Pending association and InProgress association
package executer

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/stretchr/testify/assert"
)

func TestOutputBuilderWithMultiplePlugins(t *testing.T) {
	results := make(map[string]*contracts.PluginResult)

	results["pluginA"] = &contracts.PluginResult{
		Status: contracts.ResultStatusPassedAndReboot,
	}
	results["pluginB"] = &contracts.PluginResult{
		Status: contracts.ResultStatusSuccess,
	}
	results["pluginC"] = &contracts.PluginResult{
		Status: contracts.ResultStatusFailed,
	}

	output := buildOutput(results, 5)

	fmt.Println(output)
	assert.NotNil(t, output)
	assert.Equal(t, output, "3 out of 5 plugins processed, 2 success, 1 failed, 0 timedout")
}

func TestOutputBuilderWithSinglePlugin(t *testing.T) {
	results := make(map[string]*contracts.PluginResult)

	results["pluginA"] = &contracts.PluginResult{
		Status: contracts.ResultStatusFailed,
	}

	output := buildOutput(results, 1)

	fmt.Println(output)
	assert.NotNil(t, output)
	assert.Equal(t, output, "1 out of 1 plugin processed, 0 success, 1 failed, 0 timedout")
}
