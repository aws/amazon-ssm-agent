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

// Package rundaemon implements rundaemon plugin and its configuration
package rundaemon

import (
	"errors"
	"regexp"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
)

// ConfigureDaemonPluginInput represents an action to run a package as a daemon.
type ConfigureDaemonPluginInput struct {
	contracts.PluginInput
	Name            string `json:"name"`
	Action          string `json:"action"`
	PackageLocation string `json:"packagelocation"`
	Command         string `json:"command"`
}

// ValidateDaemonInput validates the input given to configure daemon
func ValidateDaemonInput(input ConfigureDaemonPluginInput) error {
	if input.Name == "" {
		return errors.New("daemon name is missing")
	}
	// Prevent names that would not be valid as file names
	validNameValue := regexp.MustCompile(`^[a-zA-Z_]+(([-.])?[a-zA-Z0-9_]+)*$`)
	if !validNameValue.MatchString(input.Name) {
		return errors.New("Invalid daemon name, must start with letter or _; end with letter, number, or _; and contain only letters, numbers, -, _, or single . characters")
	}
	if input.PackageLocation == "" {
		return errors.New("daemon location is missing")
	}
	if !fileutil.Exists(input.PackageLocation) {
		return errors.New("daemon location does not exist")
	}
	if input.Action == "Start" && input.Command == "" {
		return errors.New("daemon launch command is missing")
	}
	return nil
}
