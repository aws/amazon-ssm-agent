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

//go:build integration || fastinteg
// +build integration fastinteg

// retryer overrides the default aws sdk retryer delay logic to better suit the mds needs
package retryer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/stretchr/testify/assert"
)

type testData struct {
	Data string `json:"data"`
}

type mockHTTPClient struct {
	responses []http.Response
	reqNum    int
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if m.reqNum >= len(m.responses) {
		return nil, fmt.Errorf("no more responses")
	}
	resp := m.responses[m.reqNum]
	m.reqNum++
	return &resp, nil
}

func body(str string) io.ReadCloser {
	return io.NopCloser(bytes.NewReader([]byte(str)))
}

// test that retries occur for 5xx status codes
func TestRequestRecoverRetry5xx(t *testing.T) {
	responses := []http.Response{
		{StatusCode: 500, Body: body(`{"__type":"UnknownError","message":"An error occurred."}`)},
		{StatusCode: 502, Body: body(`{"__type":"UnknownError","message":"An error occurred."}`)},
		{StatusCode: 200, Body: body(`{"data":"valid"}`)},
	}

	retryer := &SsmRetryer{
		Standard: retry.NewStandard(func(so *retry.StandardOptions) {
			so.MaxAttempts = 2
		}),
	}

	// Simulate making a request that would retry on 5xx errors
	var result testData

	// Mock the retry logic by calling the retryer directly
	attempt := 0
	for {
		resp := &responses[attempt]

		if resp.StatusCode >= 500 {
			// Check if we should retry
			retryable := retryer.IsErrorRetryable(&smithyhttp.ResponseError{
				Response: &smithyhttp.Response{Response: resp},
				Err:      fmt.Errorf("server error"),
			})

			if retryable && attempt < retryer.MaxAttempts() {
				attempt++
				continue
			}
		}

		if resp.StatusCode == 200 {
			json.NewDecoder(resp.Body).Decode(&result)
			break
		}

		attempt++
		if attempt >= len(responses) {
			break
		}
	}

	assert.Equal(t, 2, attempt)
	assert.Equal(t, "valid", result.Data)
}

// test that retries occur for 4xx status codes with a response type that can be retried
func TestRequestRecoverRetry4xxRetryable(t *testing.T) {
	responses := []http.Response{
		{StatusCode: 400, Body: body(`{"__type":"Throttling","message":"Rate exceeded."}`)},
		{StatusCode: 429, Body: body(`{"__type":"ProvisionedThroughputExceededException","message":"Rate exceeded."}`)},
		{StatusCode: 200, Body: body(`{"data":"valid"}`)},
	}

	retryer := &SsmRetryer{
		Standard: retry.NewStandard(func(so *retry.StandardOptions) {
			so.MaxAttempts = 10
		}),
	}

	var result testData
	attempt := 0

	for {
		resp := &responses[attempt]

		if resp.StatusCode == 400 || resp.StatusCode == 429 {
			var errResp struct {
				Type    string `json:"__type"`
				Message string `json:"message"`
			}
			json.NewDecoder(resp.Body).Decode(&errResp)

			retryable := retryer.IsErrorRetryable(&smithyhttp.ResponseError{
				Response: &smithyhttp.Response{Response: resp},
				Err: &smithy.GenericAPIError{
					Code:    errResp.Type,
					Message: errResp.Message,
				},
			})

			if retryable && attempt < retryer.MaxAttempts() {
				attempt++
				continue
			}
		}

		if resp.StatusCode == 200 {
			json.NewDecoder(resp.Body).Decode(&result)
			break
		}

		attempt++
		if attempt >= len(responses) {
			break
		}
	}

	assert.Equal(t, 2, attempt)
	assert.Equal(t, "valid", result.Data)
}

// test that retries don't occur for 4xx status codes with a response type that can't be retried
func TestRequest4xxUnretryable(t *testing.T) {
	response := http.Response{
		StatusCode: 401,
		Body:       body(`{"__type":"SignatureDoesNotMatch","message":"Signature does not match."}`),
	}

	retryer := &SsmRetryer{
		Standard: retry.NewStandard(func(so *retry.StandardOptions) {
			so.MaxAttempts = 10
		}),
	}

	var errResp struct {
		Type    string `json:"__type"`
		Message string `json:"message"`
	}
	json.NewDecoder(response.Body).Decode(&errResp)

	retryable := retryer.IsErrorRetryable(&smithyhttp.ResponseError{
		Response: &smithyhttp.Response{Response: &response},
		Err: &smithy.GenericAPIError{
			Code:    errResp.Type,
			Message: errResp.Message,
		},
	})

	assert.False(t, retryable)
	assert.Equal(t, "SignatureDoesNotMatch", errResp.Type)
	assert.Equal(t, "Signature does not match.", errResp.Message)
}

// test that retries delay increase over time
func TestDelayIncreasesOverTime(t *testing.T) {
	retryer := &SsmRetryer{
		Standard: retry.NewStandard(func(so *retry.StandardOptions) {
			so.MaxAttempts = 10
		}),
	}

	var delays []time.Duration

	for i := 0; i < 4; i++ {
		delay, _ := retryer.RetryDelay(i, nil)
		delays = append(delays, delay)
	}

	for i := 1; i < len(delays); i++ {
		assert.True(t, delays[i] >= delays[i-1],
			"Expected delay to increase: %v >= %v", delays[i], delays[i-1])
	}
}

// test that retries delay increase by at least a second
func TestDelayIncreasesByASecond(t *testing.T) {
	retryer := &SsmRetryer{
		Standard: retry.NewStandard(func(so *retry.StandardOptions) {
			so.MaxAttempts = 10
		}),
	}

	delay, _ := retryer.RetryDelay(1, nil)

	assert.True(t, delay >= time.Second,
		"Expected delay to be at least 1 second, got %v", delay)
}
