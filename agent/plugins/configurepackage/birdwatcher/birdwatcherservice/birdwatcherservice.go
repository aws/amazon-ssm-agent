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

package birdwatcherservice

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher/archive"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher/birdwatcherarchive"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher/documentarchive"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher/facade"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/envdetect"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/packageservice"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/trace"
	"github.com/aws/aws-sdk-go/service/ssm"
)

// NanoTime is helper interface for mocking time
type NanoTime interface {
	NowUnixNano() int64
}

type TimeImpl struct {
}

func (t *TimeImpl) NowUnixNano() int64 {
	return time.Now().UnixNano()
}

// PackageService is the concrete type for Birdwatcher PackageService
type PackageService struct {
	Context        context.T
	pkgSvcName     string
	facadeClient   facade.BirdwatcherFacade
	manifestCache  packageservice.ManifestCache
	collector      envdetect.Collector
	timeProvider   NanoTime
	packageArchive archive.IPackageArchive
}

func NewBirdwatcherArchive(ctx context.T, facadeClient facade.BirdwatcherFacade, manifestCache packageservice.ManifestCache, context map[string]string) packageservice.PackageService {
	pkgArchive := birdwatcherarchive.New(facadeClient, context)
	pkgArchive.SetManifestCache(manifestCache)
	return New(ctx, pkgArchive, facadeClient, manifestCache, packageservice.PackageServiceName_birdwatcher)
}

func NewDocumentArchive(context context.T, facadeClient facade.BirdwatcherFacade, manifestCache packageservice.ManifestCache) packageservice.PackageService {
	pkgArchive := documentarchive.New(facadeClient)
	pkgArchive.SetManifestCache(manifestCache)
	return New(context, pkgArchive, facadeClient, manifestCache, packageservice.PackageServiceName_document)
}

// New constructor for PackageService
func New(context context.T, pkgArchive archive.IPackageArchive, facadeClient facade.BirdwatcherFacade, manifestCache packageservice.ManifestCache, name string) packageservice.PackageService {

	return &PackageService{
		Context:        context,
		pkgSvcName:     name,
		facadeClient:   facadeClient,
		manifestCache:  manifestCache,
		collector:      &envdetect.CollectorImp{},
		timeProvider:   &TimeImpl{},
		packageArchive: pkgArchive,
	}
}

func (ds *PackageService) PackageServiceName() string {
	return ds.pkgSvcName
}

func (ds *PackageService) GetPackageArnAndVersion(packageName string, packageVersion string) (name string, version string) {
	return ds.packageArchive.GetResourceVersion(packageName, packageVersion)
}

// DownloadManifest downloads the manifest for a given version (or latest) and returns the agent version specified in manifest
func (ds *PackageService) DownloadManifest(tracer trace.Tracer, packageName string, version string) (string, string, bool, error) {
	manifest, isSameAsCache, err := downloadManifest(tracer, ds, packageName, version)
	if err != nil {
		return "", "", isSameAsCache, err
	}
	return ds.packageArchive.GetResourceArn(packageName, version), manifest.Version, isSameAsCache, nil
}

// DownloadArtifact downloads the platform matching artifact specified in the manifest
func (ds *PackageService) DownloadArtifact(tracer trace.Tracer, packageName string, version string) (string, error) {
	trace := tracer.BeginSection("download artifact")
	defer trace.End()
	file, getManifestErr := getFileFromManifest(ds, packageName, version, trace, tracer)
	if getManifestErr != nil {
		return "", getManifestErr
	}

	return downloadFile(ds, tracer, file, packageName, version, false)
}

