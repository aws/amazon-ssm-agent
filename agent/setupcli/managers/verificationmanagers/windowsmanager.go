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

// Package verificationmanagers is used to verify the agent packages
package verificationmanagers

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/common"
)

var (
	isNanoPlatform = platform.IsPlatformNanoServer
)

type windowsManager struct {
	managerHelper common.IManagerHelper
}

// VerifySignature verifies the agent binary signature
func (w *windowsManager) VerifySignature(log log.T, signaturePath string, artifactsPath string, fileExtension string) error {
	//check to see if signtool is installed
	isNano, _ := isNanoPlatform(log)
	setupPath := ""
	if isNano {
		setupPath = filepath.Join(artifactsPath, common.AmazonSSMExecutable)
	} else {
		setupPath = filepath.Join(artifactsPath, common.AmazonWindowsSetupFile)
	}

	output, err := w.managerHelper.RunCommand(appconfig.PowerShellPluginCommandName, "(Get-AuthenticodeSignature", "-FilePath", "\""+setupPath+"\").SignerCertificate.SubjectName.Name")
	if err != nil {
		if w.managerHelper.IsTimeoutError(err) {
			return fmt.Errorf("certificate subject name: Command timed out")
		}
		return fmt.Errorf("certificate subject name: Failed to verify signature with output '%v' and error: %v", output, err)
	}

	amazonCommonName := "Amazon.com Services LLC"
	subjectName := parseSubjectName(output)
	if subjectName != amazonCommonName {
		return fmt.Errorf("cerificate identity is not valid")
	}

	log.Infof("Verifying signature using sign tool verify agent in %s", setupPath)
	output, err = w.managerHelper.RunCommand(appconfig.PowerShellPluginCommandName, "(Get-AuthenticodeSignature", "-FilePath", "\""+setupPath+"\").Status")
	//return error of failed verification
	if err != nil {
		if w.managerHelper.IsTimeoutError(err) {
			return fmt.Errorf("signature verification: Command timed out")
		}
		return fmt.Errorf("signature verification failed: %v", err)
	}
	ValidStr := "Valid"
	if strings.TrimSpace(output) == ValidStr {
		log.Infof("Successfully verified signature")
		return nil
	}
	return fmt.Errorf("invalid signing: output - %v, err - %v", output, err)
}

func parseSubjectName(subjectName string) string {
	commonNamePrefix := "CN="
	for _, val := range strings.Split(subjectName, ", ") {
		if strings.HasPrefix(val, commonNamePrefix) {
			values := strings.Split(val, commonNamePrefix)
			if len(values) > 1 {
				return strings.TrimSpace(values[1])
			}
		}
	}
	return ""
}
