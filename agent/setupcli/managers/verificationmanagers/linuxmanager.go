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

// Package verificationmanagers is used to verify the agent packages
package verificationmanagers

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/common"
)

var (
	ioWriteUtil      = ioutil.WriteFile
	fileUtilMakeDirs = fileutil.MakeDirs
)

type linuxManager struct {
	managerHelper common.IManagerHelper
}

func (l *linuxManager) createPublicKeyFile(publicKeyPath string) error {
	data := GetLinuxPublicKey()
	return ioWriteUtil(publicKeyPath, data, appconfig.ReadWriteAccess)
}

// VerifySignature verifies the agent binary signature
func (l *linuxManager) VerifySignature(log log.T, signaturePath string, artifactsPath string, fileExtension string) error {
	binaryPath := filepath.Join(artifactsPath, appconfig.DefaultAgentName+fileExtension)

	gpgExtension := ".gpg"
	amazonSSMAgentGPGKey := filepath.Join(artifactsPath, appconfig.DefaultAgentName+gpgExtension)

	//create public key file
	log.Infof("Creating public key file at: %s", amazonSSMAgentGPGKey)
	if err := l.createPublicKeyFile(amazonSSMAgentGPGKey); err != nil {
		return fmt.Errorf("failed to create amazon-ssm-agent.gpg file: %v", err)
	}

	//check to see if gpg is installed
	log.Infof("Checking to see if gpg is installed")
	if !l.managerHelper.IsCommandAvailable("gpg") {
		return fmt.Errorf("gpg is not installed. Please install gpg to validate the signature of binaries or pass -skip-signature-validation flag")
	}

	tempKeyRing := "keyring"
	keyringPath := filepath.Join(artifactsPath, tempKeyRing)
	err := fileUtilMakeDirs(keyringPath)
	if err != nil {
		return fmt.Errorf("keyring directory creation failed: %v", err)
	}

	log.Debugf("Importing public key: gpg --import %s", amazonSSMAgentGPGKey)
	output, err := l.managerHelper.RunCommand("gpg", "--no-default-keyring", "--keyring", keyringPath, "--import", amazonSSMAgentGPGKey)
	if err != nil {
		if l.managerHelper.IsTimeoutError(err) {
			return fmt.Errorf("gpg command timed out")
		}
	}
	log.Infof("Successfully imported keyring: %v", output)

	log.Info("Verifying agent signature")
	output, err = l.managerHelper.RunCommand("gpg", "--no-default-keyring", "--keyring", keyringPath, "--verify", signaturePath, binaryPath)
	if err != nil {
		if l.managerHelper.IsTimeoutError(err) {
			return fmt.Errorf("gpg verify: command timed out")
		}
		return fmt.Errorf("gpg verify: failed to verify signature using gpg with output '%v' and error: %v", output, err)
	}
	goodSignatureText := "Good signature from \"SSM Agent <ssm-agent-signer@amazon.com>\""
	if !strings.Contains(output, goodSignatureText) {
		return fmt.Errorf("signature verification failed %v", output)
	}
	log.Infof("Successfully verified signature")
	return nil
}