// ReportResult sents back the result of the install/upgrade/uninstall run back to Birdwatcher
func (ds *PackageService) ReportResult(tracer trace.Tracer, result packageservice.PackageResult) error {
	env, _ := ds.collector.CollectData(ds.Context)

	var previousPackageVersion *string
	if result.PreviousPackageVersion != "" {
		previousPackageVersion = &result.PreviousPackageVersion
	}

	var steps []*ssm.ConfigurePackageResultStep
	for _, t := range result.Trace {
		timing := (t.Timing - result.Timing) / 1000000 // converting nano to miliseconds
		steps = append(steps,
			&ssm.ConfigurePackageResultStep{
				Action: &t.Operation,
				Result: &t.Exitcode,
				Timing: &timing,
			})
	}

	overallTiming := (ds.timeProvider.NowUnixNano() - result.Timing) / 1000000

	input := &ssm.PutConfigurePackageResultInput{
		PackageName:            &result.PackageName,
		PackageVersion:         &result.Version,
		PreviousPackageVersion: previousPackageVersion,
		Operation:              &result.Operation,
		OverallTiming:          &overallTiming,
		Result:                 &result.Exitcode,
		Attributes: map[string]*string{
			"platformName":     &env.OperatingSystem.Platform,
			"platformVersion":  &env.OperatingSystem.PlatformVersion,
			"architecture":     &env.OperatingSystem.Architecture,
			"instanceID":       &env.Ec2Infrastructure.InstanceID,
			"instanceType":     &env.Ec2Infrastructure.InstanceType,
			"region":           &env.Ec2Infrastructure.Region,
			"availabilityZone": &env.Ec2Infrastructure.AvailabilityZone,
		},
		Steps: steps,
	}

	_, err := ds.facadeClient.PutConfigurePackageResult(input)

	if err != nil {
		return fmt.Errorf("failed to report results: %v", err)
	}

	return nil
}

//utils
func downloadManifest(tracer trace.Tracer, ds *PackageService, packageName string, version string) (*birdwatcher.Manifest, bool, error) {
	isSameAsCache := false
	if ds == nil {
		return nil, isSameAsCache, fmt.Errorf("PackageService doesn't exist")
	}
	manifest, err := ds.packageArchive.DownloadArchiveInfo(tracer, packageName, version)
	if err != nil {
		return nil, isSameAsCache, fmt.Errorf("failed to download manifest - %v", err)
	}

	byteManifest := []byte(manifest)

	parsedManifest, err := archive.ParseManifest(&byteManifest)
	if err != nil {
		return nil, isSameAsCache, err
	}
	ds.packageArchive.SetResource(packageName, version, parsedManifest)

	cachedManifest, err := ds.packageArchive.ReadManifestFromCache(parsedManifest.PackageArn, parsedManifest.Version)

	if reflect.DeepEqual(parsedManifest, cachedManifest) {
		isSameAsCache = true
	}

	err = ds.packageArchive.WriteManifestToCache(parsedManifest.PackageArn, parsedManifest.Version, byteManifest)
	if err != nil {
		return nil, isSameAsCache, fmt.Errorf("failed to write manifest to file: %v", err)
	}

	return parsedManifest, isSameAsCache, nil
}

func (ds *PackageService) findFileFromManifest(tracer trace.Tracer, manifest *birdwatcher.Manifest) (*archive.File, error) {
	var fileInfo *birdwatcher.FileInfo
	var file archive.File
	var filename string

	pkginfo, err := ds.extractPackageInfo(tracer, manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to find platform: %v", err)
	}

	for name, f := range manifest.Files {
		if name == pkginfo.FileName {
			fileInfo = f
			filename = name
			break
		}
	}

	if fileInfo == nil {
		return nil, fmt.Errorf("failed to find file for %+v", pkginfo)
	}
	file.Info = *fileInfo
	file.Name = filename

	return &file, nil
}

func getFileFromManifest(ds *PackageService, packageName string, version string, trace *trace.Trace, tracer trace.Tracer) (*archive.File, error) {
	manifest, err := ds.packageArchive.ReadManifestFromCache(packageName, version)
	if err != nil {
		trace.AppendInfof("error when reading the manifest from cache %v", err)
		manifest, _, err = downloadManifest(tracer, ds, packageName, version)
		if err != nil {
			trace.WithError(err)
			return nil, fmt.Errorf("failed to download the manifest: %v", err)
		}
	}

	file, err := ds.findFileFromManifest(tracer, manifest)
	if err != nil {
		trace.WithError(err)
		return nil, err
	}
	return file, nil
}

