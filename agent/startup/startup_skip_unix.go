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
//
// +build freebsd netbsd openbsd darwin

package startup

// IsAllowed returns true if the current platform allows startup processor.
// Implement this if ExecuteTasks is implemented.
func (p *Processor) IsAllowed() bool {
	// return false since no startup tasks available for unix platform.
	return false
}

// ExecuteTasks executes startup tasks in unix platform.
func (p *Processor) ExecuteTasks() error {
	return nil
}
