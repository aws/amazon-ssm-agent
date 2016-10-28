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

// Package configurepackage implements the ConfigurePackage plugin.
package configurepackage

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/aws/amazon-ssm-agent/agent/log"
)

// PackageManifest represents json structure of package's online configuration file.
type PackageManifest struct {
	Name         string `json:"name"`
	Platform     string `json:"platform"`
	Architecture string `json:"architecture"`
	Version      string `json:"version"`
}

// parsePackageManifest parses the manifest to provide install/uninstall information.
func parsePackageManifest(log log.T, fileName string) (parsedManifest *PackageManifest, err error) {
	// load specified file from file system
	var result = []byte{}
	if result, err = filesysdep.ReadFile(fileName); err != nil {
		if log != nil {
			log.Errorf("Failed to read package's JSON configuration file: %v", err)
		}
		return
	}

	// parse package's JSON configuration file
	if err = json.Unmarshal([]byte(result), &parsedManifest); err != nil {
		if log != nil {
			log.Errorf("Failed to parse package's JSON configuration file: %v", err)
		}
		return
	}

	// ensure manifest conforms to defined schema
	if err = validatePackageManifest(log, parsedManifest); err != nil {
		if log != nil {
			log.Errorf("Invalid JSON configuration file due to %v", err)
		}
	}

	return
}

// TODO:MF: better descriptions of validity requirements when validation fails
// validateManifest ensures all the fields are provided.
func validatePackageManifest(log log.T, parsedManifest *PackageManifest) error {
	// ensure non-empty struct
	if parsedManifest == (&PackageManifest{}) {
		return fmt.Errorf("empty package manifest file") //TODO:MF: This isn't triggering when the manifest is empty per coverage.html - but it will get caught in the next validation case - is this necessary?
	}

	// ensure non-empty and properly formatted required fields
	if parsedManifest.Name == "" {
		return fmt.Errorf("empty package name")
	} else {
		name := parsedManifest.Name
		if err := validatePathPackage(log, name); err != nil {
			return fmt.Errorf("invalid package name %v", name)
		}
	}
	if parsedManifest.Version == "" {
		return fmt.Errorf("empty package version")
	} else {
		// ensure version follows format <major>.<minor>.<build>
		version := parsedManifest.Version
		if matched, err := regexp.MatchString(PatternVersion, version); matched == false || err != nil {
			return fmt.Errorf("invalid version string %v", version)
		}
	}
	// TODO:MF: validate platform and arch against this instance's platform and arch?  We don't really use them...

	return nil
}

// validatePathPackage ensures that a given name is a valid part of a folder path or S3 bucket URI
func validatePathPackage(log log.T, name string) error {
	// TODO:MF: Validate
	return nil
}
