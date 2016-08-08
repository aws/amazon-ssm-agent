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

	"github.com/aws/amazon-ssm-agent/agent/log"
)

const (
	RebootTrue  = "true"
	RebootFalse = "false"
)

// ComponentManifest represents json structure of component's online configuration file.
type ComponentManifest struct {
	Name      string `json:"Name"`
	Version   string `json:"Version"`
	Install   string `json:"Install"`
	Uninstall string `json:"Uninstall"`
	Reboot    string `json:"Reboot"`
}

// ParseComponentManifest parses the manifest to provide install/uninstall information.
func ParseComponentManifest(log log.T, fileName string) (parsedManifest *ComponentManifest, err error) {
	// load specified file from file system
	var result = []byte{}
	if result, err = ioutil.ReadFile(fileName); err != nil {
		log.Errorf("Failed to read component's JSON configuration file: %v", err)
		return
	}

	// parse component's JSON configuration file
	if err = json.Unmarshal([]byte(result), &parsedManifest); err != nil {
		log.Errorf("Failed to parse component's JSON configuration file: %v", err)
		return
	}

	// ensure manifest confirms to defined schema
	if err = validateComponentManifest(log, parsedManifest); err != nil {
		log.Errorf("Invalid JSON configuration file due to %v", err)
	}

	return
}

// validateManifest ensures all the fields are provided.
func validateComponentManifest(log log.T, parsedManifest *ComponentManifest) error {
	// ensure non-empty struct
	if parsedManifest == (&ComponentManifest{}) {
		return fmt.Errorf("empty component manifest file")
	}

	// ensure non-empty fields
	if parsedManifest.Name == "" {
		return fmt.Errorf("empty component name")
	} else if parsedManifest.Version == "" {
		return fmt.Errorf("empty version")
	} else if parsedManifest.Install == "" {
		return fmt.Errorf("empty install command")
	} else if parsedManifest.Uninstall == "" {
		return fmt.Errorf("empty uninstall command")
	} else if parsedManifest.Reboot == "" {
		return fmt.Errorf("empty reboot flag")
	}

	// ensure version follows format <major>.<minor>.<build>.<release>
	// validate if string is a valid version string
	version := parsedManifest.Version
	if matched, err := regexp.MatchString("\\d+(\\.\\d+)?", version); matched == false || err != nil {
		return fmt.Errorf("invalid version string %v", version)
	}

	// ensure reboot is true or false
	rebootFlag := parsedManifest.Reboot
	if (rebootFlag != RebootTrue) && (rebootFlag != RebootFalse) {
		return fmt.Errorf("invalid reboot flag")
	}

	return nil
}
