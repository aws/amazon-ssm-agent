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
// Package rundaemon implements rundaemon plugin and its configuration
package rundaemon

import "github.com/aws/amazon-ssm-agent/agent/contracts"

// DaemonPluginInput represents an action to run a package as a daemon.
type DaemonPluginInput struct {
	contracts.PluginInput
	Name            string `json:"name"`
	Action          string `json:"action"`
	PackageLocation string `json:"packagelocation"`
	Command         string `json:"command"`
}
