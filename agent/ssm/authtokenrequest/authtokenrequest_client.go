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

package authtokenrequest

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
)

type IClient interface {
	RequestManagedInstanceRoleToken(fingerprint string) (response *ssm.RequestManagedInstanceRoleTokenOutput, err error)
	UpdateManagedInstancePublicKey(publicKey, publicKeyType string) (response *ssm.UpdateManagedInstancePublicKeyOutput, err error)
}

// ISsmSdk defines the functions needed from the AWS SSM SDK
type ISsmSdk interface {
	RequestManagedInstanceRoleToken(input *ssm.RequestManagedInstanceRoleTokenInput) (*ssm.RequestManagedInstanceRoleTokenOutput, error)
	UpdateManagedInstancePublicKey(input *ssm.UpdateManagedInstancePublicKeyInput) (*ssm.UpdateManagedInstancePublicKeyOutput, error)
}

// Client is a service wrapper that delegates to the ssm sdk.
type Client struct {
	sdk ISsmSdk
}

// NewClient returns an initialized pointer to an IClient
func NewClient(sdk ISsmSdk) IClient {
	return &Client{
		sdk: sdk,
	}
}

// RequestManagedInstanceRoleToken calls the RequestManagedInstanceRoleToken SSM API.
func (svc *Client) RequestManagedInstanceRoleToken(fingerprint string) (response *ssm.RequestManagedInstanceRoleTokenOutput, err error) {
	params := &ssm.RequestManagedInstanceRoleTokenInput{
		Fingerprint: aws.String(fingerprint),
	}

	return svc.sdk.RequestManagedInstanceRoleToken(params)
}

// UpdateManagedInstancePublicKey calls the UpdateManagedInstancePublicKey SSM API.
func (svc *Client) UpdateManagedInstancePublicKey(publicKey, publicKeyType string) (response *ssm.UpdateManagedInstancePublicKeyOutput, err error) {

	params := ssm.UpdateManagedInstancePublicKeyInput{
		NewPublicKey:     aws.String(publicKey),
		NewPublicKeyType: aws.String(publicKeyType),
	}

	return svc.sdk.UpdateManagedInstancePublicKey(&params)
}
