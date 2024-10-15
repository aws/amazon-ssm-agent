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
	"math"
	"math/rand"
	"strings"
	"time"
)

type Retryer interface {
	Call() error
	NextSleepTime(int32) time.Duration
}

// TODO Move to a common package for retry and merge with HibernateRetryStrategy
type ExponentialRetryer struct {
	CallableFunc   func() (interface{}, error)
	GeometricRatio float64
	// a random amount of jitter up to JitterRatio percentage, 0.0 means no jitter, 0.15 means 15% added to the total wait time.
	JitterRatio         float64
	InitialDelayInMilli int
	MaxDelayInMilli     int
	MaxAttempts         int
	NonRetryableErrors  []string
}

// Init initializes the retryer
func (retryer *ExponentialRetryer) Init() {
	rand.Seed(time.Now().UnixNano())
}

// NextSleepTime calculates the next delay of retry. Returns next sleep time as well as if it reaches max delay
func (retryer *ExponentialRetryer) NextSleepTime(attempt int) (time.Duration, bool) {
	sleep := time.Duration(float64(retryer.InitialDelayInMilli)*math.Pow(retryer.GeometricRatio, float64(attempt))) * time.Millisecond
	exceedMaxDelay := false
	if int(sleep/time.Millisecond) > retryer.MaxDelayInMilli {
		sleep = time.Duration(retryer.MaxDelayInMilli) * time.Millisecond
		exceedMaxDelay = true
	}
	jitter := int64(0)
	maxJitter := int64(float64(sleep) * retryer.JitterRatio)
	if maxJitter > 0 {
		jitter = rand.Int63n(maxJitter)
	}
	return sleep + time.Duration(jitter), exceedMaxDelay
}

// Call calls the operation and does exponential retry if error happens until it reaches MaxAttempts if specified.
func (retryer *ExponentialRetryer) Call() (channel interface{}, err error) {
	attempt := 0
	failedAttemptsSoFar := 0
	for {
		channel, err := retryer.CallableFunc()
		if err == nil || failedAttemptsSoFar == retryer.MaxAttempts || retryer.isNonRetryableError(err) {
			return channel, err
		}
		sleep, exceedMaxDelay := retryer.NextSleepTime(attempt)
		if !exceedMaxDelay {
			attempt++
		}
		time.Sleep(sleep)
		failedAttemptsSoFar++
	}
}

// isNonRetryableError returns true if passed error is in the list of NonRetryableErrors
func (retryer *ExponentialRetryer) isNonRetryableError(err error) bool {
	for _, nonRetryableError := range retryer.NonRetryableErrors {
		if strings.Contains(err.Error(), nonRetryableError) {
			return true
		}
	}
	return false
}
