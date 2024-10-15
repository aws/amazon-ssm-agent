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

// package onprem contains functions that help procure the managed instance auth credentials
// dependencies
package onpremprovider

import (
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/managedInstances/registration"
	"github.com/aws/amazon-ssm-agent/agent/ssm/authtokenrequest"
	"github.com/aws/amazon-ssm-agent/common/identity/endpoint"

	"github.com/aws/aws-sdk-go/aws/credentials"

	"github.com/cenkalti/backoff/v4"
)

var backoffRetry = backoff.Retry

// onpremCredentialsProvider implements the AWS SDK credential provider, and is used to the create AWS client.
// It retrieves credentials from the SSM Auth service, and keeps track if those credentials are expired.
type onpremCredentialsProvider struct {
	credentials.Expiry

	// ExpiryWindow will allow the credentials to trigger refreshing prior to
	// the credentials actually expiring. This is beneficial so race conditions
	// with expiring credentials do not cause request to fail unexpectedly
	// due to ExpiredTokenException exceptions.
	//
	// So a ExpiryWindow of 10s would cause calls to IsExpired() to return true
	// 10 seconds before the credentials are actually expired.
	//
	// If ExpiryWindow is 0 or less it will be ignored.
	ExpiryWindow time.Duration

	// client is the required SSM managed instance service client to use when connecting to SSM Auth service.
	client authtokenrequest.IClient
	config *appconfig.SsmagentConfig
	log    log.T

	registrationInfo registration.IOnpremRegistrationInfo

	isSharingCreds        bool
	executableToRotateKey string
	shareFile             string

	endpointHelper endpoint.IEndpointHelper
}
