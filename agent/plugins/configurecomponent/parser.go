// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package configurecomponent implements the ConfigureComponent plugin.
package configurecomponent

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"regexp"
	"strconv"

	"github.com/aws/amazon-ssm-agent/agent/log"
)

// ComponentManifest represents json structure of component's online configuration file.
type ComponentManifest struct {
	Name         string `json:"name"`
	Platform     string `json:"platform"`
	Architecture string `json:"architecture"`
	Version      string `json:"version"`
	Install      string `json:"install"`
	Uninstall    string `json:"uninstall"`
	Reboot       string `json:"reboot"`
	Launch       string `json:"launch"`
}

// parseComponentManifest parses the manifest to provide install/uninstall information.
func parseComponentManifest(log log.T, fileName string) (parsedManifest *ComponentManifest, err error) {
	// load specified file from file system
	var result = []byte{}
	if result, err = ioutil.ReadFile(fileName); err != nil {
		if log != nil {
			log.Errorf("Failed to read component's JSON configuration file: %v", err)
		}
		return
	}

	// parse component's JSON configuration file
	if err = json.Unmarshal([]byte(result), &parsedManifest); err != nil {
		if log != nil {
			log.Errorf("Failed to parse component's JSON configuration file: %v", err)
		}
		return
	}

	// ensure manifest conforms to defined schema
	if err = validateComponentManifest(log, parsedManifest); err != nil {
		if log != nil {
			log.Errorf("Invalid JSON configuration file due to %v", err)
		}
	}

	return
}

// TODO:MF: better descriptions of validity requirements when validation fails
// validateManifest ensures all the fields are provided.
func validateComponentManifest(log log.T, parsedManifest *ComponentManifest) error {
	// ensure non-empty struct
	if parsedManifest == (&ComponentManifest{}) {
		return fmt.Errorf("empty component manifest file") //TODO:MF: This isn't triggering when the manifest is empty per coverage.html - but it will get caught in the next validation case - is this necessary?
	}

	// ensure non-empty and properly formatted required fields
	if parsedManifest.Name == "" {
		return fmt.Errorf("empty component name")
	} else {
		name := parsedManifest.Name
		if err := validatePathComponent(log, name); err != nil {
			return fmt.Errorf("invalid component name %v", name)
		}
	}
	if parsedManifest.Version == "" {
		return fmt.Errorf("empty component version")
	} else {
		// ensure version follows format <major>.<minor>.<build>
		version := parsedManifest.Version
		if matched, err := regexp.MatchString(PatternVersion, version); matched == false || err != nil {
			return fmt.Errorf("invalid version string %v", version)
		}
	}
	// TODO:MF: Should we require at least install+uninstall or launch?  Otherwise we just unzip or delete which would work, but seems likely pointless

	// ensure properly formatted optional fields and set defaults
	if parsedManifest.Reboot != "" {
		// ensure reboot is true or false
		if _, err := strconv.ParseBool(parsedManifest.Reboot); err != nil {
			return fmt.Errorf("invalid reboot flag")
		}
	} else {
		parsedManifest.Reboot = "false" //TODO:MF: Can we make this a bool in parsedManifest?
	}
	if parsedManifest.Install != "" {
		install := parsedManifest.Install
		if err := validateCommand(log, install); err != nil {
			return fmt.Errorf("invalid install command string %v", install)
		}
	}
	if parsedManifest.Uninstall != "" {
		uninstall := parsedManifest.Uninstall
		if err := validateCommand(log, uninstall); err != nil {
			return fmt.Errorf("invalid uninstall command string %v", uninstall)
		}
	}
	if parsedManifest.Launch != "" {
		launch := parsedManifest.Launch
		if err := validateCommand(log, launch); err != nil {
			return fmt.Errorf("invalid launch command string %v", launch)
		}
	}
	// TODO:MF: validate platform and arch against this instance's platform and arch?  We don't really use them...

	return nil
}

// validatePathComponent ensures that a given name is a valid part of a folder path or S3 bucket URI
func validatePathComponent(log log.T, name string) error {
	// TODO:MF: Validate
	return nil
}

// validateCommand ensures that a command string is not fundamentally invalid for execution
func validateCommand(log log.T, command string) error {
	// TODO:MF: Validate
	return nil
}
