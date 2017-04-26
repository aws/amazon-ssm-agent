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

// Package scheduleexpression provides interface for schedule expression and factory for constructing generic parsed
// schedule expression
package scheduleexpression

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/association/rateexpr"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/gorhill/cronexpr"
)

const (
	expressionTypeCron = "cron"
	expressionTypeRate = "rate"
)

//ScheduleExpression defines operations of a valid schedule expression which association/model makes use of
type ScheduleExpression interface {
	Next(fromTime time.Time) time.Time
}

func CreateScheduleExpression(log log.T, scheduleExpression string) (ScheduleExpression, error) {

	lowerCasedScheduledExpression := strings.ToLower(scheduleExpression)

	if strings.HasPrefix(lowerCasedScheduledExpression, expressionTypeCron) {
		err := validateCronExpression(log, scheduleExpression)
		if err != nil {
			return nil, fmt.Errorf(err.Error())
		}

		cronExpression := scheduleExpression[len(expressionTypeCron)+1 : len(scheduleExpression)-1]
		parsedCronExpression, err := cronexpr.Parse(cronExpression)

		if err == nil {
			return parsedCronExpression, nil
		} else {
			message := fmt.Sprintf("Error %v received while parsing cron expression %v", err, scheduleExpression)
			log.Error(message)
			return nil, fmt.Errorf(message)
		}
	}

	if strings.HasPrefix(lowerCasedScheduledExpression, expressionTypeRate) {
		parsedRateExpression, err := rateexpr.Parse(scheduleExpression)

		if err == nil {
			return parsedRateExpression, nil
		} else {
			message := fmt.Sprintf("An error %v received while parsing rate expression %v", err, scheduleExpression)
			log.Error(message)
			return nil, fmt.Errorf(message)
		}
	}

	return nil, fmt.Errorf("Unknown expression type detected in expression %v", scheduleExpression)
}

func validateCronExpression(log log.T, scheduleExpression string) error {
	cronRegularExpression := regexp.MustCompile("(?i)(cron\\(.*\\))")
	result := cronRegularExpression.FindAllStringSubmatch(scheduleExpression, -1)
	errorMessage := fmt.Sprintf("Cron expression %v is invalid.", scheduleExpression)

	if len(result) != 1 {
		log.Error(errorMessage)
		return fmt.Errorf(errorMessage)
	}

	match := result[0]
	if match == nil {
		log.Error(errorMessage)
		return fmt.Errorf(errorMessage)
	}

	if len(match) == 2 && match[1] != "" {
		// Ensure we do not match cron(0 0 0/1 * * ? *)abc
		if len(match[1]) != len(scheduleExpression) {
			log.Error(errorMessage)
			return fmt.Errorf(errorMessage)
		}
	}

	return nil
}
