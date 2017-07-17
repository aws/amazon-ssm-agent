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
// +build linux

package startup

import (
	"errors"
	"fmt"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/startup/serialport"
	"github.com/aws/amazon-ssm-agent/agent/version"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
)

const (
	// Retry max count for opening serial port
	serialPortRetryMaxCount = 10

	// Wait time before retrying to open serial port
	serialPortRetryWaitTime = 5
)

// IsAllowed returns true if the current environment allows startup processor.
func (p *Processor) IsAllowed() bool {
	// check if metadata is reachable which indicates the instance is in EC2.
	// maximum retry is 10 to ensure the failure/error is not caused by arbitrary reason.
	ec2MetadataService := ec2metadata.New(session.New(aws.NewConfig().WithMaxRetries(10)))
	if metadata, err := ec2MetadataService.GetMetadata(""); err != nil || metadata == "" {
		return false
	}

	return true
}

// ExecuteTasks executes startup tasks in unix platform.
func (p *Processor) ExecuteTasks() (err error) {
	var sp *serialport.SerialPort

	log := p.context.Log()
	log.Info("Executing startup processor tasks")

	platformName := ""
	if n, err := platform.PlatformName(log); err == nil {
		platformName = *aws.String(n)
	} else {
		log.Warn(err)
	}

	platformVersion := ""
	if v, err := platform.PlatformVersion(log); err == nil {
		platformVersion = *aws.String(v)
	} else {
		log.Warn(err)
	}

	// attempt to initialize and open the serial port.
	// since only three minute is allowed to write logs to console during boot,
	// it attempts to open serial port for approximately three minutes.
	retryCount := 0
	for retryCount < serialPortRetryMaxCount {
		sp = serialport.NewSerialPort(log)
		if err = sp.OpenPort(); err != nil {
			log.Errorf("%v. Retrying in %v seconds...", err.Error(), serialPortRetryWaitTime)
			time.Sleep(serialPortRetryWaitTime * time.Second)
			retryCount++
		} else {
			break
		}

		// if the retry count hits the maximum count, log the error and return.
		if retryCount == serialPortRetryMaxCount {
			err = errors.New("Timeout: Serial port is in use or not available")
			log.Errorf("Error occurred while opening serial port: %v", err.Error())
			return
		}
	}

	// defer is set to close the serial port during unexpected.
	defer func() {
		//serial port MUST be closed.
		sp.ClosePort()
	}()

	// write the agent version to serial port.
	sp.WritePort(fmt.Sprintf("Amazon SSM Agent v%v is running", version.Version))

	// write the platform name and version to serial port.
	sp.WritePort(fmt.Sprintf("OsProductName: %v", platformName))
	sp.WritePort(fmt.Sprintf("OsVersion: %v", platformVersion))

	return nil
}
