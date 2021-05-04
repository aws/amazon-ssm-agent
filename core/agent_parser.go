// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package main represents the entry point of the agent.
// Parser contains logic for commandline handling flags
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/managedInstances/fingerprint"
	"github.com/aws/amazon-ssm-agent/agent/managedInstances/registration"
	"github.com/aws/amazon-ssm-agent/agent/ssm/anonauth"
	"github.com/aws/amazon-ssm-agent/agent/version"
)

// parseFlags displays flags and handles them
func parseFlags() {
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flag.Usage = flagUsage

	// managed instance registration
	flag.BoolVar(&register, registerFlag, false, "")
	flag.BoolVar(&disableSimilarityCheck, disableSimilarityCheckFlag, false, "")
	flag.StringVar(&activationCode, activationCodeFlag, "", "")
	flag.StringVar(&activationID, activationIDFlag, "", "")
	flag.StringVar(&region, regionFlag, "", "")
	flag.BoolVar(&agentVersionFlag, versionFlag, false, "")

	// clear registration
	flag.BoolVar(&clear, "clear", false, "")

	// fingerprint similarity threshold
	flag.BoolVar(&fpFlag, fingerprintFlag, false, "")
	flag.IntVar(&similarityThreshold, similarityThresholdFlag, 40, "")

	// force flag
	flag.BoolVar(&force, "y", false, "")

	flag.Parse()
}

// handles registration and fingerprint flags
func handleRegistrationAndFingerprintFlags(log logger.T) {
	if flag.NFlag() > 0 {
		exitCode := 1
		if register {
			exitCode = processRegistration(log)
		} else if fpFlag {
			exitCode = processFingerprint(log)
		} else {
			flagUsage()
		}
		log.Flush()
		log.Close()
		os.Exit(exitCode)
	}
}

// handles agent version flag.
// This function is without logger and will not print extra statements
func handleAgentVersionFlag() {
	if flag.NFlag() == 1 {
		if agentVersionFlag {
			fmt.Println("SSM Agent version: " + version.Version)
			os.Exit(0)
		}
	}
}

// flagUsage displays a command-line friendly usage message
func flagUsage() {
	fmt.Fprintln(os.Stderr, "\n\nCommand-line Usage:")
	fmt.Fprintln(os.Stderr, "\t-register\tregister managed instance")
	fmt.Fprintln(os.Stderr, "\t\t-id                    \tSSM activation ID                                                                          \t(REQUIRED)")
	fmt.Fprintln(os.Stderr, "\t\t-code                  \tSSM activation code                                                                        \t(REQUIRED)")
	fmt.Fprintln(os.Stderr, "\t\t-region                \tSSM region                                                                                 \t(REQUIRED)")
	fmt.Fprintln(os.Stderr, "\t\t-disableSimilarityCheck\tDisable the agent hardware/fingerprint similarity check (similarity threshold is set to -1)\t(OPTIONAL)")
	fmt.Fprintln(os.Stderr, "\n\t\t-clear\tClears the previously saved SSM registration")
	fmt.Fprintln(os.Stderr, "\t-fingerprint\tWhether to update the machine fingerprint similarity threshold\t(OPTIONAL)")
	fmt.Fprintln(os.Stderr, "\t\t-similarityThreshold\tThe new required percentage of matching hardware values (-1 disables hardware check)\t(OPTIONAL)")
	fmt.Fprintln(os.Stderr, "\n\t-y\tAnswer yes for all questions")
}

// processRegistration handles flags related to the registration category
func processRegistration(log logger.T) (exitCode int) {
	if activationCode == "" || activationID == "" || region == "" {
		// clear registration
		if clear {
			fingerprint.ClearStoredHardwareInfo(log)
			return clearRegistration(log)
		}
		flagUsage()
		return 1
	}

	// check if previously registered
	if !force && registration.InstanceID(log) != "" {
		confirmation, err := askForConfirmation()
		if err != nil {
			log.Errorf("Registration failed due to %v", err)
			return 1
		}

		if !confirmation {
			log.Info("Registration canceled by user")
			return 1
		}
	}

	managedInstanceID, err := registerManagedInstance(log)
	if err != nil {
		log.Errorf("Registration failed due to %v", err)
		return 1
	}

	log.Infof("Successfully registered the instance with AWS SSM using Managed instance-id: %s", managedInstanceID)
	return 0
}

