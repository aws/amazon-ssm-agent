// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package birdwatcher

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/packageservice"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/birdwatcherstationservice"
	"github.com/aws/aws-sdk-go/service/birdwatcherstationservice/birdwatcherstationserviceiface"
)

// PackageService is the concrete type for Birdwatcher PackageService
type PackageService struct {
	bwclient birdwatcherstationserviceiface.BirdwatcherStationServiceAPI
}

// New constructor for PackageService
func New(log log.T, endpoint string) packageservice.PackageService {
	// TODO: endpoint vs appconfig
	cfg := sdkutil.AwsConfig()

	// overrides ssm client config from appconfig if applicable
	if appCfg, err := appconfig.Config(false); err == nil {
		if appCfg.Birdwatcher.Endpoint != "" {
			cfg.Endpoint = &appCfg.Birdwatcher.Endpoint
		}
		if appCfg.Birdwatcher.Region != "" {
			cfg.Region = &appCfg.Birdwatcher.Region
		}
		if appCfg.Birdwatcher.DisableSSL {
			cfg.DisableSSL = &appCfg.Birdwatcher.DisableSSL
		}
	}

	return &PackageService{
		bwclient: birdwatcherstationservice.New(session.New(cfg)),
	}
}

// DownloadManifest downloads the manifest for a given version (or latest) and returns the agent version specified in manifest
func (ds *PackageService) DownloadManifest(log log.T, packageName string, version string, targetDir string) (string, error) {
	var manifest Manifest

	resp, err := ds.bwclient.GetManifest(
		&birdwatcherstationservice.GetManifestInput{
			PackageName:    &packageName,
			PackageVersion: &version,
		},
	)

	if err != nil {
		return "", fmt.Errorf("failed to retrieve manifest: %v", err)
	}

	if err := json.NewDecoder(strings.NewReader(*resp.Manifest)).Decode(&manifest); err != nil {
		return "", fmt.Errorf("failed to decode manifest: %v", err)
	}

	// TODO: sanitize filepath
	dir := filepath.Join(targetDir, packageName, manifest.Version)
	file := filepath.Join(dir, "manifest.json")

	err = filesysdep.MakeDirExecute(dir)
	if err != nil {
		return "", fmt.Errorf("failed to create directory for manifest: %v", err)
	}

	err = filesysdep.WriteFile(file, *resp.Manifest)
	if err != nil {
		return "", fmt.Errorf("failed to write manifest to file: %v", err)
	}

	return manifest.Version, nil
}

// DownloadArtifact downloads the platform matching artifact specified in the manifest
func (*PackageService) DownloadArtifact(log log.T, packageName string, version string, targetDir string) (string, error) {
	file, err := findFileFromManifest(log, packageName, version, targetDir)
	if err != nil {
		return "", err
	}

	return downloadFile(log, file, targetDir)
}

// ReportResult sents back the result of the install/upgrade/uninstall run back to Birdwatcher
func (ds *PackageService) ReportResult(log log.T, result packageservice.PackageResult) error {
	// TODO: include trace and properties
	// TODO: collect as much as possible data:
	// * AZ, instance id, instance type, platform, version, arch, init system, ...
	_, err := ds.bwclient.PutConfigurePackageResult(
		&birdwatcherstationservice.PutConfigurePackageResultInput{
			PackageName:    &result.PackageName,
			PackageVersion: &result.Version,
			OverallTiming:  &result.Timing,
			Result:         &result.Exitcode,
		},
	)

	if err != nil {
		return fmt.Errorf("failed to report results: %v", err)
	}

	return nil
}

// utils
func findFileFromManifest(log log.T, packageName string, version string, targetDir string) (*File, error) {
	var manifest Manifest
	var file *File

	manifestfile := filepath.Join(targetDir, packageName, version, "manifest.json")
	content, err := filesysdep.ReadFile(manifestfile)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest from file system: %v", err)
	}

	if err := json.NewDecoder(bytes.NewReader(content)).Decode(&manifest); err != nil {
		return nil, fmt.Errorf("failed to decode manifest from file system: %v", err)
	}

	pkginfo, err := extractPackageInfo(log, &manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to find platform: %v", err)
	}

	for name, f := range manifest.Files {
		if name == pkginfo.File {
			file = f
			break
		}
	}

	if file == nil {
		return nil, fmt.Errorf("failed to find file for %+v", pkginfo)
	}

	return file, nil
}

func downloadFile(log log.T, file *File, targetDir string) (string, error) {
	downloadInput := artifact.DownloadInput{
		SourceURL:            file.DownloadLocation,
		DestinationDirectory: targetDir,
		// TODO don't hardcode sha256 - use multiple checksums
		SourceHashValue: file.Checksums["sha256"],
		SourceHashType:  "sha256",
	}

	downloadOutput, downloadErr := networkdep.Download(log, downloadInput)
	if downloadErr != nil || downloadOutput.LocalFilePath == "" {
		errMessage := fmt.Sprintf("failed to download installation package reliably, %v", downloadInput.SourceURL)
		if downloadErr != nil {
			errMessage = fmt.Sprintf("%v, %v", errMessage, downloadErr.Error())
		}
		// TODO: attempt to clean up failed download folder?

		// return download error
		return "", errors.New(errMessage)
	}

	return downloadOutput.LocalFilePath, nil
}

// ExtractPackageInfo returns the correct PackageInfo for the current instances platform/version/arch
func extractPackageInfo(log log.T, manifest *Manifest) (*PackageInfo, error) {
	name, err := platformProviderdep.Name(log)
	if err != nil {
		return nil, fmt.Errorf("failed to detect platform: %v", err)
	}

	version, err := platformProviderdep.Version(log)
	if err != nil {
		return nil, fmt.Errorf("failed to detect platform version: %v", err)
	}

	arch, err := platformProviderdep.Architecture(log)
	if err != nil {
		return nil, fmt.Errorf("failed to detect architecture: %v", err)
	}

	if keyplatform, ok := matchPackageSelectorPlatform(name, manifest.Packages); ok {
		if keyversion, ok := matchPackageSelectorVersion(version, manifest.Packages[keyplatform]); ok {
			if keyarch, ok := matchPackageSelectorArch(arch, manifest.Packages[keyplatform][keyversion]); ok {
				return manifest.Packages[keyplatform][keyversion][keyarch], nil
			}
		}
	}

	return nil, fmt.Errorf("no manifest found for platform: %s, version %s, architecture %s", name, version, arch)
}

func matchPackageSelectorPlatform(key string, dict map[string]map[string]map[string]*PackageInfo) (string, bool) {
	if _, ok := dict[key]; ok {
		return key, true
	} else if _, ok := dict["_any"]; ok {
		return "_any", true
	}

	return "", false
}

func matchPackageSelectorVersion(key string, dict map[string]map[string]*PackageInfo) (string, bool) {
	if _, ok := dict[key]; ok {
		return key, true
	} else if _, ok := dict["_any"]; ok {
		return "_any", true
	}

	return "", false
}

func matchPackageSelectorArch(key string, dict map[string]*PackageInfo) (string, bool) {
	if _, ok := dict[key]; ok {
		return key, true
	} else if _, ok := dict["_any"]; ok {
		return "_any", true
	}

	return "", false
}
