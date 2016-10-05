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

package sdkutil

import (
	"errors"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
)

var (
	errSample = errors.New("some error")
)

func TestHandleAwsErrorCount(t *testing.T) {
	errorCount := 10
	stopPolicy := NewStopPolicy("test", errorCount)

	log := log.NewMockLog()

	for i := 0; i < errorCount-1; i++ {
		HandleAwsError(log, errSample, stopPolicy)
		if !stopPolicy.IsHealthy() {
			assert.Fail(t, "stoppolicy should be healthy")
		}
	}

	HandleAwsError(log, errSample, stopPolicy)
	if stopPolicy.IsHealthy() {
		assert.Fail(t, "stoppolicy should not be healthy")
	}

	HandleAwsError(log, nil, stopPolicy)
	if !stopPolicy.IsHealthy() {
		assert.Fail(t, "stoppolicy should have reset and be heallthy")
	}

}
