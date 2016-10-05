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
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/request"
)

type SsmRetryer struct {
	client.DefaultRetryer
}

// RetryRules returns the delay duration before retrying this request again
func (s SsmRetryer) RetryRules(r *request.Request) time.Duration {
	// Handle GetMessages Client.Timeout error
	if r.Operation.Name == "GetMessages" && r.Error != nil && strings.Contains(r.Error.Error(), "Client.Timeout") {
		// expected error. we will retry with a short 100 ms delay
		return time.Duration(100 * time.Millisecond)
	}

	// retry after a > 1 sec timeout, increasing exponentially with each retry
	rand.Seed(time.Now().UnixNano())
	delay := int(math.Pow(2, float64(r.RetryCount))) * (rand.Intn(500) + 1000)
	return time.Duration(delay) * time.Millisecond
}
