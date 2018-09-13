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
	"time"
)

type Retryer interface {
	Call() error
	NextSleepTime(int32) time.Duration
}

// TODO Move to a common package for retry and merge with HibernateRetryStrategy
type ExponentialRetryer struct {
	CallableFunc        func() (interface{}, error)
	GeometricRatio      float64
	InitialDelayInMilli int
	MaxDelayInMilli     int
	MaxAttempts         int
}

// NextSleepTime calculates the next delay of retry.
func (retryer *ExponentialRetryer) NextSleepTime(attempt int) time.Duration {
	return time.Duration(float64(retryer.InitialDelayInMilli)*math.Pow(retryer.GeometricRatio, float64(attempt))) * time.Millisecond
}

// Call calls the operation and does exponential retry if error happens until it reaches MaxAttempts if specified.
func (retryer *ExponentialRetryer) Call() (channel interface{}, err error) {
	attempt := 0
	failedAttemptsSoFar := 0
	for {
		channel, err := retryer.CallableFunc()
		if err == nil || failedAttemptsSoFar == retryer.MaxAttempts {
			return channel, err
		}
		sleep := retryer.NextSleepTime(attempt)
		if int(sleep/time.Millisecond) > retryer.MaxDelayInMilli {
			sleep = time.Duration(retryer.MaxDelayInMilli) * time.Millisecond
		} else {
			attempt++
		}
		time.Sleep(sleep)
		failedAttemptsSoFar++
	}
}
