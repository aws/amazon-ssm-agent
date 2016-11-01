// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the AWS Customer Agreement (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/agreement/

// +build windows

// Package gatherers contains routines for different types of inventory gatherers
package gatherers

import (
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/application"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/awscomponent"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/custom"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/network"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/windowsUpdate"
)

var supportedGathererNames = []string{
	application.GathererName,
	awscomponent.GathererName,
	custom.GathererName,
	network.GathererName,
	windowsUpdate.GathererName,
}
