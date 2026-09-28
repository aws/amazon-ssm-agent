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

//go:build integration || fastinteg
// +build integration fastinteg

// Package retryer overrides the default ssm retryer delay logic to suit GetManifest, DescribeDocument and GetDocument
package retryer

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/aws/smithy-go"
)

type testData struct {
	Data string
}

type mockTransport struct {
	responses []http.Response
	reqNum    int
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if m.reqNum >= len(m.responses) {
		return nil, fmt.Errorf("no more mock responses")
	}
	resp := m.responses[m.reqNum]
	m.reqNum++

	// Clone the response to avoid modifying the original
	respCopy := resp
	respCopy.Request = req
	return &respCopy, nil
}

func body(str string) io.ReadCloser {
	return io.NopCloser(bytes.NewReader([]byte(str)))
}

func createMockClient(retryer aws.Retryer, responses []http.Response) *ssm.Client {
	cfg, _ := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
		config.WithRegion("us-east-1"),
		config.WithRetryer(func() aws.Retryer { return retryer }),
		config.WithHTTPClient(&http.Client{
			Transport: &mockTransport{responses: responses},
		}),
	)

	return ssm.NewFromConfig(cfg)
}

func TestRetryRulesThrottled1stAttempt(t *testing.T) {
	reqs := []http.Response{
		{StatusCode: 400, Body: body(`{"__type":"Throttling","message":"Rate exceeded."}`)},
		{StatusCode: 429, Body: body(`{"__type":"ProvisionedThroughputExceededException","message":"Rate exceeded."}`)},
		{StatusCode: 200, Body: body(`{"Manifest":"valid"}`)},
	}

	retryer := &BirdwatcherRetryer{
		Standard: retry.NewStandard(func(so *retry.StandardOptions) {
			so.MaxAttempts = 2 // 1 retry
		}),
	}
	defer func() { timeUnit = 1000 }()
	timeUnit = 1

	client := createMockClient(retryer, reqs)

	// Test GetDocument operation
	_, err := client.GetDocument(context.TODO(), &ssm.GetDocumentInput{
		Name: aws.String("test-doc"),
	})

	assert.Error(t, err)

	// Test retry delay calculation
	testErr := &smithy.GenericAPIError{Code: "Throttling", Message: "Rate exceeded"}
	duration, _ := retryer.RetryDelay(1, testErr)

	durationValid := duration >= 1*time.Millisecond && duration <= 21*time.Millisecond
	assert.True(t, durationValid, "Duration should be between 1ms and 21ms, got %v", duration)
}

func TestRetryRulesThrottled2ndAttempt(t *testing.T) {
	reqs := []http.Response{
		{StatusCode: 400, Body: body(`{"__type":"Throttling","message":"Rate exceeded."}`)},
		{StatusCode: 429, Body: body(`{"__type":"ProvisionedThroughputExceededException","message":"Rate exceeded."}`)},
		{StatusCode: 400, Body: body(`{"__type":"Throttling","message":"Rate exceeded."}`)},
	}

	retryer := &BirdwatcherRetryer{
		Standard: retry.NewStandard(func(so *retry.StandardOptions) {
			so.MaxAttempts = 3 // 2 retry
		}),
	}
	defer func() { timeUnit = 1000 }()
	timeUnit = 1

	client := createMockClient(retryer, reqs)

	// Test GetManifest operation
	_, err := client.GetManifest(context.TODO(), &ssm.GetManifestInput{
		PackageName:    aws.String("test-package"),
		PackageVersion: aws.String("1.0"),
	})

	assert.Error(t, err)

	// Test retry delay calculation for 2nd attempt
	testErr := &smithy.OperationError{
		OperationName: "GetManifest",
		Err: &types.ThrottlingException{
			Message: aws.String("Rate exceeded"),
		},
	}
	duration, _ := retryer.RetryDelay(2, testErr)

	durationValid := duration >= 4*time.Millisecond && duration <= 84*time.Millisecond
	assert.True(t, durationValid, "Duration should be between 4ms and 84ms, got %v", duration)
}

func TestRetryRulesThrottled3rdAttempt(t *testing.T) {
	reqs := []http.Response{
		{StatusCode: 400, Body: body(`{"__type":"Throttling","message":"Rate exceeded."}`)},
		{StatusCode: 429, Body: body(`{"__type":"ProvisionedThroughputExceededException","message":"Rate exceeded."}`)},
		{StatusCode: 400, Body: body(`{"__type":"Throttling","message":"Rate exceeded."}`)},
		{StatusCode: 400, Body: body(`{"__type":"Throttling","message":"Rate exceeded."}`)},
	}

	retryer := &BirdwatcherRetryer{
		Standard: retry.NewStandard(func(so *retry.StandardOptions) {
			so.MaxAttempts = 4 // 3 retry
		}),
	}
	defer func() { timeUnit = 1000 }()
	timeUnit = 1

	client := createMockClient(retryer, reqs)

	// Test DescribeDocument operation
	_, err := client.DescribeDocument(context.TODO(), &ssm.DescribeDocumentInput{
		Name: aws.String("test-doc"),
	})

	assert.Error(t, err)

	// Test retry delay calculation for 3rd attempt
	testErr := &smithy.OperationError{
		OperationName: "DescribeDocument",
		Err: &types.ThrottlingException{
			Message: aws.String("Rate exceeded"),
		},
	}
	duration, _ := retryer.RetryDelay(3, testErr)

	durationValid := duration >= 9*time.Millisecond && duration <= 329*time.Millisecond
	assert.True(t, durationValid, "Duration should be between 9ms and 329ms, got %v", duration)
}

