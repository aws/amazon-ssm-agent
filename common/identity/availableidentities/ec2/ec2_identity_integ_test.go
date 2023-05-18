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

package ec2

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/common/runtimeconfig"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/stretchr/testify/assert"
)

// / TestEC2Identity_Register_CancelTest tests that aws calls are cancelled when the context is cancelled
func TestEC2Identity_Register_CancelTest(t *testing.T) {
	// Arrange
	awsConfig := &aws.Config{}
	awsConfig = awsConfig.WithMaxRetries(3).
		WithEndpoint("www.google.com:81").                      // Endpoint is unreachable which causes timeout
		WithEC2MetadataDisableTimeoutOverride(true).            // IMDS timeout is 1 second by default
		WithHTTPClient(&http.Client{Timeout: time.Second * 10}) // Decrease timeout from http default for test efficiency
	sess, _ := session.NewSession(awsConfig)

	region := "SomeRegion"
	getStoredPrivateKey = func(log log.T, manifestFileNamePrefix, vaultKey string) string {
		assert.Equal(t, IdentityType, manifestFileNamePrefix)
		return ""
	}

	getStoredPrivateKeyType = func(log log.T, manifestFileNamePrefix, vaultKey string) string {
		assert.Equal(t, IdentityType, manifestFileNamePrefix)
		return ""
	}

	updateServerInfo = func(instanceID, region, publicKey, privateKey, privateKeyType, manifestFileNamePrefix, vaultKey string) (err error) {
		assert.Equal(t, IdentityType, manifestFileNamePrefix)
		return nil
	}

	log := logmocks.NewMockLog()
	imdsClient := newImdsClient(sess)
	identity := &Identity{
		Log:                 log,
		Client:              imdsClient,
		authRegisterService: newAuthRegisterService(log, region, imdsClient),
		shareLock:           &sync.RWMutex{},
		runtimeConfigClient: runtimeconfig.NewIdentityRuntimeConfigClient(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	complete := make(chan struct{})

	go func() {
		err := identity.RegisterWithContext(ctx)
		assert.Error(t, err)
		complete <- struct{}{}
		close(complete)
	}()

	<-time.After(1 * time.Second)

	// Act
	cancel()

	// Assert
	select {
	case <-time.After(1 * time.Second):
		assert.Fail(t, "Test did not complete in allotted time")
	case <-complete:
	}
}
