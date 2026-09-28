// Copyright 2026 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package azure

import (
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/managedInstances/registration"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders"
)

const (
	IdentityType = "Azure"
	SourceType   = "Microsoft.Compute/virtualMachines"
)

// Identity is the struct defining the IAgentIdentityInner for Azure metadata service
type Identity struct {
	Log                    log.T
	Config                 *appconfig.SsmagentConfig
	registrationInfo       registration.IOnpremRegistrationInfo
	credentials            aws.Credentials
	credentialsProvider    credentialproviders.IRemoteProvider
	shareFile              string
	shouldShareCredentials bool
	credsInitMutex         sync.Mutex
}
