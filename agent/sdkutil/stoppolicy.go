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

// Package sdkutil provides utilities used to call awssdk.
package sdkutil

import (
	"fmt"
	"sync"
)

// StopPolicy specifies the execution policy on data points like errors, duration
type StopPolicy struct {
	Name                  string
	errorCount            int
	MaximumErrorThreshold int
	SyncObject            *sync.Mutex
}

// NewStopPolicy creates an object of StopPolicy
func NewStopPolicy(name string, errorThreshold int) *StopPolicy {
	s := new(StopPolicy)
	s.Name = name
	s.errorCount = 0
	s.MaximumErrorThreshold = errorThreshold
	s.SyncObject = new(sync.Mutex)
	return s
}

// ProcessException sets provides a default implementation when errors occur
func (s *StopPolicy) ProcessException(err error) {
	s.AddErrorCount(1)
}

// IsHealthy returns true if the policy determines the handler is safe to call otherwise false.
func (s *StopPolicy) IsHealthy() (healthy bool) {
	s.SyncObject.Lock()
	defer s.SyncObject.Unlock()
	if s.MaximumErrorThreshold == 0 {
		return true
	}

	// now if either of policy allows we allow the agent to proceed
	return (0 < s.MaximumErrorThreshold && s.errorCount < s.MaximumErrorThreshold)
}

// AddErrorCount increments the error count by the set amount
func (s *StopPolicy) AddErrorCount(x int) {
	s.SyncObject.Lock()
	defer s.SyncObject.Unlock()
	s.errorCount += x
}

// ResetErrorCount resets the error count, typically on successful operation
func (s *StopPolicy) ResetErrorCount() {
	s.SyncObject.Lock()
	defer s.SyncObject.Unlock()
	s.errorCount = 0
}

// String returns the string representation of the stop policy
func (s *StopPolicy) String() string {
	s.SyncObject.Lock()
	defer s.SyncObject.Unlock()
	return fmt.Sprintf("{Name: %v; errorCount: %v; MaximumErrorThreshold: %v}",
		s.Name, s.errorCount, s.MaximumErrorThreshold)
}
