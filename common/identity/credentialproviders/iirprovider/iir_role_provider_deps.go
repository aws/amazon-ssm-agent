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

package iirprovider

import (
	"time"
)

const (
	// ProviderName is the role provider name that is returned with credentials
	ProviderName       = "EC2IdentityRoleProvider"
	iirCredentialsPath = "identity-credentials/ec2/security-credentials/ec2-instance"
)

// Ec2RoleCreds defines the structure for EC2 credentials returned from IMDS
// Copied from github.com/aws/credentials/ec2rolecreds/ec2_role_provider.go
// A ec2RoleCredRespBody provides the shape for unmarshalling credential
// request responses.
type Ec2RoleCreds struct {
	// Success State
	Expiration      time.Time
	AccessKeyID     string
	SecretAccessKey string
	Token           string

	// Error state
	Code    string
	Message string
}

// IEC2MdsSdkClient defines the functions that the role provider depends on from the aws sdk
type IEC2MdsSdkClient interface {
	GetMetadata(string) (string, error)
	Region() (string, error)
}
