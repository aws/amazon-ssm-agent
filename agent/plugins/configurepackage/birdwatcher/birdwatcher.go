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

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher/facade"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/envdetect"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/packageservice"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/amazon-ssm-agent/agent/version"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
)

// PackageService is the concrete type for Birdwatcher PackageService
type PackageService struct {
	facadeClient  facade.BirdwatcherFacade
	manifestCache packageservice.ManifestCache
	collector     envdetect.Collector
}

// New constructor for PackageService
func New(endpoint string, manifestCache packageservice.ManifestCache) packageservice.PackageService {
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

	facadeClientSession := session.New(cfg)

	// Define a request handler with current agentName and version
	SSMAgentVersionUserAgentHandler := request.NamedHandler{
		Name: "ssm.SSMAgentVersionUserAgentHandler",
		Fn:   request.MakeAddToUserAgentHandler(appconfig.DefaultConfig().Agent.Name, version.Version),
	}

	// Add the handler to each request to the BirdwatcherStationService
	facadeClientSession.Handlers.Build.PushBackNamed(SSMAgentVersionUserAgentHandler)

	return &PackageService{
		facadeClient:  ssm.New(facadeClientSession),
		manifestCache: manifestCache,
		collector:     &envdetect.CollectorImp{},
	}
}

// DownloadManifest downloads the manifest for a given version (or latest) and returns the agent version specified in manifest
func (ds *PackageService) DownloadManifest(log log.T, packageName string, version string) (string, error) {
	manifest, err := downloadManifest(ds, packageName, version)
	if err != nil {
		return "", err
	}
	return manifest.Version, nil
}

// DownloadArtifact downloads the platform matching artifact specified in the manifest
func (ds *PackageService) DownloadArtifact(log log.T, packageName string, version string) (string, error) {
	manifest, err := readManifestFromCache(ds.manifestCache, packageName, version)
	if err != nil {
		manifest, err = downloadManifest(ds, packageName, version)
		if err != nil {
			return "", fmt.Errorf("failed to read manifest from file system: %v", err)
		}
	}

	file, err := ds.findFileFromManifest(log, manifest)
	if err != nil {
		return "", err
	}

	return downloadFile(log, file)
}

// ReportResult sents back the result of the install/upgrade/uninstall run back to Birdwatcher
func (ds *PackageService) ReportResult(log log.T, result packageservice.PackageResult) error {
	// TODO: include trace and properties
	// TODO: collect as much as possible data:
	// * AZ, instance id, instance type, platform, version, arch, init system, ...

	env, _ := ds.collector.CollectData(log)

	_, err := ds.facadeClient.PutConfigurePackageResult(
		&ssm.PutConfigurePackageResultInput{
			PackageName:            &result.PackageName,
			PackageVersion:         &result.Version,
			PreviousPackageVersion: &result.PreviousPackageVersion,
			Operation:              &result.Operation,
			OverallTiming:          &result.Timing,
			Result:                 &result.Exitcode,
			PackageResultAttributes: map[string]*string{
				"platformName":     &env.OperatingSystem.Platform,
				"platformVersion":  &env.OperatingSystem.PlatformVersion,
				"architecture":     &env.OperatingSystem.Architecture,
				"instanceID":       &env.Ec2Infrastructure.InstanceID,
				"instanceType":     &env.Ec2Infrastructure.InstanceType,
				"region":           &env.Ec2Infrastructure.Region,
				"availabilityZone": &env.Ec2Infrastructure.AvailabilityZone,
			},
		},
	)

	if err != nil {
		return fmt.Errorf("failed to report results: %v", err)
	}

	return nil
}

// utils
func readManifestFromCache(cache packageservice.ManifestCache, packageName string, version string) (*Manifest, error) {
	data, err := cache.ReadManifest(packageName, version)
	if err != nil {
		return nil, err
	}

	return parseManifest(&data)
}

func downloadManifest(ds *PackageService, packageName string, version string) (*Manifest, error) {
	resp, err := ds.facadeClient.GetManifest(
		&ssm.GetManifestInput{
			PackageName:    &packageName,
			PackageVersion: &version,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve manifest: %v", err)
	}

	byteManifest := []byte(*resp.Manifest)

	manifest, err := parseManifest(&byteManifest)
	if err != nil {
		return nil, err
	}

	err = ds.manifestCache.WriteManifest(packageName, version, byteManifest)
	if err != nil {
		return nil, fmt.Errorf("failed to write manifest to file: %v", err)
	}

	return manifest, nil
}

func parseManifest(data *[]byte) (*Manifest, error) {
	var manifest Manifest

	// TODO: additional validation
	if err := json.NewDecoder(bytes.NewReader(*data)).Decode(&manifest); err != nil {
		return nil, fmt.Errorf("failed to decode manifest: %v", err)
	}

	return &manifest, nil
}

func (ds *PackageService) findFileFromManifest(log log.T, manifest *Manifest) (*File, error) {
	var file *File

	pkginfo, err := ds.extractPackageInfo(log, manifest)
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

func downloadFile(log log.T, file *File) (string, error) {
	downloadInput := artifact.DownloadInput{
		SourceURL: file.DownloadLocation,
		// TODO don't hardcode sha256 - use multiple checksums
		SourceChecksums: file.Checksums,
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
func (ds *PackageService) extractPackageInfo(log log.T, manifest *Manifest) (*PackageInfo, error) {
	env, err := ds.collector.CollectData(log)
	if err != nil {
		return nil, fmt.Errorf("failed to collect data: %v", err)
	}

	if keyplatform, ok := matchPackageSelectorPlatform(env.OperatingSystem.Platform, manifest.Packages); ok {
		if keyversion, ok := matchPackageSelectorVersion(env.OperatingSystem.PlatformVersion, manifest.Packages[keyplatform]); ok {
			if keyarch, ok := matchPackageSelectorArch(env.OperatingSystem.Architecture, manifest.Packages[keyplatform][keyversion]); ok {
				return manifest.Packages[keyplatform][keyversion][keyarch], nil
			}
		}
	}

	return nil, fmt.Errorf("no manifest found for platform: %s, version %s, architecture %s",
		env.OperatingSystem.Platform, env.OperatingSystem.PlatformVersion, env.OperatingSystem.Architecture)
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
