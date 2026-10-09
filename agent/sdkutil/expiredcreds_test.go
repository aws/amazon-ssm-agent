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

package sdkutil

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	ctxmocks "github.com/aws/amazon-ssm-agent/agent/mocks/context"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	identityMocks "github.com/aws/amazon-ssm-agent/common/identity/mocks"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/stretchr/testify/assert"
)

type countingProvider struct{ calls int }

func (p *countingProvider) Retrieve(context.Context) (aws.Credentials, error) {
	p.calls++
	return aws.Credentials{AccessKeyID: "AKID", SecretAccessKey: "SECRET", CanExpire: true, Expires: time.Now().Add(time.Hour)}, nil
}

type stubHTTPClient struct{ responses []string }

func (c *stubHTTPClient) Do(*http.Request) (*http.Response, error) {
	body := c.responses[0]
	c.responses = c.responses[1:]
	status := http.StatusOK
	if body != "{}" {
		status = http.StatusBadRequest
	}
	return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
}

type noDelayRetryer struct{ aws.Retryer }

func (noDelayRetryer) RetryDelay(int, error) (time.Duration, error) { return 0, nil }

func newSSMClient(provider aws.CredentialsProvider, httpClient *stubHTTPClient) *ssm.Client {
	agentIdentity := &identityMocks.IAgentIdentity{}
	agentIdentity.On("CredentialsProvider").Return(provider)
	ctx := &ctxmocks.Mock{}
	ctx.On("Log").Return(logmocks.NewMockLog())
	ctx.On("AppConfig").Return(appconfig.SsmagentConfig{})
	ctx.On("Identity").Return(agentIdentity)

	conf := AwsConfigForEndpoint(ctx, "https://ssm.us-east-1.amazonaws.com", "us-east-1")
	conf.HTTPClient = httpClient
	return ssm.NewFromConfig(conf, func(o *ssm.Options) { o.Retryer = noDelayRetryer{newRetryer()} })
}

func TestAwsConfig_ExpiredTokenInvalidatesCredentialsAndRetries(t *testing.T) {
	for _, code := range []string{"ExpiredToken", "ExpiredTokenException", "RequestExpired"} {
		t.Run(code, func(t *testing.T) {
			provider := &countingProvider{}
			httpClient := &stubHTTPClient{responses: []string{`{"__type":"` + code + `","message":"expired"}`, "{}"}}

			_, err := newSSMClient(aws.NewCredentialsCache(provider), httpClient).ListAssociations(context.TODO(), &ssm.ListAssociationsInput{})

			assert.NoError(t, err)
			assert.Equal(t, 2, provider.calls, "retry must sign with freshly retrieved credentials")
		})
	}
}

func TestAwsConfig_OtherErrorsKeepCachedCredentials(t *testing.T) {
	provider := &countingProvider{}
	httpClient := &stubHTTPClient{responses: []string{`{"__type":"AccessDeniedException","message":"denied"}`}}

	_, err := newSSMClient(aws.NewCredentialsCache(provider), httpClient).ListAssociations(context.TODO(), &ssm.ListAssociationsInput{})

	assert.Error(t, err)
	assert.Equal(t, 1, provider.calls)
}
