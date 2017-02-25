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

// Package rebooter provides utilities used to reboot a machine.
package rebooter

import "github.com/aws/amazon-ssm-agent/agent/log"

type RebootType string

const (
	RebootRequestTypeReboot RebootType = "reboot"
	RebootRequestTypeUpdate RebootType = "update"
)

var ch = make(chan RebootType)

func GetChannel() chan RebootType {
	return ch
}

//RebootMachine reboots the machine
func RebootMachine(log log.T) {

	if err := reboot(log); err != nil {
		log.Error("error in rebooting the machine", err)
		return
	}
}

func RequestPendingReboot(log log.T) bool {

	//non-blocking send
	select {
	case ch <- RebootRequestTypeReboot:
		log.Info("successfully requested a reboot")
		return true
	default:
		log.Info("reboot has already been requested...")
		return false
	}

}
