// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

//go:build integration
// +build integration

package registrar

import (
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/ssm/authregister"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/ec2"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/ec2/mocks"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/mock"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/stretchr/testify/assert"
)

func TestRetryableRegistrar_RegisterWithRetry_WhenIMDSAvailable_AndSSMUnavailable_Cancelled(t *testing.T) {
	// Arrange
	log := log.NewMockLog()
	awsConfig := &aws.Config{}
	awsConfig = awsConfig.WithMaxRetries(3).
		WithEndpoint("www.google.com:81").                      // Endpoint is unreachable which causes timeout
		WithHTTPClient(&http.Client{Timeout: time.Second * 10}) // Decrease timeout from http default for test efficiency
	imdsClient := &mocks.IEC2MdsSdkClient{}
	imdsClient.On("GetMetadataWithContext", mock.Anything, mock.Anything).
		Return("SomeInstanceId", nil).
		Repeatability = 0
	// TODO: GetMetadata is still called by the IIRRoleProvider Retrieve() method instead of GetMetadataWithContext
	imdsClient.On("GetMetadata", mock.Anything).
		Return("SomeInstanceId", nil).
		Repeatability = 0
	imdsClient.On("RegionWithContext", mock.Anything).
		Return("SomeRegion", nil).
		Repeatability = 0

	config := appconfig.SsmagentConfig{
		Ssm: appconfig.SsmCfg{},
	}
	authRegisterService := authregister.NewClientWithConfig(log, config, imdsClient, *awsConfig)
	ec2Identity := &ec2.Identity{
		Log:                 log,
		Client:              imdsClient,
		Config:              &config,
		AuthRegisterService: authRegisterService,
	}

	isRegistrarRunning := atomic.Value{}
	isRegistrarRunning.Store(true)
	registrar := &RetryableRegistrar{
		log:                       log,
		identityRegistrar:         ec2Identity,
		registrationAttemptedChan: make(chan struct{}, 1),
		stopRegistrarChan:         make(chan struct{}, 1),
		timeAfterFunc:             time.After,
		isRegistrarRunning:        isRegistrarRunning,
	}

	// Act
	complete := make(chan struct{})
	go func() {
		registrar.RegisterWithRetry()
		complete <- struct{}{}
		close(complete)
	}()

	<-time.After(2 * time.Second)

	// Act
	registrar.Stop()

	// Assert
	select {
	case <-complete:
	case <-time.After(1 * time.Second):
		assert.Fail(t, "Test did not complete in allotted time")
	}

	// Assert
	assert.False(t, registrar.isRegistrarRunning.Load().(bool), "Registrar still running after Stop() called")
	select {
	case <-registrar.registrationAttemptedChan:
		// Registration attempted chan closed successfully
	default:
		assert.Fail(t, "registration not marked as attempted")
	}
}
