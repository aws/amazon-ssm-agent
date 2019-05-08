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

// +build darwin freebsd linux netbsd openbsd

// Package gatherers contains routines for different types of inventory gatherers
package gatherers

import (
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/application"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/awscomponent"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/billinginfo"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/custom"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/file"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/instancedetailedinformation"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/network"
)

var supportedGathererNames = []string{
	application.GathererName,
	awscomponent.GathererName,
	custom.GathererName,
	billinginfo.GathererName,
	network.GathererName,
	file.GathererName,
	instancedetailedinformation.GathererName,
}
