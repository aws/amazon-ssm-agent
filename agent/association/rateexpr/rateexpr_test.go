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

package rateexpr

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseReturnsRateExpressionSuccessfullyWhenItIsValidAndHasSpace(t *testing.T) {
	// Assemble
	currentTime := time.Now()
	expectedTime := getExpectedNextTime(currentTime, 30, "minutes")

	// Act
	rateExpression, err := Parse("rate(30 minutes)")

	// Assert
	assert.Equal(t, expectedTime, rateExpression.Next(currentTime))
	assert.Nil(t, err)
}

func TestParseReturnsRateExpressionSuccessfullyWhenItIsValidAndDoesNotHaveSpace(t *testing.T) {
	// Assemble
	currentTime := time.Now()
	expectedTime := getExpectedNextTime(currentTime, 45, "minutes")

	// Act
	rateExpression, err := Parse("rate(45minutes)")

	// Assert
	assert.Equal(t, expectedTime, rateExpression.Next(currentTime))
	assert.Nil(t, err)
}

func TestParseReturnsRateExpressionSuccessfullyWhenCaseInSensitivity(t *testing.T) {
	// Assemble
	currentTime := time.Now()
	expectedTime := getExpectedNextTime(currentTime, 25, "minutes")

	// Act
	rateExpression, err := Parse("rate(25 MINUTES)")

	// Assert
	assert.Equal(t, expectedTime, rateExpression.Next(currentTime))
	assert.Nil(t, err)
}

func TestParseReturnsRateExpressionSuccessfullyWhenUnitOfTimeIsMinute(t *testing.T) {
	// Assemble
	currentTime := time.Now()
	expectedTime := getExpectedNextTime(currentTime, 1, "minute")

	// Act
	rateExpression, err := Parse("rate(1 MINUTE)")

	// Assert
	assert.Equal(t, expectedTime, rateExpression.Next(currentTime))
	assert.Nil(t, err)
}

func TestParseReturnsRateExpressionSuccessfullyWhenValueHasPrecedingZeros(t *testing.T) {
	// Assemble
	currentTime := time.Now()
	expectedTime := getExpectedNextTime(currentTime, 10, "minutes")

	// Act
	rateExpression, err := Parse("rate(0010 minutes)")

	// Assert
	assert.Equal(t, expectedTime, rateExpression.Next(currentTime))
	assert.Nil(t, err)
}

func TestParseReturnsRateExpressionSuccessfullyWhenUnitOfTimeIsHour(t *testing.T) {
	// Assemble
	currentTime := time.Now()
	expectedTime := getExpectedNextTime(currentTime, 1, "hour")

	// Act
	rateExpression, err := Parse("rate(1 Hour)")

	// Assert
	assert.Equal(t, expectedTime, rateExpression.Next(currentTime))
	assert.Nil(t, err)
}

func TestParseReturnsRateExpressionSuccessfullyWhenUnitOfTimeIsHours(t *testing.T) {
	// Assemble
	currentTime := time.Now()
	expectedTime := getExpectedNextTime(currentTime, 5, "hours")

	// Act
	rateExpression, err := Parse("rate(5 Hours)")

	// Assert
	assert.Equal(t, expectedTime, rateExpression.Next(currentTime))
	assert.Nil(t, err)
}

func TestParseReturnsRateExpressionSuccessfullyWhenUnitOfTimeIsDay(t *testing.T) {
	// Assemble
	currentTime := time.Now()
	expectedTime := getExpectedNextTime(currentTime, 1, "day")

	// Act
	rateExpression, err := Parse("rate(1 Day)")

	// Assert
	assert.Equal(t, expectedTime, rateExpression.Next(currentTime))
	assert.Nil(t, err)
}

func TestParseReturnsRateExpressionSuccessfullyWhenUnitOfTimeIsDays(t *testing.T) {
	// Assemble
	currentTime := time.Now()
	expectedTime := getExpectedNextTime(currentTime, 3, "days")

	// Act
	rateExpression, err := Parse("rate(3 days)")

	// Assert
	assert.Equal(t, expectedTime, rateExpression.Next(currentTime))
	assert.Nil(t, err)
}

// This test is to prove that we do not have singular/plural restrictions in agent code.
func TestParseReturnsRateExpressionSuccessfullyWhenTimeUnitIsMinuteAndValueGreaterThanOne(t *testing.T) {
	// Assemble
	currentTime := time.Now()
	expectedTime := getExpectedNextTime(currentTime, 40, "minutes")

	// Act
	rateExpression, err := Parse("rate(40 minute)")

	// Assert
	assert.Equal(t, expectedTime, rateExpression.Next(currentTime))
	assert.Nil(t, err)
}

func TestParseShouldReturnErrorWhenRateExpressionHasInvalidValue(t *testing.T) {
	// Act
	rateExpression, err := Parse("rate(1Test days)")

	// Assert
	assert.Nil(t, rateExpression)
	assert.Equal(t, "Schedule expression is not a valid rate expression.", err.Error())
}

func TestParseShouldReturnErrorWhenRateExpressionHasHigherThanInt64MaxValue(t *testing.T) {
	// Act
	rateExpression, err := Parse("rate(10000000000000000000 days)")

	// Assert
	assert.Nil(t, rateExpression)
	assert.Equal(t, "Schedule expression is not a valid rate expression. Time value should be a positive number.", err.Error())
}

