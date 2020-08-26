// Copyright 2019 The Mangos Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use file except in compliance with the License.
// You may obtain a copy of the license at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES O R CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package test

import (
	"reflect"
	"runtime"
	"testing"
)

// MustSucceed verifies that that the supplied error is nil.
// If it is not nil, we call t.Fatalf() to fail the test immediately.
func MustSucceed(t *testing.T, e error) {
	if e != nil {
		_, file, line, _ := runtime.Caller(1)
		t.Fatalf("Failed at %s:%d: Error is not nil: %v",
			file, line, e)
	}
}

// MustFail verifies that the error is not nil.
// If it is nil, the test is a fatal failure.
func MustFail(t *testing.T, e error) {
	if e == nil {
		_, file, line, _ := runtime.Caller(1)
		t.Fatalf("Failed at %s:%d: Error is nil", file, line)
	}
}

// MustBeError verifies that a value is a specific error.
func MustBeError(t *testing.T, e error, compare error) {

	if e == nil {
		_, file, line, _ := runtime.Caller(1)
		t.Fatalf("Failed at %s:%d: Error is nil", file, line)
	}
	if e.Error() != compare.Error() {
		_, file, line, _ := runtime.Caller(1)
		t.Fatalf("Failed at %s:%d: Error was %v, expected %v",
			file, line, e, compare)
	}
}

// MustBeTrue verifies that the condition is true.
func MustBeTrue(t *testing.T, b bool) {
	if !b {
		_, file, line, _ := runtime.Caller(1)
		t.Fatalf("Failed at %s:%d: Condition is false", file, line)
	}
}

// MustBeFalse verifies that the condition is true.
func MustBeFalse(t *testing.T, b bool) {
	if b {
		_, file, line, _ := runtime.Caller(1)
		t.Fatalf("Failed at %s:%d: Condition is true", file, line)
	}
}

// MustNotBeNil verifies that the provided value is not nil
func MustNotBeNil(t *testing.T, v interface{}) {
	if reflect.ValueOf(v).IsNil() {
		_, file, line, _ := runtime.Caller(1)
		t.Fatalf("Failed at %s:%d: Value is nil", file, line)
	}
}

// MustBeNil verifies that the provided value is nil
func MustBeNil(t *testing.T, v interface{}) {
	if !reflect.ValueOf(v).IsNil() {
		_, file, line, _ := runtime.Caller(1)
		t.Fatalf("Failed at %s:%d: Value is not nil", file, line)
	}
}
