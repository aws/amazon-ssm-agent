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

package model

import (
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/association/scheduleexpression"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/stretchr/testify/assert"
)

func TestRateExpressionIsParsedSuccessfullyWhenItIsValid(t *testing.T) {

	// Assemble
	logger := log.DefaultLogger()

	assocRawData := InstanceAssociation{
		CreateDate: time.Now(),
	}

	assocRawData.Association = &ssm.InstanceAssociationSummary{}
	testRateExpression := "rate(5 days)"
	assocRawData.Association.ScheduleExpression = &testRateExpression

	// Act
	err := assocRawData.ParseExpression(logger)

	// Assert
	assert.Nil(t, err)
	assert.NotNil(t, assocRawData.ParsedExpression)
}

func TestUpperCasedRateExpressionIsParsedSuccessfullyWhenItIsValid(t *testing.T) {

	// Assemble
	logger := log.DefaultLogger()

	assocRawData := InstanceAssociation{
		CreateDate: time.Now(),
	}

	assocRawData.Association = &ssm.InstanceAssociationSummary{}
	testRateExpression := "RATE(5 DAYS)"
	assocRawData.Association.ScheduleExpression = &testRateExpression

	// Act
	err := assocRawData.ParseExpression(logger)

	// Assert
	assert.Nil(t, err)
	assert.NotNil(t, assocRawData.ParsedExpression)
}

func TestCronExpressionIsParsedSuccessfullyWhenItIsValid(t *testing.T) {

	// Assemble
	logger := log.DefaultLogger()

	assocRawData := InstanceAssociation{
		CreateDate: time.Now(),
	}

	assocRawData.Association = &ssm.InstanceAssociationSummary{}
	testExpression := "0 0 0/1 * * ? *"
	testCronExpression := "cron(" + testExpression + ")"
	assocRawData.Association.ScheduleExpression = &testCronExpression

	// Act
	err := assocRawData.ParseExpression(logger)

	// Assert
	assert.Equal(t, nil, err)
	assert.NotNil(t, assocRawData.ParsedExpression)
}

func TestParseExpressionReturnsErrorWhenCronExpressionIsInvalid(t *testing.T) {

	// Assemble
	logger := log.DefaultLogger()

	assocRawData := InstanceAssociation{
		CreateDate: time.Now(),
	}

	assocRawData.Association = &ssm.InstanceAssociationSummary{}
	testExpression := "test 0 0 0/1 * * ? *"
	testCronExpression := "cron(" + testExpression + ")"
	assocRawData.Association.ScheduleExpression = &testCronExpression

	// Act
	err := assocRawData.ParseExpression(logger)

	// Assert
	assert.NotNil(t, err)
}

func TestNextScheduledDateIsCorrectWhenParsedExpressionIsValidCronExpression(t *testing.T) {

	// Assemble
	logger := log.DefaultLogger()

	assocRawData := InstanceAssociation{}

	assocRawData.Association = &ssm.InstanceAssociationSummary{}
	testAssociationName := "Test"
	assocRawData.Association.Name = &testAssociationName
	assocId := "b2f71a28-cbe1-4429-b848-26c7e1f5ad0d"
	assocRawData.Association.AssociationId = &assocId
	testExpression := "0 0 0/1 * * ? *" // hourly cron expression
	testCronExpression := "cron(" + testExpression + ")"
	assocRawData.Association.ScheduleExpression = &testCronExpression
	parsedScheduleExpression, _ := scheduleexpression.CreateScheduleExpression(logger, testCronExpression)

	assocRawData.ParsedExpression = parsedScheduleExpression
	assocRawData.Association.ScheduleExpression = &testCronExpression

	// time.Date takes following parameters:
	// Year, Month, Day, Hour, Minute, Second, Nanosecond, Location
	lastExecutionDateTime := time.Date(
		2009, 11, 17, 20, 34, 58, 651387237, time.UTC)
	assocRawData.Association.LastExecutionDate = &lastExecutionDateTime

	expectedNextScheduledDateTime := time.Date(
		2009, 11, 17, 21, 00, 00, 000000000, time.UTC)
	// Act
	assocRawData.SetNextScheduledDate(logger)

	// Assert
	assert.Equal(t, expectedNextScheduledDateTime, *assocRawData.NextScheduledDate)
}

