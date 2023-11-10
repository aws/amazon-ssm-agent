// Copyright 2023 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

//go:build windows
// +build windows

// Package helpers contains helper functions for SSM-Setup-CLI
package helpers

import (
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/packagemanagers"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/servicemanagers"
)

// UninstallAgent uninstalls SSM agent installed on the instance
func UninstallAgent(log log.T, pkgManager packagemanagers.IPackageManager, sManager servicemanagers.IServiceManager, installedVersionPath string) error {
	return pkgManager.UninstallAgent(log, installedVersionPath)
}
