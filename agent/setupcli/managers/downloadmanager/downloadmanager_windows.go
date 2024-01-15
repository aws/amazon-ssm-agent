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

// Package downloadmanager helps us with file download related functions in ssm-setup-cli
package downloadmanager

import (
	"fmt"
	"path/filepath"

	"github.com/aws/amazon-ssm-agent/agent/log"
)

const (
	// packageZip denotes name of the package that contains agent binaries for Nano platform
	packageZip = "package.zip"
)

func (d *downloadManager) fileUnCompress(log log.T, agentSetupFilePath string, artifactsStorePath string) error {
	var err error
	// Un-compress downloaded files
	if err = fileUtilUnCompress(log, agentSetupFilePath, artifactsStorePath); err != nil {
		return fmt.Errorf("failed to uncompress agent installation package, %v", err)
	}
	if d.isNano {
		err = fileUtilUnCompress(d.log, filepath.Join(artifactsStorePath, packageZip), artifactsStorePath)
	}
	return err
}

func (d *downloadManager) DownloadSignatureFile(version, artifactsStorePath, extension string) (path string, err error) {
	return "", nil
}

func hasLowerKernelVersion() bool {
	return false
}
