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

// Package times provides a set of utilities related to processing time.
package times

import (
	"fmt"
	"time"
)

// Clock is an interface that can provide time related functionality.
type Clock interface {
	// Now returns the current time.
	Now() time.Time

	// After returns a channel that will receive after the given duration.
	After(time.Duration) <-chan time.Time
}

// DefaultClock implements Clock by delegating to methods in package time.
var DefaultClock = &defaultClock{}

// defaultClock implements Clock by delegating to methods in package time.
type defaultClock struct{}

// Now returns the current time.
func (defaultClock) Now() time.Time {
	return time.Now()
}

// After returns a channel that will receive after the given duration has elapsed.
func (defaultClock) After(d time.Duration) <-chan time.Time {
	return time.After(d)
}

// ToIso8601UTC converts a time into a string in Iso8601 format in UTC timezone (yyyy-MM-ddTHH:mm:ss.fffZ).
func ToIso8601UTC(t time.Time) string {
	t = t.UTC()
	return fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:%02d.%03dZ", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond()/1000000)
}

// ToIsoDashUTC converts a time into a string in Iso format yyyy-MM-ddTHH-mm-ss.fffZ in UTC timezone.
func ToIsoDashUTC(t time.Time) string {
	t = t.UTC()
	return fmt.Sprintf("%04d-%02d-%02dT%02d-%02d-%02d.%03dZ", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond()/1000000)
}

// ParseIso8601UTC parses a time in Iso8601 format and UTC timezone (yyyy-MM-ddTHH:mm:ss.fffZ).
func ParseIso8601UTC(t string) time.Time {
	var y int
	var m time.Month
	var d int
	var h int
	var min int
	var s int
	var ms int
	fmt.Sscanf(t, "%04d-%02d-%02dT%02d:%02d:%02d.%03dZ", &y, &m, &d, &h, &min, &s, &ms)
	return time.Date(y, m, d, h, min, s, ms*1000000, time.UTC)
}

// ParseIsoDashUTC parses a time in IsoDash format and UTC timezone (yyyy-MM-ddTHH-mm-ss.fffZ).
func ParseIsoDashUTC(t string) (time.Time, error) {
	var y int
	var m time.Month
	var d int
	var h int
	var min int
	var s int
	var ms int

	if _, err := fmt.Sscanf(t, "%04d-%02d-%02dT%02d-%02d-%02d.%03dZ", &y, &m, &d, &h, &min, &s, &ms); err != nil {
		return time.Time{}, err
	}
	return time.Date(y, m, d, h, min, s, ms*1000000, time.UTC), nil
}
