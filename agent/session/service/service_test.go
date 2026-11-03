// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package service is a wrapper for the message gateway Service
package service

import (
	"crypto/tls"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/common/identity"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	contextmocks "github.com/aws/amazon-ssm-agent/agent/mocks/context"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	mgsConfig "github.com/aws/amazon-ssm-agent/agent/session/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

var (
	signer     = &v4.Signer{} // Credentials: credentials.NewStaticCredentials("AKID", "SECRET", "SESSION")
	tlsConfig  = &tls.Config{}
	region     = "us-east-1"
	instanceId = "i-12345678"
	sessionId  = "s-12345678"
	token      = "abcdefg"
	mgsHost    = "ssmmessages.us-east-1.amazonaws.com"
)

func TestGetRegion(t *testing.T) {
	service := getService()

	result := service.GetRegion()

	assert.Equal(t, region, result)
}

func TestGetV4Signer(t *testing.T) {
	service := getService()

	result := service.GetV4Signer()

	assert.Equal(t, result, signer)
}

func TestCreateControlChannel(t *testing.T) {
	service := getService()
	requestId := uuid.New().String()
	createControlChannelInput := &CreateControlChannelInput{
		MessageSchemaVersion: aws.String(mgsConfig.MessageSchemaVersion),
		RequestId:            aws.String(requestId),
	}
	mgsConfig.GetMgsEndpoint = func(context context.T, region string) string {
		return mgsHost
	}
	makeRestcall = func(log log.T, appConfig appconfig.SsmagentConfig, identity identity.IAgentIdentity, request []byte, methodType string, url string, region string, signer *v4.Signer) ([]byte, error) {
		output := &CreateControlChannelOutput{
			TokenValue:           aws.String(token),
			MessageSchemaVersion: aws.String(mgsConfig.MessageSchemaVersion),
		}
		return xml.Marshal(output)
	}
	output, err := service.CreateControlChannel(logmocks.NewMockLog(), createControlChannelInput, instanceId)

	assert.Nil(t, err)
	assert.Equal(t, token, *output.TokenValue)
}

func TestCreateDataChannel(t *testing.T) {
	service := getService()
	requestId := uuid.New().String()
	clientId := uuid.New().String()
	createDataChannelInput := &CreateDataChannelInput{
		MessageSchemaVersion: aws.String(mgsConfig.MessageSchemaVersion),
		RequestId:            aws.String(requestId),
		ClientId:             aws.String(clientId),
	}
	mgsConfig.GetMgsEndpoint = func(context context.T, region string) string {
		return mgsHost
	}
	makeRestcall = func(log log.T, appConfig appconfig.SsmagentConfig, identity identity.IAgentIdentity, request []byte, methodType string, url string, region string, signer *v4.Signer) ([]byte, error) {
		output := &CreateDataChannelOutput{
			TokenValue:           aws.String(token),
			MessageSchemaVersion: aws.String(mgsConfig.MessageSchemaVersion),
		}
		return xml.Marshal(output)
	}
	output, err := service.CreateDataChannel(logmocks.NewMockLog(), createDataChannelInput, sessionId)

	assert.Nil(t, err)
	assert.Equal(t, token, *output.TokenValue)
}

func TestGetBaseUrl(t *testing.T) {
	mgsConfig.GetMgsEndpoint = func(context context.T, region string) string {
		return mgsHost
	}

	// data channel url test - must carry the https scheme, otherwise the http
	// client rejects the request with "unsupported protocol scheme".
	dataChannelUrlResult, err := getMGSBaseUrl(contextmocks.NewMockDefault(), mgsConfig.DataChannel, sessionId, region)

	expectedDataChannelUrl := mgsConfig.HttpsPrefix + mgsHost + "/v1/data-channel/" + sessionId
	assert.Nil(t, err)
	assert.Equal(t, expectedDataChannelUrl, dataChannelUrlResult)

	// control channel url test - must carry the https scheme.
	controlChannelUrlResult, err := getMGSBaseUrl(contextmocks.NewMockDefault(), mgsConfig.ControlChannel, instanceId, region)

	expectedControlChannelUrl := mgsConfig.HttpsPrefix + mgsHost + "/v1/control-channel/" + instanceId
	assert.Nil(t, err)
	assert.Equal(t, expectedControlChannelUrl, controlChannelUrlResult)
}

func getService() Service {
	return &MessageGatewayService{
		region:  "us-east-1",
		signer:  signer,
		context: contextmocks.NewMockDefault(),
	}
}

func TestNewMgsHttpClient_DefaultDialTimeout(t *testing.T) {
	appConfig := appconfig.DefaultConfig()
	dialTimeout := time.Duration(appConfig.Mgs.ControlChannelDialTimeoutMillis) * time.Millisecond

	client := newMgsHttpClient(logmocks.NewMockLog(), appConfig)

	tr, ok := client.Transport.(*http.Transport)
	assert.True(t, ok)
	assert.NotNil(t, tr.DialContext)
	assert.NotNil(t, tr.TLSClientConfig)
	assert.NotNil(t, tr.Proxy)
	assert.Equal(t, mgsClientTimeout, tr.ResponseHeaderTimeout)
	// The client timeout must cover the dial, the TLS handshake and the response
	// headers, so that a slow first address does not consume the whole budget.
	assert.Equal(t, dialTimeout+tr.TLSHandshakeTimeout+mgsClientTimeout, client.Timeout)
	assert.Greater(t, client.Timeout, dialTimeout)
}

func TestNewMgsHttpClient_CustomDialTimeout(t *testing.T) {
	appConfig := appconfig.DefaultConfig()
	appConfig.Mgs.ControlChannelDialTimeoutMillis = 4000

	client := newMgsHttpClient(logmocks.NewMockLog(), appConfig)

	tr := client.Transport.(*http.Transport)
	assert.Equal(t, mgsClientTimeout, tr.ResponseHeaderTimeout)
	assert.Equal(t, 4*time.Second+tr.TLSHandshakeTimeout+mgsClientTimeout, client.Timeout)
}

func TestNewMgsHttpClient_DialTimeoutDisabled(t *testing.T) {
	appConfig := appconfig.DefaultConfig()
	appConfig.Mgs.ControlChannelDialTimeoutMillis = 0

	client := newMgsHttpClient(logmocks.NewMockLog(), appConfig)

	// A dial timeout of 0 keeps the pre-fix behavior: the default transport
	// with a 15s overall client timeout and no separate response header timeout.
	tr := client.Transport.(*http.Transport)
	defaultTr := http.DefaultTransport.(*http.Transport)
	assert.Equal(t, time.Duration(0), tr.ResponseHeaderTimeout)
	assert.Equal(t, mgsClientTimeout, client.Timeout)
	assert.Equal(t, defaultTr.TLSHandshakeTimeout, tr.TLSHandshakeTimeout)
}

func TestNewMgsHttpClient_RoundTrip(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(httpStatusCodeCreated)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	client := newMgsHttpClient(logmocks.NewMockLog(), appconfig.DefaultConfig())

	resp, err := client.Post(server.URL, "application/json", nil)
	assert.Nil(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	assert.Nil(t, err)
	assert.Equal(t, httpStatusCodeCreated, resp.StatusCode)
	assert.Equal(t, "ok", string(body))
}

func TestNewMgsHttpClient_ResponseHeaderTimeout(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
		w.WriteHeader(httpStatusCodeCreated)
	}))
	defer server.Close()
	defer close(release)

	client := newMgsHttpClient(logmocks.NewMockLog(), appconfig.DefaultConfig())
	tr := client.Transport.(*http.Transport)
	// Shorten the response header timeout so the test runs quickly.
	tr.ResponseHeaderTimeout = 100 * time.Millisecond

	start := time.Now()
	_, err := client.Post(server.URL, "application/json", nil)

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "timeout awaiting response headers")
	assert.Less(t, time.Since(start), client.Timeout)
}
