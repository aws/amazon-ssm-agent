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
	"context"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ssm"
)

type IClient interface {
	RequestManagedInstanceRoleToken(fingerprint string) (response *ssm.RequestManagedInstanceRoleTokenOutput, err error)
	RequestManagedInstanceRoleTokenWithContext(ctx context.Context, fingerprint string) (response *ssm.RequestManagedInstanceRoleTokenOutput, err error)
	UpdateManagedInstancePublicKey(publicKey, publicKeyType string) (response *ssm.UpdateManagedInstancePublicKeyOutput, err error)
	UpdateManagedInstancePublicKeyWithContext(ctx context.Context, publicKey, publicKeyType string) (response *ssm.UpdateManagedInstancePublicKeyOutput, err error)
}

// ISsmSdk defines the functions needed from the AWS SSM SDK
type ISsmSdk interface {
	RequestManagedInstanceRoleToken(input *ssm.RequestManagedInstanceRoleTokenInput) (*ssm.RequestManagedInstanceRoleTokenOutput, error)
	RequestManagedInstanceRoleTokenWithContext(ctx context.Context, input *ssm.RequestManagedInstanceRoleTokenInput, opts ...request.Option) (*ssm.RequestManagedInstanceRoleTokenOutput, error)
	UpdateManagedInstancePublicKey(input *ssm.UpdateManagedInstancePublicKeyInput) (*ssm.UpdateManagedInstancePublicKeyOutput, error)
	UpdateManagedInstancePublicKeyWithContext(ctx context.Context, input *ssm.UpdateManagedInstancePublicKeyInput, opts ...request.Option) (*ssm.UpdateManagedInstancePublicKeyOutput, error)
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
	return svc.RequestManagedInstanceRoleTokenWithContext(context.Background(), fingerprint)
}

// RequestManagedInstanceRoleTokenWithContext calls the RequestManagedInstanceRoleToken SSM API.
func (svc *Client) RequestManagedInstanceRoleTokenWithContext(ctx context.Context, fingerprint string) (response *ssm.RequestManagedInstanceRoleTokenOutput, err error) {
	params := &ssm.RequestManagedInstanceRoleTokenInput{
		Fingerprint: aws.String(fingerprint),
	}

	return svc.sdk.RequestManagedInstanceRoleTokenWithContext(ctx, params)
}

// UpdateManagedInstancePublicKey calls the UpdateManagedInstancePublicKey SSM API.
func (svc *Client) UpdateManagedInstancePublicKey(publicKey, publicKeyType string) (response *ssm.UpdateManagedInstancePublicKeyOutput, err error) {
	return svc.UpdateManagedInstancePublicKeyWithContext(context.Background(), publicKey, publicKeyType)
}

// UpdateManagedInstancePublicKeyWithContext calls the UpdateManagedInstancePublicKey SSM API.
func (svc *Client) UpdateManagedInstancePublicKeyWithContext(ctx context.Context, publicKey, publicKeyType string) (response *ssm.UpdateManagedInstancePublicKeyOutput, err error) {
	params := ssm.UpdateManagedInstancePublicKeyInput{
		NewPublicKey:     aws.String(publicKey),
		NewPublicKeyType: aws.String(publicKeyType),
	}

	return svc.sdk.UpdateManagedInstancePublicKeyWithContext(ctx, &params)
}
