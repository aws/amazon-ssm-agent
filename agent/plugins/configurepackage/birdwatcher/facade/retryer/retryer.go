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

// Package retryer overrides the default ssm retryer delay logic to suit GetManifest, DescribeDocument and GetDocument
package retryer

import (
	"errors"
	"math"
	"math/rand"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ssm/types"

	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/smithy-go"
)

type BirdwatcherRetryer struct {
	*retry.Standard
}

var timeUnit = 1000

// isThrottleError reports whether the error passed to RetryDelay represents a
// service throttling response. It matches on the shapes observable inside the
// retry Attempt middleware: the modelled *types.ThrottlingException that SSM
// emits for a subset of operations, and any smithy.APIError whose ErrorCode is
// one of the well-known throttle codes (Throttling / ThrottlingException /
// TooManyRequestsException / ProvisionedThroughputExceededException /
// RequestThrottled / RequestLimitExceeded).
func isThrottleError(err error) bool {
	var throttling *types.ThrottlingException
	if errors.As(err, &throttling) {
		return true
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "Throttling",
			"ThrottlingException",
			"TooManyRequestsException",
			"ProvisionedThroughputExceededException",
			"RequestThrottled",
			"RequestLimitExceeded":
			return true
		}
	}
	return false
}

func (s BirdwatcherRetryer) RetryDelay(attempt int, err error) (time.Duration, error) {
	rand.Seed(time.Now().UnixNano())

	// BirdwatcherRetryer is attached only to the aws.Config used by the
	// birdwatcher facade, which issues GetManifest / DescribeDocument /
	// GetDocument, so every retry decision here belongs to one of those
	// operations and no per-operation gating is needed.
	if isThrottleError(err) {
		// Throttled calls: widen the backoff window roughly as
		// 4^attempt × [0,5) seconds so we back off aggressively during
		// SSM/Distributor throttling events.
		throttleDelay := (int(math.Pow(4, float64(attempt)))*rand.Intn(5) + int(math.Pow(float64(attempt+1), 2))) * timeUnit
		return time.Duration(throttleDelay) * time.Millisecond, nil
	}

	// Regular retry strategy for transient/network errors.
	delay := (int(math.Pow(2, float64(attempt)))*rand.Intn(2) + 1) * timeUnit
	return time.Duration(delay) * time.Millisecond, nil
}
