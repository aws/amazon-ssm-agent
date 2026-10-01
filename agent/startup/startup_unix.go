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
//go:build linux
// +build linux

package startup

import (
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/startup/serialport"
	"github.com/aws/amazon-ssm-agent/agent/version"
)

// IsAllowed returns true if the current environment allows startup processor.
// TODO: Check if is EC2 identity type instead of calling imds, first confirm if onprem on ec2 should be allowed
func (p *Processor) IsAllowed() bool {
	return serialport.IsEC2Environment(p.context.Log())
}

// ExecuteTasks executes startup tasks in unix platform.
func (p *Processor) ExecuteTasks() (err error) {
	log := p.context.Log()
	log.Info("Executing startup processor tasks")

	platformName := ""
	if n, err := platform.PlatformName(log); err == nil {
		platformName = n
	} else {
		log.Warn(err)
	}

	platformVersion := ""
	if v, err := platform.PlatformVersion(log); err == nil {
		platformVersion = v
	} else {
		log.Warn(err)
	}

	sp, err := serialport.NewSerialPortWithRetry(log)
	if err != nil {
		log.Errorf("Error occurred while opening serial port: %v", err.Error())
		return
	}

	// defer is set to close the serial port during unexpected.
	defer func() {
		// serial port MUST be closed.
		sp.ClosePort()
	}()

	// write the agent version to serial port.
	sp.WritePort(fmt.Sprintf("Amazon SSM Agent v%v is running", version.Version))

	// write the platform name and version to serial port.
	sp.WritePort(fmt.Sprintf("OsProductName: %v", platformName))
	sp.WritePort(fmt.Sprintf("OsVersion: %v", platformVersion))

	return nil
}