func TestNextScheduledDateIsCorrectWhenParsedExpressionIsValidUpperCasedCronExpression(t *testing.T) {

	// Assemble
	logger := log.DefaultLogger()

	assocRawData := InstanceAssociation{}

	assocRawData.Association = &ssm.InstanceAssociationSummary{}
	testAssociationName := "Test"
	assocRawData.Association.Name = &testAssociationName
	assocId := "b2f71a28-cbe1-4429-b848-26c7e1f5ad0d"
	assocRawData.Association.AssociationId = &assocId
	testExpression := "0 0 0/1 * * ? *" // hourly cron expression
	testCronExpression := "CRON(" + testExpression + ")"
	assocRawData.Association.ScheduleExpression = &testCronExpression
	parsedScheduleExpression, _ := scheduleexpression.CreateScheduleExpression(logger, testCronExpression)

	assocRawData.ParsedExpression = parsedScheduleExpression
	assocRawData.Association.ScheduleExpression = &testCronExpression

	// time.Date takes following parameters:
	// Year, Month, Day, Hour, Minute, Second, Nanosecond, Location
	lastExecutionDateTime := time.Date(
		2009, 11, 17, 20, 34, 58, 651387237, time.UTC)
	assocRawData.Association.LastExecutionDate = &lastExecutionDateTime

	expectedNextScheduledDateTime := time.Date(
		2009, 11, 17, 21, 00, 00, 000000000, time.UTC)
	// Act
	assocRawData.SetNextScheduledDate(logger)

	// Assert
	assert.Equal(t, expectedNextScheduledDateTime, *assocRawData.NextScheduledDate)
}

func TestNextScheduledDateIsCorrectWhenExpressionIsValidCronAndHasNotBeenParsedBefore(t *testing.T) {

	// Assemble
	logger := log.DefaultLogger()

	assocRawData := InstanceAssociation{}

	assocRawData.Association = &ssm.InstanceAssociationSummary{}
	testAssociationName := "Test"
	assocRawData.Association.Name = &testAssociationName
	assocId := "b2f71a28-cbe1-4429-b848-26c7e1f5ad0d"
	assocRawData.Association.AssociationId = &assocId
	testExpression := "0 0 0/1 * * ? *" // hourly cron expression
	testCronExpression := "cron(" + testExpression + ")"
	assocRawData.Association.ScheduleExpression = &testCronExpression

	// time.Date takes following parameters:
	// Year, Month, Day, Hour, Minute, Second, Nanosecond, Location
	lastExecutionDateTime := time.Date(
		2009, 11, 17, 20, 34, 58, 651387237, time.UTC)
	assocRawData.Association.LastExecutionDate = &lastExecutionDateTime

	expectedNextScheduledDateTime := time.Date(
		2009, 11, 17, 21, 00, 00, 000000000, time.UTC)

	// Act
	assocRawData.SetNextScheduledDate(logger)

	// Assert
	assert.Equal(t, expectedNextScheduledDateTime, *assocRawData.NextScheduledDate)
}

func TestNextScheduledDateIsNilWhenCronExpressionIsInvalid(t *testing.T) {

	// Assemble
	logger := log.DefaultLogger()

	assocRawData := InstanceAssociation{}

	assocRawData.Association = &ssm.InstanceAssociationSummary{}
	testAssociationName := "Test"
	assocRawData.Association.Name = &testAssociationName
	assocId := "b2f71a28-cbe1-4429-b848-26c7e1f5ad0d"
	assocRawData.Association.AssociationId = &assocId
	testExpression := "Foo0 0 0/1 * * ? *" // hourly cron expression
	testCronExpression := "cron(" + testExpression + ")"
	assocRawData.Association.ScheduleExpression = &testCronExpression

	// time.Date takes following parameters:
	// Year, Month, Day, Hour, Minute, Second, Nanosecond, Location
	lastExecutionDateTime := time.Date(
		2009, 11, 17, 20, 34, 58, 651387237, time.UTC)
	assocRawData.Association.LastExecutionDate = &lastExecutionDateTime

	// Act
	assocRawData.SetNextScheduledDate(logger)

	// Assert
	assert.Nil(t, assocRawData.NextScheduledDate)
}

func TestNextScheduledDateIsCorrectWhenParsedExpressionIsValidRateExpression(t *testing.T) {
	// Assemble
	logger := log.DefaultLogger()

	testInstanceAssociation := InstanceAssociation{}

	testInstanceAssociation.Association = &ssm.InstanceAssociationSummary{}
	testAssociationName := "Test"
	testInstanceAssociation.Association.Name = &testAssociationName
	assocId := "b2f71a28-cbe1-4429-b848-26c7e1f5ad0d"
	testInstanceAssociation.Association.AssociationId = &assocId
	testRateExpression := "rate(3 days)"
	testInstanceAssociation.Association.ScheduleExpression = &testRateExpression
	parsedScheduleExpression, _ := scheduleexpression.CreateScheduleExpression(logger, testRateExpression)
	testInstanceAssociation.ParsedExpression = parsedScheduleExpression

	// time.Date takes following parameters:
	// Year, Month, Day, Hour, Minute, Second, Nanosecond, Location
	lastExecutionDateTime := time.Date(
		2009, 11, 17, 20, 34, 58, 651387237, time.UTC)
	testInstanceAssociation.Association.LastExecutionDate = &lastExecutionDateTime

	expectedNextScheduledDateTime := time.Date(
		2009, 11, 20, 20, 34, 58, 651387237, time.UTC)
	// Act
	testInstanceAssociation.SetNextScheduledDate(logger)

	// Assert
	assert.Equal(t, expectedNextScheduledDateTime, *testInstanceAssociation.NextScheduledDate)
}

