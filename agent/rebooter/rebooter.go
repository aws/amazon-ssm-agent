// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Amazon Software License (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/asl/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package rebooter provides utilities used to reboot a machine.
package rebooter

import (
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/log"
)

var rebootRequestCount uint
var rebootInitiated = false
var syncObject sync.Mutex

// RebootMachine reboots the machine
func RebootMachine(log log.T) {
	log.Info("Executing reboot request...")
	if RebootInitiated() {
		return
	}

	syncObject.Lock()
	defer syncObject.Unlock()
	if err := reboot(log); err != nil {
		log.Error("error in rebooting the machine", err)
		return
	}
	rebootInitiated = true
}

// RequestPendingReboot requests a pending reboot.
// A reboot will be initiated by the agent at an appropriate time.
func RequestPendingReboot() {
	syncObject.Lock()
	defer syncObject.Unlock()
	rebootRequestCount++
}

// RebootInitiated returns whether the Reboot request has initiated.
func RebootInitiated() bool {
	syncObject.Lock()
	defer syncObject.Unlock()
	return rebootInitiated
}

// RebootRequestCount returns the reboot request count.
func RebootRequestCount() uint {
	syncObject.Lock()
	defer syncObject.Unlock()
	return rebootRequestCount
}

// RebootRequested returns the reboot request count.
func RebootRequested() bool {
	syncObject.Lock()
	defer syncObject.Unlock()
	return rebootRequestCount > 0
}
