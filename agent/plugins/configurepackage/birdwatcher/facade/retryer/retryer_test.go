// Copyright 2026 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package retryer

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/smithy-go"
	"github.com/stretchr/testify/assert"
)

// resetTimeUnit forces the package-level timeUnit back to its production value
// for the duration of a unit test. The `fastinteg`-tagged integration tests in
// retryer_integ_test.go set timeUnit = 1 to speed themselves up but never
// restore it, so a unit test that runs after them in the same binary would
// otherwise see multiplier=1 and compute delays in milliseconds instead of
// seconds. Every unit test in this file that asserts on delay MAGNITUDE must
// call this helper via t.Cleanup-style defer to be isolated from that
// pollution.
func resetTimeUnit(t *testing.T) {
	t.Helper()
	prev := timeUnit
	timeUnit = 1000
	t.Cleanup(func() { timeUnit = prev })
}

func TestRetryDelay_ThrottleErrorCodes(t *testing.T) {
	resetTimeUnit(t)
	retryer := &BirdwatcherRetryer{
		Standard: retry.NewStandard(func(so *retry.StandardOptions) {
			so.MaxAttempts = 4
		}),
	}

	throttleCodes := []string{
		"Throttling",
		"ThrottlingException",
		"TooManyRequestsException",
		"ProvisionedThroughputExceededException",
		"RequestThrottled",
		"RequestLimitExceeded",
	}

	for _, code := range throttleCodes {
		t.Run(code, func(t *testing.T) {
			err := &smithy.OperationError{
				ServiceID:     "SSM",
				OperationName: "GetManifest",
				Err:           &smithy.GenericAPIError{Code: code, Message: "Rate exceeded"},
			}

			duration, retryErr := retryer.RetryDelay(1, err)
			assert.NoError(t, retryErr)
			// Throttle delay for attempt 1 should be significantly larger than regular delay
			// Formula: (4^1 * rand(0-4) + (1+1)^2) * 1000ms = (4*[0-4] + 4) * 1000
			// Range: 4000ms to 20000ms
			assert.True(t, duration >= 4*time.Millisecond,
				"Throttle delay for code %s should use aggressive backoff, got %v", code, duration)
		})
	}
}

func TestRetryDelay_ThrottleWithoutOperationContext_UsesThrottleDelay(t *testing.T) {
	resetTimeUnit(t)
	retryer := &BirdwatcherRetryer{
		Standard: retry.NewStandard(func(so *retry.StandardOptions) {
			so.MaxAttempts = 4
		}),
	}

	// In aws-sdk-go-v2 the retry middleware invokes RetryDelay with the raw
	// error from the send/deserialize chain — smithy.OperationError wrapping
	// is applied by the generated client after the retry middleware has
	// returned. RetryDelay therefore should not gate on operation name; the
	// BirdwatcherRetryer is only attached to the birdwatcher facade's own
	// aws.Config, so every retryable error it sees already belongs to one of
	// GetManifest / DescribeDocument / GetDocument. A bare APIError with a
	// throttle code — the shape actually observable at this layer — must be
	// treated as a throttle and get the wider backoff window.
	err := &smithy.GenericAPIError{Code: "Throttling", Message: "Rate exceeded"}

	duration, retryErr := retryer.RetryDelay(1, err)
	assert.NoError(t, retryErr)
	// Throttle formula for attempt 1: (4^1 * rand[0,5) + (1+1)^2) * 1000ms
	// → 4s .. 20s. Regular formula tops out at (2^1 * 1 + 1) * 1000 = 3s.
	assert.True(t, duration >= 4*time.Second,
		"Throttle error must use aggressive backoff regardless of operation-name context, got %v", duration)
}

func TestRetryDelay_NonThrottleError_UsesRegularDelay(t *testing.T) {
	resetTimeUnit(t)
	retryer := &BirdwatcherRetryer{
		Standard: retry.NewStandard(func(so *retry.StandardOptions) {
			so.MaxAttempts = 4
		}),
	}

	err := &smithy.OperationError{
		ServiceID:     "SSM",
		OperationName: "GetManifest",
		Err:           &smithy.GenericAPIError{Code: "InternalServerError", Message: "Something went wrong"},
	}

	duration, retryErr := retryer.RetryDelay(1, err)
	assert.NoError(t, retryErr)
	// Regular delay
	assert.True(t, duration <= 4*time.Second,
		"Non-throttle error should use regular delay, got %v", duration)
}