// processFingerprint handles flags related to the fingerprint category
func processFingerprint(log logger.T) (exitCode int) {
	if err := fingerprint.SetSimilarityThreshold(log, similarityThreshold); err != nil {
		log.Errorf("Error setting the SimilarityThreshold. %v", err)
		return 1
	}
	log.Infof("Fingerprint SimilarityThreshold set to %v", similarityThreshold)
	return 0
}

// registerManagedInstance checks for activation credentials and performs managed instance registration when present
func registerManagedInstance(log logger.T) (managedInstanceID string, err error) {
	// try to activate the instance with the activation credentials
	publicKey, privateKey, keyType, err := registration.GenerateKeyPair()
	if err != nil {
		return "", fmt.Errorf("error generating signing keys. %v", err)
	}

	// checking write access before registering
	err = registration.UpdateServerInfo("", "", privateKey, keyType)
	if err != nil {
		return "",
			fmt.Errorf("Unable to save registration information. %v\nTry running as sudo/administrator.", err)
	}

	// generate fingerprint
	fingerprintUUID, err := registration.Fingerprint(log)
	if err != nil {
		return "", fmt.Errorf("error generating instance fingerprint. %v", err)
	}

	// set similarity threshold
	if disableSimilarityCheck {
		log.Debugf("disableSimilarityCheck is set to true, setting similarity threshold to -1")
		if err = fingerprint.SetSimilarityThreshold(log, -1); err != nil {
			fingerprint.ClearStoredHardwareInfo(log)
			clearRegistration(log)
			return "", fmt.Errorf("failed to set SimilarityThreshold: %v", err)
		}
	}

	service := anonauth.NewAnonymousService(log, region)
	managedInstanceID, err = service.RegisterManagedInstance(
		activationCode,
		activationID,
		publicKey,
		keyType,
		fingerprintUUID,
	)

	if err != nil {
		return managedInstanceID, fmt.Errorf("error registering the instance with AWS SSM. %v", err)
	}

	err = registration.UpdateServerInfo(managedInstanceID, region, privateKey, keyType)
	if err != nil {
		return managedInstanceID, fmt.Errorf("error persisting the instance registration information. %v", err)
	}

	// saving registration information to the registration file
	reg := map[string]string{
		"ManagedInstanceID": managedInstanceID,
		"Region":            region,
	}

	var regData []byte
	if regData, err = json.Marshal(reg); err != nil {
		return "", fmt.Errorf("Failed to marshal registration info. %v", err)
	}

	if err = ioutil.WriteFile(registrationFile, regData, appconfig.ReadWriteAccess); err != nil {
		return "", fmt.Errorf("Failed to write registration info to file. %v", err)
	}

	return managedInstanceID, nil
}

// clearRegistration clears any existing registration data
func clearRegistration(log logger.T) (exitCode int) {
	err := registration.UpdateServerInfo("", "", "", "")
	if err == nil {
		log.Info("Registration information has been removed from the instance.")
		return 0
	}
	log.Errorf("error clearing the instance registration information. %v\nTry running as sudo/administrator.", err)
	return 1
}

// askForConfirmation will ask user for confirmation if they want to proceed.
func askForConfirmation() (result bool, err error) {
	var response string
	fmt.Print("\nInstance already registered. Would you like to override existing with new registration information? [Yes/No]: ")
	_, err = fmt.Scanln(&response)
	if err != nil {
		return false, err
	}

	if len(response) > 0 {
		firstChar := strings.ToLower(string(response[0]))
		if firstChar == "y" {
			return true, nil
		}
		if firstChar == "n" {
			return false, nil
		}
	}
	return false, fmt.Errorf("Invalid response received.")
}
