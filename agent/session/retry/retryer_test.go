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

// Package retry implements back off retry strategy for session manager channel connection.
package retry

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type RetryCounter struct {
	TotalAttempts int
}

var (
	retryGeometricRatio = 2.0
	jitterRatio         = 0.0
	initialDelayInMilli = 100
	maxDelayInMilli     = 1000
	maxAttempts         = 5
	totalAttempts       = 0
	callableFunc        = func() (interface{}, error) {
		totalAttempts = totalAttempts + 1
		return RetryCounter{TotalAttempts: totalAttempts}, errors.New("error occured in callable function")
	}
	nonRetryableError = "non retryable error"
	retryableError    = "retryable error"
)

func TestRepeatableExponentialRetryerRetriesForGivenNumberOfMaxAttempts(t *testing.T) {
	retryer := ExponentialRetryer{
		callableFunc,
		retryGeometricRatio,
		jitterRatio,
		initialDelayInMilli,
		maxDelayInMilli,
		maxAttempts,
		[]string{},
	}

	retryCounterInterface, err := retryer.Call()

	retryCounter := retryCounterInterface.(RetryCounter)
	assert.NotNil(t, err)
	assert.Equal(t, retryCounter.TotalAttempts, maxAttempts+1)
}

func TestExponentialRetryerWithJitter(t *testing.T) {
	jitterRatio = 0.1
	retryerWithJitter := ExponentialRetryer{
		callableFunc,
		retryGeometricRatio,
		jitterRatio,
		initialDelayInMilli,
		maxDelayInMilli,
		1,
		[]string{},
	}
	minDelay := int64(initialDelayInMilli) * time.Millisecond.Nanoseconds()
	maxDelay := int64(float64(minDelay) * (1.0 + jitterRatio))
	sleep, _ := retryerWithJitter.NextSleepTime(0)
	assert.True(t, sleep.Nanoseconds() >= minDelay && sleep.Nanoseconds() < maxDelay)
}

func TestRepeatableExponentialRetryerDoesNotRetryInCaseOfNoError(t *testing.T) {
	totalAttempts := 0
	callableFunc := func() (interface{}, error) {
		totalAttempts = totalAttempts + 1
		return RetryCounter{TotalAttempts: totalAttempts}, nil
	}

	retryer := ExponentialRetryer{
		callableFunc,
		retryGeometricRatio,
		jitterRatio,
		initialDelayInMilli,
		maxDelayInMilli,
		maxAttempts,
		[]string{nonRetryableError},
	}

	retryCounterInterface, err := retryer.Call()

	retryCounter := retryCounterInterface.(RetryCounter)
	assert.Nil(t, err)
	assert.Equal(t, retryCounter.TotalAttempts, 1)
}

func TestRepeatableExponentialRetryerDoesNotRetryInCaseOfNonRetryableError(t *testing.T) {
	totalAttempts := 0
	callableFunc := func() (interface{}, error) {
		totalAttempts = totalAttempts + 1
		return RetryCounter{TotalAttempts: totalAttempts}, errors.New(nonRetryableError)
	}

	retryer := ExponentialRetryer{
		callableFunc,
		retryGeometricRatio,
		jitterRatio,
		initialDelayInMilli,
		maxDelayInMilli,
		maxAttempts,
		[]string{nonRetryableError},
	}

	retryCounterInterface, err := retryer.Call()

	retryCounter := retryCounterInterface.(RetryCounter)
	assert.NotNil(t, err)
	assert.Equal(t, retryCounter.TotalAttempts, 1)
}

func TestRepeatableExponentialRetryerRetriesInCaseOfRetryableError(t *testing.T) {
	totalAttempts := 0
	callableFunc := func() (interface{}, error) {
		totalAttempts = totalAttempts + 1
		return RetryCounter{TotalAttempts: totalAttempts}, errors.New(retryableError)
	}

	retryer := ExponentialRetryer{
		callableFunc,
		retryGeometricRatio,
		jitterRatio,
		initialDelayInMilli,
		maxDelayInMilli,
		maxAttempts,
		[]string{nonRetryableError},
	}

	retryCounterInterface, err := retryer.Call()

	retryCounter := retryCounterInterface.(RetryCounter)
	assert.NotNil(t, err)
	assert.Equal(t, retryCounter.TotalAttempts, maxAttempts+1)
}
