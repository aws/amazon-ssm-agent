// Copyright 2025 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package health

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/common/identity"
	"github.com/stretchr/testify/assert"
)

func TestNewEC2Provider_ReturnsProvider(t *testing.T) {
	mockLog := logmocks.NewMockLog()

	// Test that NewEC2Provider function exists and can be called
	provider := NewEC2Provider(mockLog)

	// The provider might be nil if not in EC2 environment, but function should not panic
	// This tests that the function signature is correct
	_ = provider
}

func TestNewAzureProvider_ReturnsProvider(t *testing.T) {
	mockLog := logmocks.NewMockLog()
	config := &appconfig.SsmagentConfig{}

	// Test that newAzureProvider function exists and can be called
	provider := newAzureProvider(mockLog, config)

	// The provider might be nil if not in Azure environment, but function should not panic
	// This tests that the function signature is correct
	_ = provider
}

func TestProviderConstants(t *testing.T) {
	// Test that the new provider constants are defined correctly
	assert.Equal(t, appconfig.Provider("EC2"), appconfig.EC2)
	assert.Equal(t, appconfig.Provider("Azure"), appconfig.Azure)
	assert.Equal(t, appconfig.Provider("GCP"), appconfig.GCP)

	// Test that the Providers map contains the expected entries
	assert.Equal(t, appconfig.EC2, appconfig.Providers["EC2"])
	assert.Equal(t, appconfig.Azure, appconfig.Providers["Azure"])
	assert.Equal(t, appconfig.GCP, appconfig.Providers["GCP"])
}

func TestProviderFunctionSignatures(t *testing.T) {
	mockLog := logmocks.NewMockLog()
	config := &appconfig.SsmagentConfig{}

	// Test NewEC2Provider signature
	var ec2Provider identity.IProvider = NewEC2Provider(mockLog)
	_ = ec2Provider

	// Test newAzureProvider signature
	var azureProvider identity.IProvider = newAzureProvider(mockLog, config)
	_ = azureProvider

	// Test newGCPProvider signature
	var gcpProvider identity.IProvider = newGCPProvider(mockLog, config)
	_ = gcpProvider
}
