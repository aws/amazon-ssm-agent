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
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type BackoffConfigTestSuite struct {
	suite.Suite
	mockLog log.T
}

func (suite *BackoffConfigTestSuite) SetupTest() {
	suite.mockLog = log.NewMockLog()
}

func TestArtifactTestSuite(t *testing.T) {
	suite.Run(t, new(BackoffConfigTestSuite))
}

func (suite *BackoffConfigTestSuite) TestBound_ReturnsNumberWhenNumberIsInRange() {
	number := 10
	min := 0
	max := 100

	result, err := bound(number, min, max)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), number, result)
}

func (suite *BackoffConfigTestSuite) TestBound_ReturnsMinWhenNumberLessThanMin() {

	number := 10
	min := 50
	max := 100

	result, err := bound(number, min, max)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), min, result)
}

func (suite *BackoffConfigTestSuite) TestBound_ReturnsMaxWhenNumberGreaterThanMax() {

	number := 10
	min := 1
	max := 5

	result, err := bound(number, min, max)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), max, result)
}

func (suite *BackoffConfigTestSuite) TestMin_ReturnsSmallerNumber() {

	smaller := int64(1)
	larger := int64(2)

	assert.Equal(suite.T(), smaller, min(smaller, larger))
	assert.Equal(suite.T(), smaller, min(larger, smaller))
	assert.Equal(suite.T(), smaller, min(smaller, smaller))
}

func (suite *BackoffConfigTestSuite) TestGetMaxElapsedTime_ReturnsMaximumPossibleElapsedTime() {

	expectedResult := 7 * time.Second
	initialInterval := 1 * time.Second
	maxInterval := 100 * time.Second
	maxDelay := 100 * time.Second
	growthFactor := 2.0
	jitterFactor := 0.0
	maxRetries := 3

	result, err := getMaxElapsedTime(
		maxRetries,
		initialInterval,
		maxInterval,
		maxDelay,
		growthFactor,
		jitterFactor)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), expectedResult, result)
}

func (suite *BackoffConfigTestSuite) TestGetMaxElapsedTime_IncludesJitter() {
	initialInterval := 1 * time.Second
	maxInterval := 100 * time.Second
	maxDelay := 100 * time.Second
	growthFactor := 2.0
	jitterFactor := 0.2
	maxRetries := 4

	expectedMillis := float64(1+2+4+8) * 1000.0 * (jitterFactor + 1)
	expectedResult := time.Duration(expectedMillis) * time.Millisecond

	result, err := getMaxElapsedTime(
		maxRetries,
		initialInterval,
		maxInterval,
		maxDelay,
		growthFactor,
		jitterFactor)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), expectedResult, result)
}

func (suite *BackoffConfigTestSuite) TestGetMaxElapsedTime_LimitsIntervals() {
	initialInterval := 1 * time.Second
	maxInterval := 2 * time.Second
	maxDelay := 100 * time.Second
	growthFactor := 2.0
	jitterFactor := 0.2
	maxRetries := 4

	expectedMillis := float64(1+2+2+2) * 1000.0 * (jitterFactor + 1.0)
	expectedResult := time.Duration(expectedMillis) * time.Millisecond

	result, err := getMaxElapsedTime(
		maxRetries,
		initialInterval,
		maxInterval,
		maxDelay,
		growthFactor,
		jitterFactor)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), expectedResult, result)
}

func (suite *BackoffConfigTestSuite) TestGetMaxElapsedTime_LimitsTotalElapsedTime() {
	initialInterval := 1 * time.Second
	maxInterval := 2 * time.Second
	maxDelay := 5 * time.Second
	growthFactor := 2.0
	jitterFactor := 0.2
	maxRetries := 4

	result, err := getMaxElapsedTime(
		maxRetries,
		initialInterval,
		maxInterval,
		maxDelay,
		growthFactor,
		jitterFactor)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), maxDelay, result)
}

func (suite *BackoffConfigTestSuite) TestGetExponentialBackoff_ReturnsExponentialBackoff() {

	initialInterval := 1 * time.Second
	maxRetries := 4

	expectedMaxMillis := int64(float64(1+2+4+8) * 1000.0 * (defaultJitterFactor + 1))

	result, err := GetExponentialBackoff(initialInterval, maxRetries)

	assert.Nil(suite.T(), err)

	assert.Equal(
		suite.T(),
		initialInterval.Milliseconds(),
		result.InitialInterval.Milliseconds(),
		"InitialInterval")

	assert.Equal(
		suite.T(),
		int64(defaultMaxIntervalMillis),
		result.MaxInterval.Milliseconds(),
		"MaxInterval")

	assert.Equal(
		suite.T(),
		defaultMultiplier,
		result.Multiplier,
		"Multiplier")

	assert.Equal(
		suite.T(),
		defaultJitterFactor,
		result.RandomizationFactor,
		"RandomizationFactor")

	assert.Equal(
		suite.T(),
		expectedMaxMillis,
		result.MaxElapsedTime.Milliseconds(),
		"MaxElapsedTime")
}

func (suite *BackoffConfigTestSuite) TestGetDefaultExponentialBackoff_ReturnsExponentialBackoff() {

	result, err := GetDefaultExponentialBackoff()

	assert.Nil(suite.T(), err)

	assert.Equal(
		suite.T(),
		defaultInitialInterval.Milliseconds(),
		result.InitialInterval.Milliseconds(),
		"InitialInterval")

	assert.Equal(
		suite.T(),
		int64(defaultMaxIntervalMillis),
		result.MaxInterval.Milliseconds(),
		"MaxInterval")

	assert.Equal(
		suite.T(),
		defaultMultiplier,
		result.Multiplier,
		"Multiplier")

	assert.Equal(
		suite.T(),
		defaultJitterFactor,
		result.RandomizationFactor,
		"RandomizationFactor")
}
