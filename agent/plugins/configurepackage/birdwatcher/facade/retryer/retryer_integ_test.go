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

// +build integration

// Package retryer overrides the default ssm retryer delay logic to suit GetManifest, DescribeDocument and GetDocument
package retryer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/stretchr/testify/assert"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/awstesting"
)

type testData struct {
	Data string
}

func body(str string) io.ReadCloser {
	return ioutil.NopCloser(bytes.NewReader([]byte(str)))
}

func unmarshal(req *request.Request) {
	defer req.HTTPResponse.Body.Close()
	if req.Data != nil {
		json.NewDecoder(req.HTTPResponse.Body).Decode(req.Data)
	}
	return
}

func unmarshalError(req *request.Request) {
	bodyBytes, err := ioutil.ReadAll(req.HTTPResponse.Body)
	if err != nil {
		req.Error = awserr.New("UnmarshalError", req.HTTPResponse.Status, err)
		return
	}
	if len(bodyBytes) == 0 {
		req.Error = awserr.NewRequestFailure(
			awserr.New("UnmarshalError", req.HTTPResponse.Status, fmt.Errorf("empty body")),
			req.HTTPResponse.StatusCode,
			"",
		)
		return
	}
	var jsonErr jsonErrorResponse
	if err := json.Unmarshal(bodyBytes, &jsonErr); err != nil {
		req.Error = awserr.New("UnmarshalError", "JSON unmarshal", err)
		return
	}
	req.Error = awserr.NewRequestFailure(
		awserr.New(jsonErr.Code, jsonErr.Message, nil),
		req.HTTPResponse.StatusCode,
		"",
	)
}

type jsonErrorResponse struct {
	Code    string `json:"__type"`
	Message string `json:"message"`
}

func TestRetryRulesThrottled1stAttempt(t *testing.T) {
	reqNum := 0
	reqs := []http.Response{
		{StatusCode: 400, Body: body(`{"__type":"Throttling","message":"Rate exceeded."}`)},
		{StatusCode: 429, Body: body(`{"__type":"ProvisionedThroughputExceededException","message":"Rate exceeded."}`)},
		{StatusCode: 200, Body: body(`{"data":"valid"}`)},
	}
	retryer := BirdwatcherRetryer{}
	timeUnit = 1
	retryer.NumMaxRetries = 1
	s := awstesting.NewClient(&aws.Config{Retryer: &retryer})

	s.Handlers.Validate.Clear()
	s.Handlers.Unmarshal.PushBack(unmarshal)
	s.Handlers.UnmarshalError.PushBack(unmarshalError)
	s.Handlers.Send.Clear() // mock sending
	s.Handlers.Send.PushBack(func(r *request.Request) {
		r.HTTPResponse = &reqs[reqNum]
		reqNum++
	})
	out := &testData{}
	r := s.NewRequest(&request.Operation{Name: "GetDocument"}, nil, out)
	err := r.Send()
	assert.Equal(t, 1, int(r.RetryCount))
	duration := retryer.RetryRules(r)
	assert.Error(t, err)
	durationVal := false
	if duration < 1*time.Millisecond || duration > 21*time.Millisecond {
		durationVal = true
	}
	assert.False(t, durationVal)
}

// This test can go on for 2 minutes
func TestRetryRulesThrottled2ndAttempt(t *testing.T) {
	reqNum := 0
	reqs := []http.Response{
		{StatusCode: 400, Body: body(`{"__type":"Throttling","message":"Rate exceeded."}`)},
		{StatusCode: 429, Body: body(`{"__type":"ProvisionedThroughputExceededException","message":"Rate exceeded."}`)},
		{StatusCode: 400, Body: body(`{"__type":"Throttling","message":"Rate exceeded."}`)},
	}
	retryer := BirdwatcherRetryer{}
	timeUnit = 1
	retryer.NumMaxRetries = 2
	s := awstesting.NewClient(&aws.Config{Retryer: &retryer})

	s.Handlers.Validate.Clear()
	s.Handlers.Unmarshal.PushBack(unmarshal)
	s.Handlers.UnmarshalError.PushBack(unmarshalError)
	s.Handlers.Send.Clear() // mock sending
	s.Handlers.Send.PushBack(func(r *request.Request) {
		r.HTTPResponse = &reqs[reqNum]
		reqNum++
	})
	out := &testData{}
	r := s.NewRequest(&request.Operation{Name: "GetManifest"}, nil, out)
	err := r.Send()
	duration := retryer.RetryRules(r)
	assert.Equal(t, 2, int(r.RetryCount))
	assert.Error(t, err)
	durationVal := false
	if duration < 4*time.Millisecond || duration > 84*time.Millisecond {
		durationVal = true
	}
	assert.False(t, durationVal)
}

func TestRetryRulesThrottled3rdAttempt(t *testing.T) {
	reqNum := 0
	reqs := []http.Response{
		{StatusCode: 400, Body: body(`{"__type":"Throttling","message":"Rate exceeded."}`)},
		{StatusCode: 429, Body: body(`{"__type":"ProvisionedThroughputExceededException","message":"Rate exceeded."}`)},
		{StatusCode: 400, Body: body(`{"__type":"Throttling","message":"Rate exceeded."}`)},
		{StatusCode: 400, Body: body(`{"__type":"Throttling","message":"Rate exceeded."}`)},
	}
	retryer := BirdwatcherRetryer{}
	timeUnit = 1
	retryer.NumMaxRetries = 3
	s := awstesting.NewClient(&aws.Config{Retryer: &retryer})

	s.Handlers.Validate.Clear()
	s.Handlers.Unmarshal.PushBack(unmarshal)
	s.Handlers.UnmarshalError.PushBack(unmarshalError)
	s.Handlers.Send.Clear() // mock sending
	s.Handlers.Send.PushBack(func(r *request.Request) {
		r.HTTPResponse = &reqs[reqNum]
		reqNum++
	})
	out := &testData{}
	r := s.NewRequest(&request.Operation{Name: "DescribeDocument"}, nil, out)
	err := r.Send()
	duration := retryer.RetryRules(r)
	assert.Equal(t, 3, int(r.RetryCount))
	assert.Error(t, err)
	durationVal := false
	if duration < 9*time.Millisecond || duration > 329*time.Millisecond {
		durationVal = true
	}
	assert.False(t, durationVal)
}

