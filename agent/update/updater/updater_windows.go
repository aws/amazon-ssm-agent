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

// +build windows

// Package main represents the entry point of the ssm agent updater.
package main

import (
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
)

// resolveUpdateRoot returns the platform specific path to update artifacts
func resolveUpdateRoot(sourceVersion string) (string, error) {
	return appconfig.UpdaterArtifactsRoot, nil
}
