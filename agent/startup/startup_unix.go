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
	"context"
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/startup/serialport"
	"github.com/aws/amazon-ssm-agent/agent/version"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
)

// IsAllowed returns true if the current environment allows startup processor.
func (p *Processor) IsAllowed() bool {
	log := p.context.Log()
	// TODO: Check if is EC2 identity type instead of calling imds, first confirm if onprem on ec2 should be allowed
	// check if metadata is reachable which indicates the instance is in EC2.
	// maximum retry is 10 to ensure the failure/error is not caused by arbitrary reason.
	// IMDS ignores RetryMaxAttempts and only honors a Retryer func, so bound
	// retries via WithRetryer instead of WithRetryMaxAttempts.
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRetryer(func() aws.Retryer {
		return retry.NewStandard(func(o *retry.StandardOptions) {
			o.MaxAttempts = 10
		})
	}))
	if err != nil {
		// LoadDefaultConfig can fail for reasons unrelated to EC2 detection
		// (e.g. unreadable shared config/credentials files). Rather than
		// treating this as "not in EC2", log the error and fall back to a
		// minimal config so we can still probe the metadata service.
		log.Warnf("failed to load default AWS config, falling back to minimal config: %v", err)
		cfg = aws.Config{
			RetryMaxAttempts: 10,
		}
	}

	client := imds.NewFromConfig(cfg)
	out, err := client.GetMetadata(context.TODO(), &imds.GetMetadataInput{Path: ""})
	if err != nil {
		return false
	}
	// The GetMetadata output Content is an io.ReadCloser that must be closed to
	// avoid leaking the underlying HTTP response body.
	defer out.Content.Close()

	return true
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
