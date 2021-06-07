// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package backoffconfig

import (
	"errors"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v4"
)

const (
	defaultMultiplier        = 2.0
	defaultMaxIntervalMillis = 30_000
	defaultJitterFactor      = 0.2
	defaultMaxDelayMillis    = 12 * 60 * 60 * 1000 // 12 hours
	defaultInitialInterval   = 100 * time.Millisecond
	defaultMaxRetries        = 5
)

// GetDefaultExponentialBackoff returns a new ExponentialBackoff configuration
func GetDefaultExponentialBackoff() (*backoff.ExponentialBackOff, error) {

	return GetExponentialBackoff(defaultInitialInterval, defaultMaxRetries)
}

// GetExponentialBackoff returns a new ExponentialBackoff configuration for the supplied initialInterval
// and maximum number of retries.
//
// ExponentialBackoff limits the maximum wait time, so this method computes the maximum amount of time
// to wait to ensure that all of the expected retries complete.
//
// initialInterval is the amount of time to wait after the first failure before retrying the operation
// maxRetries is the number of times backoff should retry in the event of a failure
func GetExponentialBackoff(initialInterval time.Duration, maxRetries int) (*backoff.ExponentialBackOff, error) {

	if initialInterval <= 0 {
		initialInterval = backoff.DefaultInitialInterval
	}

	maxRetries, err := bound(maxRetries, 1, 100)
	if err != nil {
		return nil, err
	}

	result := backoff.NewExponentialBackOff()
	result.InitialInterval = initialInterval
	result.MaxInterval = defaultMaxIntervalMillis * time.Millisecond
	result.Multiplier = defaultMultiplier
	result.RandomizationFactor = defaultJitterFactor
	result.MaxElapsedTime, err = getMaxElapsedTime(
		maxRetries,
		initialInterval,
		result.MaxInterval,
		defaultMaxDelayMillis*time.Millisecond,
		defaultMultiplier,
		defaultJitterFactor)

	if err != nil {
		return nil, err
	}

	result.Reset()
	return result, err
}

// bound returns a number that is constrained to be within a particular range (min, max).
// If number is within the indicated range, then the number is returned.  If number is less than
// min, then min is returned.  If number is greater than max, then max is returned.
func bound(number int, min int, max int) (int, error) {
	result := number

	if max < min {
		errorMessage := fmt.Sprintf("Invalid input. min (%d) is greater than max (%d)", min, max)
		return result, errors.New(errorMessage)
	}

	if result < min {
		result = min
	} else if max < result {
		result = max
	}

	return result, nil
}

// getMaxElapsedTime returns the maximum possible amount of time required for a given
// number of failure retries
//
// maxRetries is the maximum number of retries if every attempt fails
// initialInterval is the amount of time between the first attempt and the second
// maximumInterval is the maximum amount of time between attempts beyond which backoff time will not increase
// maximumElapsedTime is the maximum allowed elapsed time
// growthFactor is the ratio of one delay time to the previous delay time
// jitterFactor is the fraction of delay time that should be randomly varied
func getMaxElapsedTime(
	maxRetries int,
	initialInterval time.Duration,
	maximumInterval time.Duration,
	maximumElapsedTime time.Duration,
	growthFactor float64,
	jitterFactor float64) (time.Duration, error) {

	if maxRetries <= 0 || 100 < maxRetries {
		message := fmt.Sprintf("maxRetries (%d) is out of range (0, 100]", maxRetries)
		return maximumElapsedTime, errors.New(message)
	}

	intervalMillis := initialInterval.Milliseconds()

	if intervalMillis <= 0 || 10_000 < intervalMillis {
		message := fmt.Sprintf("initialInterval (%d ms) is out of range (0 ms, 10 s]", intervalMillis)
		return maximumElapsedTime, errors.New(message)
	}

	maximumIntervalMillis := maximumInterval.Milliseconds()
	if maximumInterval <= 0 {
		message := fmt.Sprintf("maximumInterval (%d ms) is non-positive", maximumIntervalMillis)
		return maximumElapsedTime, errors.New(message)
	}

	if growthFactor <= 1.0 || 10.0 < growthFactor {
		message := fmt.Sprintf("growthFactor (%f) is out of range (1.0, 10.0]", growthFactor)
		return maximumElapsedTime, errors.New(message)
	}

	if jitterFactor < 0.0 || 1.0 < jitterFactor {
		message := fmt.Sprintf("jitterFactor (%f) is out of range (0.0, 1.0)", jitterFactor)
		return maximumElapsedTime, errors.New(message)
	}

	maxElapsedMillis := intervalMillis

	for retry := 1; retry < maxRetries; retry++ {
		nextIntervalMillis := float64(intervalMillis) * growthFactor
		intervalMillis = min(int64(nextIntervalMillis), maximumIntervalMillis)
		maxElapsedMillis += intervalMillis
	}

	maxElapsedMillis = int64(float64(maxElapsedMillis) * (1.0 + jitterFactor))
	maxElapsedMillis = min(maxElapsedMillis, maximumElapsedTime.Milliseconds())
	result := time.Duration(maxElapsedMillis) * time.Millisecond
	return result, nil
}

// min returns the smaller of two int64 values
func min(a int64, b int64) int64 {
	result := a
	if b < a {
		result = b
	}

	return result
}
