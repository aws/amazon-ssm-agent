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
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/aws-sdk-go/aws/credentials"
)

// IIRRoleProvider gets identity role credentials from instance metadata service
type IIRRoleProvider struct {
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
	Config       *appconfig.SsmagentConfig
	Log          log.T
	IMDSClient   IEC2MdsSdkClient
}

// Retrieve returns nil if it successfully retrieved the instance identity role credentials.
// Error is returned if the value were not obtainable, or empty.
func (p *IIRRoleProvider) Retrieve() (credentials.Value, error) {
	resp, err := p.IMDSClient.GetMetadata(iirCredentialsPath)
	if err != nil {
		p.Log.Errorf("failed to retrieve instance identity role. Error: %v", err)
		return EmptyCredentials(), err
	}

	respCreds := Ec2RoleCreds{}

	if err := json.NewDecoder(strings.NewReader(resp)).Decode(&respCreds); err != nil {
		p.Log.Errorf("failed to decode instance identity role credentials. Error: %v", err)
		return EmptyCredentials(), err
	}

	if respCreds.Code != "Success" {
		// If an error code was returned something failed requesting the role.
		return EmptyCredentials(), fmt.Errorf("instance metadata response is invalid")
	}

	// Set the expiration window to be half of the token's lifetime. This allows credential refreshes to survive transient
	// network issues more easily. Expiring at half the lifetime also follows the behavior of other protocols such as DHCP
	// https://tools.ietf.org/html/rfc2131#section-4.4.5. Note that not all of the behavior specified in that RFC is
	// implemented, just the suggestion to start renewals at 50% of token validity.
	p.ExpiryWindow = time.Until(respCreds.Expiration) / 2

	// Set the expiration of our credentials
	p.SetExpiration(respCreds.Expiration, p.ExpiryWindow)

	return credentials.Value{
		AccessKeyID:     respCreds.AccessKeyID,
		SecretAccessKey: respCreds.SecretAccessKey,
		SessionToken:    respCreds.Token,
		ProviderName:    ProviderName,
	}, nil

}

// EmptyCredentials returns empty instance identity role credentials
func EmptyCredentials() credentials.Value {
	return credentials.Value{ProviderName: ProviderName}
}
