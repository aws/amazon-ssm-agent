// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package testutils represents the common logic needed for agent tests
package testutils

import (
	"github.com/aws/amazon-ssm-agent/agent/agentlogstocloudwatch/cloudwatchlogspublisher"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/coremanager"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/rebooter"
)

// NewCoreManager creates coremanager with mock mds service injected
func NewCoreManager(context context.T, coremodules *[]contracts.ICoreModule, log log.T) (cpm *coremanager.CoreManager, err error) {
	instanceIDStr, _ := platform.InstanceID()
	regionStr := TestRegion
	instanceIDPtr := &instanceIDStr
	regionPtr := &regionStr
	cloudwatchPublisher := &cloudwatchlogspublisher.CloudWatchPublisher{}
	reboot := &rebooter.SSMRebooter{}
	cpm, err = coremanager.NewCoreManager(context, *coremodules, cloudwatchPublisher, instanceIDPtr, regionPtr, log, reboot)
	return
}
