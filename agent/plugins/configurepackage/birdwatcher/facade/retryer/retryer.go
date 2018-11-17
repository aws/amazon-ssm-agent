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
	"math"
	"math/rand"
	"time"

	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/request"
)

type BirdwatcherRetryer struct {
	client.DefaultRetryer
}

var timeUnit = 1000

// RetryRules returns the delay duration before retrying this request again
func (s BirdwatcherRetryer) RetryRules(r *request.Request) time.Duration {
	// retry after a > 1 sec timeout, increasing exponentially with each retry
	// attempt 1: 1s - 21s
	// attempt 2: 4s - 1.4min
	// attempt 3: 9s - 5.48min
	rand.Seed(time.Now().UnixNano())
	throttleDelay := (int(math.Pow(4, float64(r.RetryCount)))*rand.Intn(5) + int(math.Pow(float64(r.RetryCount+1), 2))) * timeUnit //for seconds

	// Handle GetManifest, GetDocument and DescribeDocument Throttled calls error
	if (r.Operation.Name == "GetManifest" || r.Operation.Name == "GetDocument" || r.Operation.Name == "DescribeDocument") && r.IsErrorThrottle() {
		// throttling attempt. Increase the delay with greater exponential backoff
		return time.Duration(throttleDelay) * time.Millisecond
	}

	// if error is not of throttle type, add regular retry strategy
	// attempt 1: 1 - 5 sec
	// attempt 2: 1 - 9 sec
	// attempt 3: 1 - 17 sec
	delay := (int(math.Pow(2, float64(r.RetryCount)))*rand.Intn(2) + 1) * timeUnit
	return time.Duration(delay) * time.Millisecond
}
