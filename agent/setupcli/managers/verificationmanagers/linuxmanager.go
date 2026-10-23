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
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/common"
)

var (
	ioWriteUtil = ioutil.WriteFile
	sleepFunc   = time.Sleep
)

type linuxManager struct {
	managerHelper common.IManagerHelper
}

func (l *linuxManager) createPublicKeyFile(publicKeyPath string) error {
	data := GetLinuxPublicKey()
	return ioWriteUtil(publicKeyPath, data, appconfig.ReadWriteAccess)
}

// createDearmoredPublicKeyFile creating dearmored key for gpgv
func (l *linuxManager) createDearmoredPublicKeyFile(publicKeyPath string) error {
	dearmored, err := GetLinuxPublicKeyDearmored()
	if err != nil {
		return fmt.Errorf("failed to dearmor public key: %v", err)
	}
	return ioWriteUtil(publicKeyPath, dearmored, appconfig.ReadWriteAccess)
}

func (l *linuxManager) createKeyRingFile(keyRingFilePath string) error {
	return ioWriteUtil(keyRingFilePath, []byte{}, appconfig.ReadWriteAccess)
}

// VerifySignature verifies the agent binary signature
func (l *linuxManager) VerifySignature(log log.T, signaturePath string, artifactsPath string, fileExtension string) error {
	binaryPath := filepath.Join(artifactsPath, appconfig.DefaultAgentName+fileExtension)

	gpgExtension := ".gpg"
	amazonSSMAgentGPGKey := filepath.Join(artifactsPath, appconfig.DefaultAgentName+gpgExtension)

	//check to see if gpg is installed
	log.Infof("Checking to see if gpg is installed")
	if !l.managerHelper.IsCommandAvailable("gpg") {
		// gpg not available, try gpgv as fallback
		log.Infof("gpg is not available, checking for gpgv")
		if !l.managerHelper.IsCommandAvailable("gpgv") {
			// gpgv also unavailable; try sqv (Debian 13 ships sqv as its OpenPGP verifier) before in-process.
			if l.managerHelper.IsCommandAvailable("sqv") {
				log.Infof("gpg and gpgv not available, using sqv for signature verification")
				// sqv reads an ASCII-armored keyring directly, so reuse the armored public key.
				sqvKeyPath := filepath.Join(artifactsPath, appconfig.DefaultAgentName+".asc")
				if err := l.createPublicKeyFile(sqvKeyPath); err != nil {
					return fmt.Errorf("failed to create armored public key file for sqv: %v", err)
				}
				// sqv takes the keyring via --keyring then the detached signature and the signed file positionally.
				output, err := l.managerHelper.RunCommand("sqv", "--keyring", sqvKeyPath, signaturePath, binaryPath)
				if err != nil {
					if l.managerHelper.IsTimeoutError(err) {
						return fmt.Errorf("sqv command timed out")
					}
					return fmt.Errorf("sqv verify: failed to verify signature using sqv with output '%v' and error: %v", output, err)
				}
				log.Infof("Successfully verified signature using sqv")
				return nil
			}

			// Neither gpg, gpgv, nor sqv is installed; verify the detached signature in-process using the embedded keys.
			log.Infof("gpg, gpgv, and sqv are not available, falling back to in-process signature verification")
			return l.verifySignatureInProcess(log, signaturePath, binaryPath)
		}

		// gpgv requires a dearmored (binary) keyring file
		log.Infof("Using gpgv for signature verification, creating dearmored keyring")
		dearmoredKeyPath := filepath.Join(artifactsPath, appconfig.DefaultAgentName+".gpg.bin")
		if err := l.createDearmoredPublicKeyFile(dearmoredKeyPath); err != nil {
			return fmt.Errorf("failed to create dearmored keyring file: %v", err)
		}

		log.Infof("Using gpgv for signature verification")
		output, err := l.managerHelper.RunCommand("gpgv", "--keyring", dearmoredKeyPath, signaturePath, binaryPath)
		if err != nil {
			if l.managerHelper.IsTimeoutError(err) {
				return fmt.Errorf("gpgv command timed out")
			}
			return fmt.Errorf("gpgv verify: failed to verify signature using gpgv with output '%v' and error: %v", output, err)
		}

		log.Infof("Successfully verified signature using gpgv")
		return nil
	}

	tempKeyRing := "keyring"
	keyringFile := filepath.Join(artifactsPath, tempKeyRing+gpgExtension)

	//create public key file (armored, for gpg --import)
	log.Infof("Creating public key file at: %s", amazonSSMAgentGPGKey)
	if err := l.createPublicKeyFile(amazonSSMAgentGPGKey); err != nil {
		return fmt.Errorf("failed to create amazon-ssm-agent.gpg file: %v", err)
	}

	//create key ring file
	log.Infof("Creating key ring file at: %s", keyringFile)
	if err := l.createKeyRingFile(keyringFile); err != nil {
		return fmt.Errorf("failed to create keyring.gpg file: %v", err)
	}

	log.Debugf("Importing public key: gpg --import %s", amazonSSMAgentGPGKey)
	output, err := l.managerHelper.RunCommand("gpg", "--no-tty", "--batch", "--no-default-keyring", "--keyring", keyringFile, "--import", amazonSSMAgentGPGKey)
	if err != nil {
		if l.managerHelper.IsTimeoutError(err) {
			return fmt.Errorf("gpg command timed out")
		}
	}
	log.Infof("Successfully imported keyring: %v", output)

	sleepFunc(1 * time.Second)

	log.Info("Verifying agent signature")
	output, err = l.managerHelper.RunCommand("gpg", "--no-tty", "--batch", "--no-default-keyring", "--keyring", keyringFile, "--verify", signaturePath, binaryPath)
	if err != nil {
		if l.managerHelper.IsTimeoutError(err) {
			return fmt.Errorf("gpg verify: command timed out")
		}
		return fmt.Errorf("gpg verify: failed to verify signature using gpg with output '%v' and error: %v", output, err)
	}

	log.Infof("Successfully verified signature")
	return nil
}
