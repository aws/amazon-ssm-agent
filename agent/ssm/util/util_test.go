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

// Package util contains helper function common for ssm service
package util

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/agent/network"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/stretchr/testify/assert"
)

const schemelessVpceEndpoint = "vpce-0123456789abcdef0-abcdefgh.ssm.cn-north-1.vpce.amazonaws.com.cn"

// sendAndCaptureURL sends one SSM request with cfg and returns the URL it was sent to.
func sendAndCaptureURL(t *testing.T, cfg aws.Config) string {
	var capturedURL string
	cfg.Credentials = aws.AnonymousCredentials{}
	client := ssm.NewFromConfig(cfg, func(o *ssm.Options) {
		o.Retryer = aws.NopRetryer{}
		o.HTTPClient = &MockClient{DoFunc: func(req *http.Request) (*http.Response, error) {
			capturedURL = req.URL.String()
			return nil, errors.New("request captured")
		}}
	})
	_, err := client.DescribeInstanceInformation(context.Background(), &ssm.DescribeInstanceInformationInput{})
	assert.Error(t, err)
	return capturedURL
}

func TestAwsConfig_SchemelessSsmEndpointOverride_UsesHttps(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		expected string
	}{
		{"scheme-less", schemelessVpceEndpoint, "https://" + schemelessVpceEndpoint + "/"},
		{"https", "https://" + schemelessVpceEndpoint, "https://" + schemelessVpceEndpoint + "/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			appConfig := appconfig.DefaultConfig()
			appConfig.Ssm.Endpoint = tt.endpoint

			cfg := AwsConfig(logmocks.NewMockLog(), appConfig, "ssm", "cn-north-1")

			assert.Equal(t, tt.expected, sendAndCaptureURL(t, cfg))
		})
	}
}

func TestAwsConfigWithEndpoint_SchemelessEndpoint_UsesHttps(t *testing.T) {
	endpoint := schemelessVpceEndpoint

	cfg := AwsConfigWithEndpoint(logmocks.NewMockLog(), appconfig.DefaultConfig(), "ssm", "cn-north-1", &endpoint)

	assert.Equal(t, "https://"+schemelessVpceEndpoint+"/", sendAndCaptureURL(t, cfg))
	assert.Equal(t, schemelessVpceEndpoint, endpoint)
}

type MockClient struct {
	DoFunc func(req *http.Request) (*http.Response, error)
}

func (m *MockClient) Do(req *http.Request) (*http.Response, error) {
	if m.DoFunc != nil {
		return m.DoFunc(req)
	}
	// just in case you want default correct return value
	return &http.Response{}, nil
}
func TestHTTPSRedirect(t *testing.T) {
	origRequests := []*http.Request{
		{
			Method: "POST",
			URL: &url.URL{
				Scheme: "https",
			},
		},
	}
	redirectRequest := &http.Request{
		Method: "POST",
		URL: &url.URL{
			Scheme: "https",
		},
	}
	client := &MockClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			//This function mocks a redirect. Do gets called and then gets sent to the CheckRedirect handler.
			//via is requests already made and req is the upcoming request
			err := network.DisableHTTPDowngrade(req, origRequests)
			return &http.Response{
				StatusCode: http.StatusOK,
			}, err
		},
	}
	resp, err := client.Do(redirectRequest)
	assert.Nil(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestHTTPDowngrade(t *testing.T) {
	origRequests := []*http.Request{
		{
			Method: "POST",
			URL: &url.URL{
				Scheme: "https",
			},
		},
	}
	redirectRequest := &http.Request{
		Method: "POST",
		URL: &url.URL{
			Scheme: "http",
		},
	}
	client := &MockClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			//This function mocks a redirect. Do gets called and then gets sent to the CheckRedirect handler.
			//via is requests already made and req is the upcoming request
			err := network.DisableHTTPDowngrade(req, origRequests)
			return &http.Response{
				StatusCode: http.StatusPermanentRedirect,
			}, err
		},
	}
	resp, err := client.Do(redirectRequest)
	assert.NotNil(t, err)
	assert.Equal(t, http.StatusPermanentRedirect, resp.StatusCode)
}