func TestParseShouldReturnErrorWhenRateExpressionHasMultipleRateExpressions(t *testing.T) {
	// Act
	rateExpression, err := Parse("rate(5 days)rate(5 days)")

	// Assert
	assert.Nil(t, rateExpression)
	assert.Equal(t, "Schedule expression is not a valid rate expression.", err.Error())
}

func TestParseShouldReturnErrorWhenRateExpressionHasUnsupportedTimeUnitOfSeconds(t *testing.T) {
	// Act
	rateExpression, err := Parse("rate(45 seconds)")

	// Assert
	assert.Nil(t, rateExpression)
	assert.Equal(t, "Schedule expression is not a valid rate expression.", err.Error())
}

func TestParseShouldReturnErrorWhenRateExpressionHasUnsupportedTimeUnitOfYears(t *testing.T) {
	// Act
	rateExpression, err := Parse("rate(45 years)")

	// Assert
	assert.Nil(t, rateExpression)
	assert.Equal(t, "Schedule expression is not a valid rate expression.", err.Error())
}

func TestParseShouldReturnErrorWhenRateExpressionHasNegativeDays(t *testing.T) {
	// Act
	rateExpression, err := Parse("rate(-2 days)")

	// Assert
	assert.Nil(t, rateExpression)
	assert.Equal(t, "Schedule expression is not a valid rate expression.", err.Error())
}

func TestParseShouldReturnErrorWhenRateExpressionHasZeroDays(t *testing.T) {
	// Act
	rateExpression, err := Parse("rate(0 days)")

	// Assert
	assert.Nil(t, rateExpression)
	assert.Equal(t, "Schedule expression is not a valid rate expression. Time value should be a positive number.", err.Error())
}

func TestParseShouldReturnErrorWhenRateExpressionHasFractionalHour(t *testing.T) {
	// Act
	rateExpression, err := Parse("rate(1.1 hours)")

	// Assert
	assert.Nil(t, rateExpression)
	assert.Equal(t, "Schedule expression is not a valid rate expression.", err.Error())
}

func TestParseShouldReturnErrorWhenRateExpressionHasExtraOpeningParenthesis(t *testing.T) {
	// Act
	rateExpression, err := Parse("rate((1 day)")

	// Assert
	assert.Nil(t, rateExpression)
	assert.Equal(t, "Schedule expression is not a valid rate expression.", err.Error())
}

func TestParseShouldReturnErrorWhenRateExpressionHasExtraParenthesis(t *testing.T) {
	// Act
	rateExpression, err := Parse("rate(1 day))")

	// Assert
	assert.Nil(t, rateExpression)
	assert.Equal(t, "Schedule expression is not a valid rate expression.", err.Error())
}

func TestParseShouldReturnErrorWhenRateExpressionHasIncorrectSpellingOfRateIdentifier(t *testing.T) {
	// Act
	rateExpression, err := Parse("rae(1 day)")

	// Assert
	assert.Nil(t, rateExpression)
	assert.Equal(t, "Schedule expression is not a valid rate expression.", err.Error())
}

func TestParseShouldReturnErrorWhenRateExpressionHasExtraParenthesisAtTheBeginning(t *testing.T) {
	// Act
	rateExpression, err := Parse("(rate(1 day)")

	// Assert
	assert.Nil(t, rateExpression)
	assert.Equal(t, "Schedule expression is not a valid rate expression.", err.Error())
}

func TestParseShouldReturnErrorWhenRateExpressionHasExtraAlphabetAtTheEnd(t *testing.T) {
	// Act
	rateExpression, err := Parse("rate(1 day)d")

	// Assert
	assert.Nil(t, rateExpression)
	assert.Equal(t, "Schedule expression is not a valid rate expression.", err.Error())
}

func TestParseShouldReturnErrorWhenRateExpressionHasExtraDigitAtTheEnd(t *testing.T) {
	// Act
	rateExpression, err := Parse("rate(1 day)5")

	// Assert
	assert.Nil(t, rateExpression)
	assert.Equal(t, "Schedule expression is not a valid rate expression.", err.Error())
}

func TestParseShouldReturnErrorWhenRateExpressionIsEmpty(t *testing.T) {
	// Act
	rateExpression, err := Parse("")

	// Assert
	assert.Nil(t, rateExpression)
	assert.Equal(t, "Schedule expression is not a valid rate expression.", err.Error())
}

func getExpectedNextTime(currentTime time.Time, value int, timeUnit string) time.Time {

	var duration time.Duration
	if timeUnit == "minute" || timeUnit == "minutes" {
		// duration takes nano second
		duration = time.Duration(value * 60 * 1000 * 1000 * 1000)
	}

	if timeUnit == "hour" || timeUnit == "hours" {
		duration = time.Duration(value * 60 * 60 * 1000 * 1000 * 1000)
	}

	if timeUnit == "day" || timeUnit == "days" {
		duration = time.Duration(value * 24 * 60 * 60 * 1000 * 1000 * 1000)
	}

	expectedTime := currentTime.Add(duration)
	return expectedTime
}
