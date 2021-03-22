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

// +build windows

// Package updateec2config implements the UpdateEC2Config plugin.
package updateec2config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateinfo"
	"github.com/aws/amazon-ssm-agent/agent/versionutil"
)

// Manifest represents the json structure of online manifest file.
type Manifest struct {
	SchemaVersion string            `json:"SchemaVersion"`
	URIFormat     string            `json:"UriFormat"`
	Packages      []*PackageContent `json:"Packages"`
}

// PackageContent section in the Manifest json.
type PackageContent struct {
	Name              string            `json:"Name"`
	FileName          string            `json:"FileName"`
	AvailableVersions []*PackageVersion `json:"AvailableVersions"`
}

// PackageVersion section in the PackageContent
type PackageVersion struct {
	Version  string `json:"Version"`
	Checksum string `json:"Checksum"`
}

// ParseManifest parses the public manifest file to provide agent update information.
func ParseManifest(log log.T,
	fileName string) (parsedManifest *Manifest, err error) {
	//Load specified file from file system
	var result = []byte{}
	if result, err = ioutil.ReadFile(fileName); err != nil {
		return
	}

	// parse manifest file
	if err = json.Unmarshal([]byte(result), &parsedManifest); err != nil {
		return
	}
	err = validateManifest(log, parsedManifest)
	return
}

// HasVersion returns if manifest file has particular version for package
func (m *Manifest) HasVersion(info updateinfo.T, version string) bool {
	for _, p := range m.Packages {
		if p.Name == EC2UpdaterPackageName && p.FileName == EC2UpdaterFileName {
			for _, v := range p.AvailableVersions {
				if v.Version == version || version == PipelineTestVersion {
					return true
				}
			}
		}
	}

	return false
}

// LatestVersion returns latest version for specific package
func (m *Manifest) LatestVersion(log log.T, info updateinfo.T) (result string, err error) {
	var version = minimumVersion
	var compareResult = 0
	for _, p := range m.Packages {
		if p.Name == EC2UpdaterPackageName && p.FileName == EC2UpdaterFileName {
			for _, v := range p.AvailableVersions {
				if compareResult, err = versionutil.VersionCompare(v.Version, version); err != nil {
					return version, err
				}
				if compareResult > 0 {
					version = v.Version
				}
			}
		}
	}
	if version == minimumVersion {
		log.Debugf("Filename: %v", EC2UpdaterFileName)
		log.Debugf("Package Name: %v", EC2UpdaterPackageName)
		log.Debugf("Manifest: %v", m)
		return version, fmt.Errorf("cannot find the latest version for package %v", EC2UpdaterPackageName)
	}

	return version, nil
}

// DownloadURLAndHash returns download source url and hash value
func (m *Manifest) DownloadURLAndHash(
	packageName string,
	version string, filename string, toformat string, fromformat string, region string) (result string, hash string, err error) {

	for _, p := range m.Packages {
		if p.Name == packageName && p.FileName == filename {
			for _, v := range p.AvailableVersions {
				if version == v.Version || version == PipelineTestVersion {
					m.URIFormat = strings.Replace(m.URIFormat, fromformat, toformat, 1)
					result = m.URIFormat
					result = strings.Replace(result, updateconstants.RegionHolder, region, -1)
					result = strings.Replace(result, updateconstants.PackageNameHolder, packageName, -1)
					result = strings.Replace(result, PackageVersionHolder, version, -1)
					result = strings.Replace(result, updateconstants.FileNameHolder, p.FileName, -1)
					if version == PipelineTestVersion {
						return result, "", nil
					}
					return result, v.Checksum, nil
				}
			}
		}
	}

	return "", "", fmt.Errorf("incorrect package name or version, %v, %v", packageName, version)
}

// validateManifest makes sure all the fields are provided.
func validateManifest(log log.T, parsedManifest *Manifest) error {
	if len(parsedManifest.URIFormat) == 0 {
		return fmt.Errorf("folder format cannot be null in the Manifest file")
	}

	for _, p := range parsedManifest.Packages {

		if p.Name == EC2UpdaterPackageName && p.FileName == EC2UpdaterFileName {
			if len(p.AvailableVersions) == 0 {
				return fmt.Errorf("at least one available version is required for the %v", EC2UpdaterFileName)
			}
			log.Debugf("validate manifest found package %v", EC2UpdaterPackageName)
			log.Debugf("validate manifest found file %v", EC2UpdaterFileName)
			return nil
		}
	}

	return fmt.Errorf("cannot find the %v or %v information in the Manifest file", EC2UpdaterPackageName, EC2UpdaterFileName)

}

// UpdaterFilePath returns updater file path
func UpdaterFilePath(updateRoot string, updaterPackageName string, version string) string {
	return filepath.Join(updateutil.UpdateArtifactFolder(updateRoot, updaterPackageName, version), Updater)
}
