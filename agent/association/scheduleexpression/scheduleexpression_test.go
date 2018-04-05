// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package scheduleexpression

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
)

func TestParseReturnsSuccessfullyForValidRateExpression(t *testing.T) {
	// Assemble
	logger := log.DefaultLogger()

	// Act
	parsedExpression, err := CreateScheduleExpression(logger, "rate(30 minutes)")

	// Assert
	assert.NotNil(t, parsedExpression)
	assert.Nil(t, err)
}

func TestParseReturnsSuccessfullyForValidUpperCasedRateExpression(t *testing.T) {
	// Assemble
	logger := log.DefaultLogger()

	// Act
	parsedExpression, err := CreateScheduleExpression(logger, "RATE(30 MINUTES)")

	// Assert
	assert.NotNil(t, parsedExpression)
	assert.Nil(t, err)
}

func TestParseReturnsErrorForInvalidRateExpression(t *testing.T) {
	// Assemble
	logger := log.DefaultLogger()

	// Act
	parsedExpression, err := CreateScheduleExpression(logger, "rate(foo)")

	// Assert
	assert.Nil(t, parsedExpression)
	assert.NotNil(t, err)
}

func TestParseReturnsSuccessfullyForValidCronExpression(t *testing.T) {
	// Assemble
	logger := log.DefaultLogger()

	// Act
	parsedExpression, err := CreateScheduleExpression(logger, "cron(0 0 0/1 * * ? *)")

	// Assert
	assert.NotNil(t, parsedExpression)
	assert.Nil(t, err)
}

func TestParseReturnsSuccessfullyForValidUpperCasedCronExpression(t *testing.T) {
	// Assemble
	logger := log.DefaultLogger()

	// Act
	parsedExpression, err := CreateScheduleExpression(logger, "CRON(0 0 0/1 * * ? *)")

	// Assert
	assert.NotNil(t, parsedExpression)
	assert.Nil(t, err)
}

func TestParseReturnsErrorWhenScheduleExpressionIsJustTheConstantCron(t *testing.T) {
	// Assemble
	logger := log.DefaultLogger()

	// Act
	parsedExpression, err := CreateScheduleExpression(logger, "cron")

	// Assert
	assert.Nil(t, parsedExpression)
	assert.NotNil(t, err)
	assert.Equal(t, "Cron expression cron is invalid.", err.Error())
}

func TestParseReturnsErrorWhenCronExpressionHasExtraCharactersAppendedAtTheEnd(t *testing.T) {
	// Assemble
	logger := log.DefaultLogger()

	// Act
	parsedExpression, err := CreateScheduleExpression(logger, "cron(0 0 0/1 * * ? *)abc")

	// Assert
	assert.Nil(t, parsedExpression)
	assert.NotNil(t, err)
	assert.Equal(t, "Cron expression cron(0 0 0/1 * * ? *)abc is invalid.", err.Error())
}

func TestParseReturnsErrorWhenCronExpressionHasBracketsInsteadOfParentheses(t *testing.T) {
	// Assemble
	logger := log.DefaultLogger()

	// Act
	parsedExpression, err := CreateScheduleExpression(logger, "cron{0 0 0/1 * * ? *}")

	// Assert
	assert.Nil(t, parsedExpression)
	assert.NotNil(t, err)
	assert.Equal(t, "Cron expression cron{0 0 0/1 * * ? *} is invalid.", err.Error())
}

func TestParseReturnsErrorWhenScheduleExpressionIsOfUnknownType(t *testing.T) {
	// Assemble
	logger := log.DefaultLogger()

	// Act
	parsedExpression, err := CreateScheduleExpression(logger, "at(12:00)")

	// Assert
	assert.Nil(t, parsedExpression)
	assert.NotNil(t, err)
	assert.Equal(t, "Unknown expression type detected in expression at(12:00)", err.Error())
}
