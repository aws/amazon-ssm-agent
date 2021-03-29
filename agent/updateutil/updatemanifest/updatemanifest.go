// Copyright 2021 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package updatemanifest implements the logic for the ssm agent s3 manifest.
package updatemanifest

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateinfo"
	"github.com/aws/amazon-ssm-agent/agent/versionutil"
)

func New(context context.T, info updateinfo.T) T {
	return &manifestImpl{
		context:  context,
		info:     info,
		manifest: &jsonManifest{},
	}
}

func (m *manifestImpl) LoadManifest(manifestPath string) error {
	var manifestBytes []byte
	var err error
	if manifestBytes, err = ioutil.ReadFile(manifestPath); err != nil {
		return err
	}

	err = json.Unmarshal(manifestBytes, m.manifest)

	if err != nil {
		return err
	}

	if err = m.validateManifest(appconfig.DefaultAgentName); err != nil {
		return err
	}

	return nil
}

// HasVersion returns if manifest file has particular version for package
func (m *manifestImpl) HasVersion(packageName string, version string) bool {
	log := m.context.Log()
	log.Debugf("checking if package %v has version %s", packageName, version)
	fileName := m.info.GenerateCompressedFileName(packageName)
	log.Debugf("Searching for file name %s", fileName)
	for _, p := range m.manifest.Packages {
		if p.Name == packageName {
			log.Debugf("Found package %s", packageName)
			for _, f := range p.Files {
				if f.Name == fileName {
					log.Debugf("Found file name %s", fileName)
					for _, v := range f.AvailableVersions {
						if v.Version == version || version == updateconstants.PipelineTestVersion {
							log.Debugf("Found version %s", version)
							return true
						}
					}

				}
			}
		}
	}
	log.Warnf("Did not find file name %s with version %s for package %s", fileName, version, packageName)
	return false
}

// GetLatestVersion returns latest version for specific package
func (m *manifestImpl) GetLatestVersion(packageName string) (result string, err error) {
	var version = updateconstants.MinimumVersion
	var compareResult = 0
	for _, p := range m.manifest.Packages {
		if p.Name == packageName {
			for _, f := range p.Files {
				if f.Name == m.info.GenerateCompressedFileName(packageName) {
					for _, v := range f.AvailableVersions {
						if compareResult, err = versionutil.VersionCompare(v.Version, version); err != nil {
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
	if version == updateconstants.MinimumVersion {
		log := m.context.Log()
		log.Debugf("Filename: %v", m.info.GenerateCompressedFileName(packageName))
		log.Debugf("Package Name: %v", packageName)
		log.Debugf("Manifest: %v", m)
		return version, fmt.Errorf("cannot find the latest version for package %v", packageName)
	}

	return version, nil
}

// GetLatestActiveVersion returns latest active version for specific package
func (m *manifestImpl) GetLatestActiveVersion(packageName string) (result string, err error) {
	var version = updateconstants.MinimumVersion
	var compareResult = 0
	for _, p := range m.manifest.Packages {
		if p.Name == packageName {
			for _, f := range p.Files {
				if f.Name == m.info.GenerateCompressedFileName(packageName) {
					for _, v := range f.AvailableVersions {
						actualStatus, verErr := m.getVersionStatus(v)

						if verErr != nil {
							// This can only happen when agent doesn't understand the 3 supported statuses
							m.context.Log().Warnf("Failed to get version status: %v", verErr)
							continue
						}

						if actualStatus != VersionStatusActive {
							continue
						}

						if compareResult, err = versionutil.VersionCompare(v.Version, version); err != nil {
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
	if version == updateconstants.MinimumVersion {
		log := m.context.Log()
		log.Debugf("Filename: %v", m.info.GenerateCompressedFileName(packageName))
		log.Debugf("Package Name: %v", packageName)
		log.Debugf("Manifest: %v", m)
		return version, fmt.Errorf("cannot find the latest version for package %v", packageName)
	}

	return version, nil
}

// GetDownloadURLAndHash returns download source url and hash value
func (m *manifestImpl) GetDownloadURLAndHash(
	packageName string,
	version string) (result string, hash string, err error) {
	fileName := m.info.GenerateCompressedFileName(packageName)
	var region string
	region, err = m.context.Identity().Region()

	if err != nil {
		return
	}

	for _, p := range m.manifest.Packages {
		if p.Name == packageName {
			for _, f := range p.Files {
				if f.Name == fileName {
					for _, v := range f.AvailableVersions {
						if version == v.Version || version == updateconstants.PipelineTestVersion {
							result = m.manifest.URIFormat
							result = strings.Replace(result, updateconstants.RegionHolder, region, -1)
							result = strings.Replace(result, updateconstants.PackageNameHolder, packageName, -1)
							result = strings.Replace(result, updateconstants.PackageVersionHolder, version, -1)
							result = strings.Replace(result, updateconstants.FileNameHolder, f.Name, -1)
							if version == updateconstants.PipelineTestVersion {
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

func (m *manifestImpl) getVersionStatus(version *packageVersion) (string, error) {
	switch version.Status {
	case "":
		return VersionStatusActive, nil
	case VersionStatusDeprecated, VersionStatusActive, VersionStatusInactive:
		return version.Status, nil
	default:
		return "", fmt.Errorf("invalid status %s for version %s", version.Status, version.Version)
	}
}

func (m *manifestImpl) isVersionStatus(packageName string, version string, status string) (isValid bool, err error) {
	log := m.context.Log()
	fileName := m.info.GenerateCompressedFileName(packageName)
	for _, p := range m.manifest.Packages {
		if p.Name == packageName {
			log.Infof("found package %v", packageName)
			for _, f := range p.Files {
				if f.Name == fileName {
					for _, v := range f.AvailableVersions {
						if v.Version == version {
							var actualStatus string
							actualStatus, err = m.getVersionStatus(v)

							if err != nil {
								return
							}

							log.Infof("Version %v status in manifest is %v while checking if status is %v", version, actualStatus, status)
							return actualStatus == status, nil
						}
					}
					break
				}
			}
		}
	}

	return isValid, fmt.Errorf("cannot find  %v information for %v in Manifest file", fileName, version)
}

func (m *manifestImpl) IsVersionDeprecated(
	packageName string,
	version string) (isValid bool, err error) {
	return m.isVersionStatus(packageName, version, VersionStatusDeprecated)
}

func (m *manifestImpl) IsVersionActive(
	packageName string,
	version string) (isValid bool, err error) {
	return m.isVersionStatus(packageName, version, VersionStatusActive)
}

// validateManifest makes sure all the fields are provided.
func (m *manifestImpl) validateManifest(packageName string) error {
	log := m.context.Log()
	if len(m.manifest.URIFormat) == 0 {
		return fmt.Errorf("uri format cannot be null in the Manifest file")
	}
	fileName := m.info.GenerateCompressedFileName(packageName)
	foundPackage := false
	foundFile := false
	for _, p := range m.manifest.Packages {
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