func TestRetryRulesNoThrottle1stAttempt(t *testing.T) {
	reqNum := 0
	reqs := []http.Response{
		{StatusCode: 500, Body: body(`{"__type":"UnknownError","message":"An error occurred."}`)},
		{StatusCode: 500, Body: body(`{"__type":"UnknownError","message":"An error occurred."}`)},
		{StatusCode: 500, Body: body(`{"__type":"UnknownError","message":"An error occurred."}`)},
		{StatusCode: 500, Body: body(`{"__type":"UnknownError","message":"An error occurred."}`)},
	}
	retryer := BirdwatcherRetryer{}
	timeUnit = 1
	retryer.NumMaxRetries = 1
	s := awstesting.NewClient(&aws.Config{Retryer: &retryer})

	s.Handlers.Validate.Clear()
	s.Handlers.Unmarshal.PushBack(unmarshal)
	s.Handlers.UnmarshalError.PushBack(unmarshalError)
	s.Handlers.Send.Clear() // mock sending
	s.Handlers.Send.PushBack(func(r *request.Request) {
		r.HTTPResponse = &reqs[reqNum]
		reqNum++
	})
	out := &testData{}
	//1st attempt
	r := s.NewRequest(&request.Operation{Name: "DescribeDocument"}, nil, out)
	err := r.Send()
	duration := retryer.RetryRules(r)
	assert.Equal(t, 1, int(r.RetryCount))
	assert.Error(t, err)
	durationVal := false
	if duration < 1*time.Millisecond || duration > 5*time.Millisecond {
		durationVal = true
	}
	assert.False(t, durationVal)
}

func TestRetryRulesNoThrottle2ndAttempt(t *testing.T) {
	reqNum := 0
	reqs := []http.Response{
		{StatusCode: 500, Body: body(`{"__type":"UnknownError","message":"An error occurred."}`)},
		{StatusCode: 500, Body: body(`{"__type":"UnknownError","message":"An error occurred."}`)},
		{StatusCode: 500, Body: body(`{"__type":"UnknownError","message":"An error occurred."}`)},
		{StatusCode: 500, Body: body(`{"__type":"UnknownError","message":"An error occurred."}`)},
	}
	retryer := BirdwatcherRetryer{}
	timeUnit = 1
	retryer.NumMaxRetries = 2
	s := awstesting.NewClient(&aws.Config{Retryer: &retryer})

	s.Handlers.Validate.Clear()
	s.Handlers.Unmarshal.PushBack(unmarshal)
	s.Handlers.UnmarshalError.PushBack(unmarshalError)
	s.Handlers.Send.Clear() // mock sending
	s.Handlers.Send.PushBack(func(r *request.Request) {
		r.HTTPResponse = &reqs[reqNum]
		reqNum++
	})
	out := &testData{}
	//1st attempt
	r := s.NewRequest(&request.Operation{Name: "DescribeDocument"}, nil, out)
	err := r.Send()
	duration := retryer.RetryRules(r)
	assert.Equal(t, 2, int(r.RetryCount))
	assert.Error(t, err)
	durationVal := false
	if duration < 1*time.Millisecond || duration > 9*time.Millisecond {
		durationVal = true
	}
	assert.False(t, durationVal)
}

func TestRetryRulesNoThrottle3rdAttempt(t *testing.T) {
	reqNum := 0
	reqs := []http.Response{
		{StatusCode: 500, Body: body(`{"__type":"UnknownError","message":"An error occurred."}`)},
		{StatusCode: 500, Body: body(`{"__type":"UnknownError","message":"An error occurred."}`)},
		{StatusCode: 500, Body: body(`{"__type":"UnknownError","message":"An error occurred."}`)},
		{StatusCode: 500, Body: body(`{"__type":"UnknownError","message":"An error occurred."}`)},
	}
	retryer := BirdwatcherRetryer{}
	timeUnit = 1
	retryer.NumMaxRetries = 3
	s := awstesting.NewClient(&aws.Config{Retryer: &retryer})

	s.Handlers.Validate.Clear()
	s.Handlers.Unmarshal.PushBack(unmarshal)
	s.Handlers.UnmarshalError.PushBack(unmarshalError)
	s.Handlers.Send.Clear() // mock sending
	s.Handlers.Send.PushBack(func(r *request.Request) {
		r.HTTPResponse = &reqs[reqNum]
		reqNum++
	})
	out := &testData{}
	r := s.NewRequest(&request.Operation{Name: "DescribeDocument"}, nil, out)
	err := r.Send()
	duration := retryer.RetryRules(r)
	assert.Equal(t, 3, int(r.RetryCount))
	assert.Error(t, err)
	durationVal := false
	if duration < 1*time.Millisecond || duration > 17*time.Millisecond {
		durationVal = true
	}
	assert.False(t, durationVal)
}
