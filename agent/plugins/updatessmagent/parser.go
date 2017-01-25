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

// Package updatessmagent implements the UpdateSsmAgent plugin.
package updatessmagent

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
)

// Manifest represents the json structure of online manifest file.
type Manifest struct {
	SchemaVersion string            `json:"SchemaVersion"`
	URIFormat     string            `json:"UriFormat"`
	Packages      []*PackageContent `json:"Packages"`
}

// PackageContent section in the Manifest json.
type PackageContent struct {
	Name  string         `json:"Name"`
	Files []*FileContent `json:"Files"`
}

// FileContent holds the file name and available versions
type FileContent struct {
	Name              string            `json:"Name"`
	AvailableVersions []*PackageVersion `json:"AvailableVersions"`
}

// PackageVersion section in the PackageContent
type PackageVersion struct {
	Version  string `json:"Version"`
	Checksum string `json:"Checksum"`
}

const (
	minimumVersion = "0"

	// CommonManifestURL is the Manifest URL for regular regions
	CommonManifestURL = "https://s3.{Region}.amazonaws.com/amazon-ssm-{Region}/ssm-agent-manifest.json"

	// ChinaManifestURL is the manifest URL for regions in China
	ChinaManifestURL = "https://s3.{Region}.amazonaws.com.cn/amazon-ssm-{Region}/ssm-agent-manifest.json"
)

// ParseManifest parses the public manifest file to provide agent update information.
func ParseManifest(log log.T,
	fileName string,
	context *updateutil.InstanceContext,
	packageName string) (parsedManifest *Manifest, err error) {
	//Load specified file from file system
	var result = []byte{}
	if result, err = ioutil.ReadFile(fileName); err != nil {
		return
	}
	// parse manifest file
	if err = json.Unmarshal([]byte(result), &parsedManifest); err != nil {
		return
	}

	err = validateManifest(log, parsedManifest, context, packageName)
	return
}

// HasVersion returns if manifest file has particular version for package
func (m *Manifest) HasVersion(context *updateutil.InstanceContext, packageName string, version string) bool {
	for _, p := range m.Packages {
		if p.Name == packageName {
			for _, f := range p.Files {
				if f.Name == context.FileName(packageName) {
					for _, v := range f.AvailableVersions {
						if v.Version == version || version == updateutil.PipelineTestVersion {
							return true
						}
					}
				}
			}
		}
	}

	return false
}

// LatestVersion returns latest version for specific package
func (m *Manifest) LatestVersion(log log.T, context *updateutil.InstanceContext, packageName string) (result string, err error) {
	var version = minimumVersion
	var compareResult = 0
	for _, p := range m.Packages {
		if p.Name == packageName {
			for _, f := range p.Files {
				if f.Name == context.FileName(packageName) {
					for _, v := range f.AvailableVersions {
						if compareResult, err = updateutil.VersionCompare(v.Version, version); err != nil {
							return version, err
						}
						if compareResult > 0 {
							version = v.Version
						}
					}
				}
			}
		}
	}
	if version == minimumVersion {
		log.Debugf("Filename: %v", context.FileName(packageName))
		log.Debugf("Package Name: %v", packageName)
		log.Debugf("Manifest: %v", m)
		return version, fmt.Errorf("cannot find the latest version for package %v", packageName)
	}

	return version, nil
}

// DownloadURLAndHash returns download source url and hash value
func (m *Manifest) DownloadURLAndHash(
	context *updateutil.InstanceContext,
	packageName string,
	version string) (result string, hash string, err error) {
	fileName := context.FileName(packageName)

	for _, p := range m.Packages {
		if p.Name == packageName {
			for _, f := range p.Files {
				if f.Name == fileName {
					for _, v := range f.AvailableVersions {
						if version == v.Version || version == updateutil.PipelineTestVersion {
							result = m.URIFormat
							result = strings.Replace(result, updateutil.RegionHolder, context.Region, -1)
							result = strings.Replace(result, updateutil.PackageNameHolder, packageName, -1)
							result = strings.Replace(result, updateutil.PackageVersionHolder, version, -1)
							result = strings.Replace(result, updateutil.FileNameHolder, f.Name, -1)
							if version == updateutil.PipelineTestVersion {
								return result, "", nil
							}
							return result, v.Checksum, nil
						}
					}
				}
			}
		}
	}

	return "", "", fmt.Errorf("incorrect package name or version, %v, %v", packageName, version)
}

// validateManifest makes sure all the fields are provided.
func validateManifest(log log.T, parsedManifest *Manifest, context *updateutil.InstanceContext, packageName string) error {
	if len(parsedManifest.URIFormat) == 0 {
		return fmt.Errorf("folder format cannot be null in the Manifest file")
	}
	fileName := context.FileName(packageName)
	foundPackage := false
	foundFile := false
	for _, p := range parsedManifest.Packages {
		if p.Name == packageName {
			log.Infof("found package %v", packageName)
			foundPackage = true
			for _, f := range p.Files {
				if f.Name == fileName {
					foundFile = true
					if len(f.AvailableVersions) == 0 {
						return fmt.Errorf("at least one available version is required for the %v", fileName)
					}

					log.Infof("found file %v", fileName)
					break
				}
			}
		}
	}

	if !foundPackage {
		return fmt.Errorf("cannot find the %v information in the Manifest file", packageName)
	}
	if !foundFile {
		return fmt.Errorf("cannot find the %v information in the Manifest file", fileName)
	}

	return nil
}
