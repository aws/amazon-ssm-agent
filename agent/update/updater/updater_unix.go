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

//go:build darwin || freebsd || linux || netbsd || openbsd
// +build darwin freebsd linux netbsd openbsd

// Package main represents the entry point of the ssm agent updater.
package main

import (
	"runtime"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/session/utility"
	"github.com/aws/amazon-ssm-agent/agent/versionutil"
)

const (
	legacyUpdaterArtifactsRoot   = "/var/log/amazon/ssm/update/"
	firstAgentWithNewUpdaterPath = "1.1.86.0"
)

// resolveUpdateRoot returns the platform specific path to update artifacts
func resolveUpdateRoot(sourceVersion string) (string, error) {
	compareResult, err := versionutil.VersionCompare(sourceVersion, firstAgentWithNewUpdaterPath)
	if err != nil {
		return "", err
	}
	// New versions that with new binary locations
	if compareResult >= 0 {
		return appconfig.UpdaterArtifactsRoot, nil
	}

	return legacyUpdaterArtifactsRoot, nil
}

func updateSSMUserShellProperties(log logger.T) {
	if runtime.GOOS == "darwin" {
		var ssmSessionUtil utility.SessionUtil
		if ok, _ := ssmSessionUtil.DoesUserExist(appconfig.DefaultRunAsUserName); ok {
			if err := ssmSessionUtil.ChangeUserShell(); err != nil {
				log.Warnf("UserShell Update Failed: %v", err)
				return
			}
			log.Infof("Successfully updated ssm-user shell properties")
		}
	}
	return
}
