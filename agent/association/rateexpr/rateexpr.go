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

// Package rateexpr provides logic for parsing and scheduling rate expressions
package rateexpr

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

/******************************************************************************/

// A Expression represents a specific rate time expression as defined at
// <http://docs.aws.amazon.com/AmazonCloudWatch/latest/events/ScheduledEvents.html#RateExpressions>
type RateExpression struct {
	intervalInSeconds int64
}

/******************************************************************************/

// Parse returns a new RateExpression pointer. An error is returned if a malformed
// rate expression is supplied.
// See <http://docs.aws.amazon.com/AmazonCloudWatch/latest/events/ScheduledEvents.html#RateExpressions> for documentation
// about what is a well-formed rate expression from this library's point of
// view.
func Parse(rateLine string) (*RateExpression, error) {
	rateRegularExpression := regexp.MustCompile("(?i)(rate\\((\\d+)(\\s+)*(minute|minutes|hour|hours|day|days)\\))")
	result := rateRegularExpression.FindAllStringSubmatch(rateLine, -1)
	if len(result) != 1 {
		return nil, fmt.Errorf("Schedule expression is not a valid rate expression.")
	}

	match := result[0]
	if match == nil {
		return nil, fmt.Errorf("Schedule expression is not a valid rate expression.")
	}

	if len(match) == 5 && match[1] != "" {
		if len(match[1]) != len(rateLine) {
			return nil, fmt.Errorf("Schedule expression is not a valid rate expression.")
		}

		frequency, err := strconv.ParseInt(match[2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("Schedule expression is not a valid rate expression. Time value should be a positive number.")
		}

		timeUnit := strings.ToLower(match[4])

		if frequency == 0 {
			return nil, fmt.Errorf("Schedule expression is not a valid rate expression. Time value should be a positive number.")
		}

		var expr = RateExpression{}

		if timeUnit == "minute" || timeUnit == "minutes" {
			expr.intervalInSeconds = frequency * 60
			return &expr, nil
		}

		if timeUnit == "hour" || timeUnit == "hours" {
			expr.intervalInSeconds = frequency * 60 * 60
			return &expr, nil
		}

		if timeUnit == "day" || timeUnit == "days" {
			expr.intervalInSeconds = frequency * 24 * 60 * 60
			return &expr, nil
		}
	}

	return nil, fmt.Errorf("Schedule expression is not a valid rate expression.")
}

// Next returns the closest time instant immediately following `fromTime` which
// matches the rate expression `expr`.
//
// The `time.Location` of the returned time instant is the same as that of
// `fromTime`.
//
// The zero value of time.Time is returned if no matching time instant exists
// or if a `fromTime` is itself a zero value.
func (expr *RateExpression) Next(fromTime time.Time) time.Time {
	// Special case
	if fromTime.IsZero() {
		return fromTime
	}

	var d = time.Duration(expr.intervalInSeconds * 1000 * 1000 * 1000)
	return fromTime.Add(d)
}
