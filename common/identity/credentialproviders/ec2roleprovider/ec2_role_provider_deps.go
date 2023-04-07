// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License").
// You may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ec2roleprovider

import (
	"time"

	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/ssmclient"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
)

const (
	agentName           = "amazon-ssm-agent"
	CredentialSourceSSM = "SSM"
	CredentialSourceEC2 = "EC2"
	IdentityTypeEC2     = "EC2"
)

var (
	iprEmptyCredential                                 = credentials.Value{ProviderName: ec2rolecreds.ProviderName}
	newV4ServiceWithCreds        ssmclient.Initializer = ssmclient.NewV4ServiceWithCreds
	timeNowFunc                                        = time.Now
	newCredentials                                     = credentials.NewCredentials
	exceptionsForDefaultHostMgmt                       = map[string]struct{}{
		"AccessDeniedException":        {},
		"EC2RoleRequestError":          {},
		"AssumeRoleUnauthorizedAccess": {},
	}
)

type IInnerProvider interface {
	credentials.Provider
	credentials.Expirer

	SetExpiration(expiration time.Time, window time.Duration)
}

type EC2InnerProviders struct {
	IPRProvider               IInnerProvider
	SsmEc2Provider            IInnerProvider
	SharedCredentialsProvider IInnerProvider
}

type IEC2RoleProvider interface {
	credentials.Expirer
	credentialproviders.IRemoteProvider
	GetInnerProvider() IInnerProvider
	Retrieve() (credentials.Value, error)
	ShareFile() string
	ShareProfile() string
	SharesCredentials() bool
}