func TestRetryRulesNoThrottle1stAttempt(t *testing.T) {
	reqs := []http.Response{
		{StatusCode: 500, Body: body(`{"__type":"UnknownError","message":"An error occurred."}`)},
		{StatusCode: 500, Body: body(`{"__type":"UnknownError","message":"An error occurred."}`)},
		{StatusCode: 500, Body: body(`{"__type":"UnknownError","message":"An error occurred."}`)},
		{StatusCode: 500, Body: body(`{"__type":"UnknownError","message":"An error occurred."}`)},
	}

	retryer := &BirdwatcherRetryer{
		Standard: retry.NewStandard(func(so *retry.StandardOptions) {
			so.MaxAttempts = 2 // 1 retry
		}),
	}
	defer func() { timeUnit = 1000 }()
	timeUnit = 1

	client := createMockClient(retryer, reqs)

	// Test DescribeDocument operation
	_, err := client.DescribeDocument(context.TODO(), &ssm.DescribeDocumentInput{
		Name: aws.String("test-doc"),
	})

	assert.Error(t, err)

	// Test retry delay calculation for non-throttle error
	testErr := &smithy.OperationError{
		OperationName: "DescribeDocument",
	}
	duration, _ := retryer.RetryDelay(1, testErr)

	durationValid := duration >= 1*time.Millisecond && duration <= 5*time.Millisecond
	assert.True(t, durationValid, "Duration should be between 1ms and 5ms, got %v", duration)
}

func TestRetryRulesNoThrottle2ndAttempt(t *testing.T) {
	reqs := []http.Response{
		{StatusCode: 500, Body: body(`{"__type":"UnknownError","message":"An error occurred."}`)},
		{StatusCode: 500, Body: body(`{"__type":"UnknownError","message":"An error occurred."}`)},
		{StatusCode: 500, Body: body(`{"__type":"UnknownError","message":"An error occurred."}`)},
		{StatusCode: 500, Body: body(`{"__type":"UnknownError","message":"An error occurred."}`)},
	}

	retryer := &BirdwatcherRetryer{
		Standard: retry.NewStandard(func(so *retry.StandardOptions) {
			so.MaxAttempts = 3 // 2 retry
		}),
	}
	defer func() { timeUnit = 1000 }()
	timeUnit = 1

	client := createMockClient(retryer, reqs)

	// Test DescribeDocument operation
	_, err := client.DescribeDocument(context.TODO(), &ssm.DescribeDocumentInput{
		Name: aws.String("test-doc"),
	})

	assert.Error(t, err)

	// Test retry delay calculation for 2nd attempt non-throttle
	testErr := &smithy.OperationError{
		OperationName: "DescribeDocument",
	}
	duration, _ := retryer.RetryDelay(2, testErr)

	durationValid := duration >= 1*time.Millisecond && duration <= 9*time.Millisecond
	assert.True(t, durationValid, "Duration should be between 1ms and 9ms, got %v", duration)
}

func TestRetryRulesNoThrottle3rdAttempt(t *testing.T) {
	reqs := []http.Response{
		{StatusCode: 500, Body: body(`{"__type":"UnknownError","message":"An error occurred."}`)},
		{StatusCode: 500, Body: body(`{"__type":"UnknownError","message":"An error occurred."}`)},
		{StatusCode: 500, Body: body(`{"__type":"UnknownError","message":"An error occurred."}`)},
		{StatusCode: 500, Body: body(`{"__type":"UnknownError","message":"An error occurred."}`)},
	}

	retryer := &BirdwatcherRetryer{
		Standard: retry.NewStandard(func(so *retry.StandardOptions) {
			so.MaxAttempts = 4 // 3 retry
		}),
	}
	defer func() { timeUnit = 1000 }()
	timeUnit = 1

	client := createMockClient(retryer, reqs)

	// Test DescribeDocument operation
	_, err := client.DescribeDocument(context.TODO(), &ssm.DescribeDocumentInput{
		Name: aws.String("test-doc"),
	})

	assert.Error(t, err)

	// Test retry delay calculation for 3rd attempt non-throttle
	testErr := &smithy.OperationError{
		OperationName: "DescribeDocument",
	}
	duration, _ := retryer.RetryDelay(3, testErr)

	durationValid := duration >= 1*time.Millisecond && duration <= 17*time.Millisecond
	assert.True(t, durationValid, "Duration should be between 1ms and 17ms, got %v", duration)
}