func TestNextScheduledDateIsCorrectWhenExpressionIsValidRateAndHasNoteBeenParsedBefore(t *testing.T) {
	// Assemble
	logger := log.DefaultLogger()

	testInstanceAssociation := InstanceAssociation{}

	testInstanceAssociation.Association = &ssm.InstanceAssociationSummary{}
	testAssociationName := "Test"
	testInstanceAssociation.Association.Name = &testAssociationName
	assocId := "b2f71a28-cbe1-4429-b848-26c7e1f5ad0d"
	testInstanceAssociation.Association.AssociationId = &assocId
	testRateExpression := "rate(3 days)"
	testInstanceAssociation.Association.ScheduleExpression = &testRateExpression

	// time.Date takes following parameters:
	// Year, Month, Day, Hour, Minute, Second, Nanosecond, Location
	lastExecutionDateTime := time.Date(
		2009, 11, 17, 20, 34, 58, 651387237, time.UTC)
	testInstanceAssociation.Association.LastExecutionDate = &lastExecutionDateTime

	expectedNextScheduledDateTime := time.Date(
		2009, 11, 20, 20, 34, 58, 651387237, time.UTC)
	// Act
	testInstanceAssociation.SetNextScheduledDate(logger)

	// Assert
	assert.Equal(t, expectedNextScheduledDateTime, *testInstanceAssociation.NextScheduledDate)
}

func TestNextScheduleDateIsNilWhenRateExpressionIsInvalid(t *testing.T) {

	// Assemble
	logger := log.DefaultLogger()

	assocRawData := InstanceAssociation{}

	assocRawData.Association = &ssm.InstanceAssociationSummary{}
	testAssociationName := "Test"
	assocRawData.Association.Name = &testAssociationName
	testRateExpression := "rate(test1 day2)"
	assocId := "b2f71a28-cbe1-4429-b848-26c7e1f5ad0d"
	assocRawData.Association.AssociationId = &assocId
	assocRawData.Association.ScheduleExpression = &testRateExpression

	// time.Date takes following parameters:
	// Year, Month, Day, Hour, Minute, Second, Nanosecond, Location
	lastExecutionDateTime := time.Date(
		2009, 11, 17, 20, 34, 58, 651387237, time.UTC)
	assocRawData.Association.LastExecutionDate = &lastExecutionDateTime

	// Act
	assocRawData.SetNextScheduledDate(logger)

	// Assert
	assert.Nil(t, assocRawData.NextScheduledDate)
}

func TestNextScheduleDateIsNilWhenExpressionTypeIsUnknownAndAssociationHasPreviouslyBeenExecuted(t *testing.T) {

	// Assemble
	logger := log.DefaultLogger()

	assocRawData := InstanceAssociation{}

	assocRawData.Association = &ssm.InstanceAssociationSummary{}
	testAssociationName := "Test"
	assocRawData.Association.Name = &testAssociationName
	assocId := "b2f71a28-cbe1-4429-b848-26c7e1f5ad0d"
	assocRawData.Association.AssociationId = &assocId
	testRateExpression := "at(5 PM)"
	assocRawData.Association.ScheduleExpression = &testRateExpression

	// Setting last execution date implies that association has been executed already.
	// time.Date takes following parameters:
	// Year, Month, Day, Hour, Minute, Second, Nanosecond, Location
	lastExecutionDateTime := time.Date(
		2009, 11, 17, 20, 34, 58, 651387237, time.UTC)
	assocRawData.Association.LastExecutionDate = &lastExecutionDateTime

	// Act
	assocRawData.SetNextScheduledDate(logger)

	// Assert
	assert.Nil(t, assocRawData.NextScheduledDate)
}

func TestNextScheduledDateIsNilWhenAssociationExpressionTypeIsUnknown(t *testing.T) {

	// Assemble
	logger := log.DefaultLogger()

	assocRawData := InstanceAssociation{}

	assocRawData.Association = &ssm.InstanceAssociationSummary{}
	testAssociationName := "Test"
	assocRawData.Association.Name = &testAssociationName
	assocId := "b2f71a28-cbe1-4429-b848-26c7e1f5ad0d"
	assocRawData.Association.AssociationId = &assocId
	testRateExpression := "at(5 PM)"
	assocRawData.Association.ScheduleExpression = &testRateExpression

	// Setting last execution date implies that association has been executed already.
	// time.Date takes following parameters:
	// Year, Month, Day, Hour, Minute, Second, Nanosecond, Location
	lastExecutionDateTime := time.Date(
		2009, 11, 17, 20, 34, 58, 651387237, time.UTC)
	assocRawData.Association.LastExecutionDate = &lastExecutionDateTime

	// Act
	assocRawData.SetNextScheduledDate(logger)

	// Assert
	assert.Nil(t, assocRawData.NextScheduledDate)
}
