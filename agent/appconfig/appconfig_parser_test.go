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

package appconfig

import (
	"os"
	"path/filepath"
	"testing"

	"io/ioutil"

	"github.com/stretchr/testify/assert"
)

// getStringValue Tests

type GetStringValueTest struct {
	Input        string
	DefaultValue string
	Output       string
}

var (
	getStringValueTests = []GetStringValueTest{
		{"", "test", "test"},
		{"val", "test", "val"},
	}
)

func TestGetStringValue(t *testing.T) {
	for _, test := range getStringValueTests {
		output := getStringValue(test.Input, test.DefaultValue)
		assert.Equal(t, test.Output, output)
	}
}

// getNumericValue Tests

type GetNumericValueTest struct {
	Input        int
	MinValue     int
	MaxValue     int
	DefaultValue int
	Output       int
}

var (
	getNumericValueTests = []GetNumericValueTest{
		{0, 10, 100, 50, 50},   // empty
		{1, 10, 100, 50, 50},   // less than min
		{200, 10, 100, 50, 50}, // greater than max
		{20, 10, 100, 50, 20},  // within range
	}
)

func TestGetNumericValue(t *testing.T) {
	for _, test := range getNumericValueTests {
		output := getNumericValue(test.Input, test.MinValue, test.MaxValue, test.DefaultValue)
		assert.Equal(t, test.Output, output)
	}
}

// Validate invalid values for json
func TestInvalidJsonVal(t *testing.T) {
	path, _ := os.Getwd()
	var sampleJsonPath = filepath.Join(path, "sample-app-config.json")

	originalFunc := retrieveAppConfigPath
	defer func() {
		os.Remove(sampleJsonPath)
		retrieveAppConfigPath = originalFunc
	}()

	tempJson := []byte(`{ "Ssm": { "SessionLogsRetentionDurationHours" : "hello", CustomInventoryDefaultLocation: 2 } }`)
	if err := ioutil.WriteFile(sampleJsonPath, tempJson, ReadWriteAccess); err != nil {
		return
	}

	retrieveAppConfigPath = func() (string, error) {
		return sampleJsonPath, nil
	}
	outputJsonConfig, err := Config(true)

	assert.Equal(t, outputJsonConfig.Ssm.CustomInventoryDefaultLocation, DefaultCustomInventoryFolder)
	assert.Equal(t, outputJsonConfig.Ssm.SessionLogsRetentionDurationHours, DefaultSessionLogsRetentionDurationHours)
	assert.NotNil(t, err)
}

// getNumeric64Value Tests

type GetNumeric64ValueTest struct {
	Input        int64
	MinValue     int64
	MaxValue     int64
	DefaultValue int64
	Output       int64
}

var (
	getNumeric64ValueTests = []GetNumeric64ValueTest{
		{0, 10, 100000000000000000, 50, 50},                                // empty
		{1, 10, 100000000000000000, 50, 50},                                // less than min
		{200000000000000000, 10, 100000000000000000, 50, 50},               // greater than max
		{30000000000000000, 10, 100000000000000000, 50, 30000000000000000}, // within range
	}
)

func TestGetNumeric64Value(t *testing.T) {
	for _, test := range getNumeric64ValueTests {
		output := getNumeric64Value(test.Input, test.MinValue, test.MaxValue, test.DefaultValue)
		assert.Equal(t, test.Output, output)
	}
}

func TestIdentityConsumptionOrder_InvalidConsumptionOrderValue(t *testing.T) {
	agentConfig := DefaultConfig()
	agentConfig.Identity.ConsumptionOrder = []string{"EC2", "InvalidValue"}
	parser(&agentConfig)

	assert.Contains(t, agentConfig.Identity.ConsumptionOrder, "EC2")
	assert.NotContains(t, agentConfig.Identity.ConsumptionOrder, "InvalidValue")
	assert.Equal(t, 1, len(agentConfig.Identity.ConsumptionOrder))
}

func TestIdentityConsumptionOrder_TwoInvalidConsumptionOrderValue(t *testing.T) {
	agentConfig := DefaultConfig()
	agentConfig.Identity.ConsumptionOrder = []string{"AnotherInvalidValue", "InvalidValue"}
	parser(&agentConfig)

	assert.NotContains(t, agentConfig.Identity.ConsumptionOrder, "AnotherInvalidValue")
	assert.NotContains(t, agentConfig.Identity.ConsumptionOrder, "InvalidValue")

	// Expect ConsumptionOrder to change to default consumption order
	for _, identityType := range DefaultIdentityConsumptionOrder {
		assert.Contains(t, agentConfig.Identity.ConsumptionOrder, identityType)
	}
	assert.Equal(t, len(DefaultIdentityConsumptionOrder), len(agentConfig.Identity.ConsumptionOrder))
}

func TestIdentityCredentialsValue_InvalidCredentialsProviderToDefault(t *testing.T) {
	agentConfig := DefaultConfig()

	agentConfig.Identity.CustomIdentities = append(agentConfig.Identity.CustomIdentities, &CustomIdentity{
		CredentialsProvider: "InvalidProvider",
	})
	parser(&agentConfig)
	assert.Equal(t, agentConfig.Identity.CustomIdentities[0].CredentialsProvider, DefaultCustomIdentityCredentialsProvider)

	agentConfig.Identity.CustomIdentities[0].CredentialsProvider = DefaultCustomIdentityCredentialsProvider
	parser(&agentConfig)
	assert.Equal(t, agentConfig.Identity.CustomIdentities[0].CredentialsProvider, DefaultCustomIdentityCredentialsProvider)
}
