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

// Package network contains a network gatherer.
package network

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/inventory/model"
)

// GetAdvancedNetworkData gets advanced network information in linux platform
func GetAdvancedNetworkData(context context.T, data []model.NetworkData) []model.NetworkData {
	log := context.Log()
	log.Infof("Unable to get further information about network interfaces in linux platform")
	return data
}
