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

// Package retryer overrides the default aws sdk retryer delay logic to better suit the mds needs
package retryer

import (
	"errors"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/smithy-go"
)

type SsmRetryer struct {
	*retry.Standard
}

// RetryDelay returns the delay duration before retrying this request again
func (s SsmRetryer) RetryDelay(attempt int, err error) (time.Duration, error) {
	// Handle GetMessages Client.Timeout error
	var oe *smithy.OperationError
	if errors.As(err, &oe) && oe.OperationName == "GetMessages" && strings.Contains(err.Error(), "Client.Timeout") {
		// expected error. we will retry with a short 100 ms delay
		return time.Duration(100 * time.Millisecond), nil
	}

	// retry after a > 1 sec timeout, increasing exponentially with each retry
	delay := int(math.Pow(2, float64(attempt))) * (rand.Intn(500) + 1000)
	// Cap at 20s to prevent excessive delays (v1's DefaultRetryer had a built-in cap)
	const maxDelayMs = 20000
	if delay > maxDelayMs {
		delay = maxDelayMs
	}
	return time.Duration(delay) * time.Millisecond, nil
}
