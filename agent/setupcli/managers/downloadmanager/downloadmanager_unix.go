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

//go:build freebsd || linux || netbsd || openbsd
// +build freebsd linux netbsd openbsd

// Package downloadmanager helps us with file download related functions in ssm-setup-cli
package downloadmanager

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/versionutil"
)

const (
	minLinuxKernelVersion = "3.2"
)

func (d *downloadManager) fileUnCompress(log log.T, agentSetupFilePath string, artifactsStorePath string) error {
	// Un-compress downloaded files
	if err := fileUtilUnCompress(log, agentSetupFilePath, artifactsStorePath); err != nil {
		return fmt.Errorf("failed to uncompress agent installation package, %v", err)
	}
	return nil
}

func (d *downloadManager) DownloadSignatureFile(version, artifactsStorePath, extension string) (path string, err error) {
	folderName := d.updateInfo.GeneratePlatformBasedFolderName()
	signatureFileURL := d.getS3BucketUrl() + "/" + version + "/" + folderName + "/" + appconfig.DefaultAgentName + extension + ".sig"
	agentSetupFilePath, err := utilHttpDownload(d.log, signatureFileURL, artifactsStorePath)
	if err != nil {
		return "", fmt.Errorf("error while downloading signatureFile")
	}
	return agentSetupFilePath, err
}

func hasLowerKernelVersion() bool {
	byteOutput, err := exec.Command("uname", "-r").Output()
	if err != nil {
		return false
	}
	splitVersion := strings.Split(strings.TrimSpace(string(byteOutput)), ".")
	if len(splitVersion) < 3 {
		return false
	}

	// Join major + minor version
	kernelVersion := strings.Join(splitVersion[:2], ".")
	comp, err := versionutil.VersionCompare(kernelVersion, minLinuxKernelVersion)
	if err != nil {
		return false
	}

	if comp < 0 {
		return true // only true case
	}

	return false
}