func downloadFile(ds *PackageService, tracer trace.Tracer, file *archive.File, packageName string, version string, isRecursiveRetry bool) (string, error) {
	if ds == nil || ds.packageArchive == nil || file == nil {
		return "", fmt.Errorf("Either package service does not exist or does not have archive information or the file information does not exist")
	}
	sourceUrl, err := ds.packageArchive.GetFileDownloadLocation(file, packageName, version)
	if err != nil {
		return "", err
	}
	downloadInput := artifact.DownloadInput{
		SourceURL:            sourceUrl,
		DestinationDirectory: appconfig.DownloadRoot,
		// TODO don't hardcode sha256 - use multiple checksums
		SourceChecksums: file.Info.Checksums,
	}

	log := tracer.CurrentTrace().Logger
	downloadOutput, downloadErr := birdwatcher.Networkdep.Download(ds.Context, downloadInput)
	if downloadErr != nil || downloadOutput.LocalFilePath == "" {
		errMessage := fmt.Sprintf("failed to download installation package reliably, %v", downloadInput.SourceURL)
		if downloadErr != nil {
			errMessage = fmt.Sprintf("%v, %v", errMessage, downloadErr.Error())
		}

		if isRecursiveRetry {
			// TODO: attempt to clean up failed download folder?
			// return download error
			return "", errors.New(errMessage)
		}

		// Delete cached manifest and retry
		log.Info("There was an error downloading the installation reliably. Deleting the cached manifest and retrying.")
		ds.packageArchive.DeleteCachedManifest(packageName, version)
		downloadManifest(tracer, ds, packageName, version)
		file, getFileError := getFileFromManifest(ds, packageName, version, tracer.CurrentTrace(), tracer)
		if getFileError != nil {
			return "", getFileError
		}

		return downloadFile(ds, tracer, file, packageName, version, true)
	}

	return downloadOutput.LocalFilePath, nil
}

// ExtractPackageInfo returns the correct PackageInfo for the current instances platform/version/arch
func (ds *PackageService) extractPackageInfo(tracer trace.Tracer, manifest *birdwatcher.Manifest) (*birdwatcher.PackageInfo, error) {
	env, err := ds.collector.CollectData(ds.Context)
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

func matchPackageSelectorPlatform(key string, dict map[string]map[string]map[string]*birdwatcher.PackageInfo) (string, bool) {
	if _, ok := dict[key]; ok {
		return key, true
	} else if _, ok := dict["_any"]; ok {
		return "_any", true
	}

	return "", false
}

func matchPackageSelectorVersion(key string, dict map[string]map[string]*birdwatcher.PackageInfo) (string, bool) {
	if _, ok := dict[key]; ok {
		return key, true
	} else {
		// Enumerate each dictionary key to do a prefix match.
		matchedLength := 0
		matchedKey := ""
		for k := range dict {
			temp := k
			// According to /agent/plugins/configurepackage/envdetect/osdetect/windows/osdetect.go, windows nano version will have
			// the 'nano' suffix. Therefore, taking 'nano' out as the first step will allow the version match to fall back to the
			// non-special case scenario.
			if strings.HasSuffix(key, "nano") && strings.HasSuffix(temp, "nano") {
				temp = strings.TrimSuffix(temp, "nano")
			} else if strings.HasSuffix(key, "nano") || strings.HasSuffix(temp, "nano") {
				continue
			}

			// If there's no wild card, there's no match.
			if !strings.HasSuffix(temp, ".*") {
				continue
			}

			// Then handle the wild card scenario
			temp = strings.TrimSuffix(temp, ".*")

			tempArray := strings.Split(temp, ".")
			keyArray := strings.Split(key, ".")
			i := 0
			matched := true
			for ; i < len(tempArray) && i < len(keyArray); i++ {
				if tempArray[i] != keyArray[i] {
					matched = false
					break
				}
			}

			// The more specific match will be preferred.
			if matched && i > matchedLength && i == len(tempArray) {
				matchedLength = i
				matchedKey = k
			}
		}

		if matchedKey != "" {
			return matchedKey, true
		}
	}

	// Evaluate _any and * last as more specific version takes precedence.
	if _, ok := dict["_any"]; ok {
		return "_any", true
	}

	if _, ok := dict["*"]; ok {
		return "*", true
	}

	return "", false
}

func matchPackageSelectorArch(key string, dict map[string]*birdwatcher.PackageInfo) (string, bool) {
	if _, ok := dict[key]; ok {
		return key, true
	} else if _, ok := dict["_any"]; ok {
		return "_any", true
	}

	return "", false
}
