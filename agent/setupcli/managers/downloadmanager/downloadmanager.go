// Copyright 2023 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package downloadmanager helps us with file download related functions in ssm-setup-cli
package downloadmanager

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/backoffconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/network"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/utility"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateinfo"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updatemanifest"
	"github.com/aws/amazon-ssm-agent/common/identity/endpoint"
	"github.com/cenkalti/backoff/v4"
)

const (
	manifestJsonFileName             = "ssm-agent-manifest.json"
	s3Service                        = "s3"
	lowerKernelVersionSupportedAgent = "3.0.1479.0"
	testVersion                      = "255.255.65535.999"
)

var (
	utilHttpDownload          = utility.HttpDownload
	updateInfoNew             = updateinfo.New
	updateManifestNew         = updatemanifest.New
	fileUtilUnCompress        = fileutil.Uncompress
	fileUtilityReadContent    = utility.HttpReadContent
	backOffRetry              = backoff.Retry
	computeAgentChecksumFunc  = utility.ComputeCheckSum
	hasLowerKernelVersionFunc = hasLowerKernelVersion
)

type downloadManager struct {
	log           log.T
	ctx           context.T
	bucketUrl     string
	region        string
	version       string
	manifestURL   string
	updateInfo    updateinfo.T
	manifestInfo  updatemanifest.T
	isNano        bool
	artifactsPath string
}

// New returns a new instance of DownloadManager
func New(log log.T, region string, manifestURL string, updateInfo updateinfo.T, setupCLIArtifactsPath string, isNano bool) IDownloadManager {
	downloadMgrLog := log.WithContext("[DownloadManager]")
	var err error
	ctx := context.Default(downloadMgrLog, appconfig.DefaultConfig(), nil)
	if updateInfo == nil {
		updateInfo, err = updateinfo.New(ctx)
		if err != nil {
			downloadMgrLog.Errorf("Error while initiating update Info: %v", err)
			return nil
		}
	}
	endpointHelper := endpoint.NewEndpointHelper(log, appconfig.SsmagentConfig{})
	if endpointHelper == nil {
		downloadMgrLog.Errorf("Error while initiating endpoint helper: %v", err)
		return nil
	}
	s3Endpoint := endpointHelper.GetServiceEndpoint(s3Service, region)
	downloadManagerRef := &downloadManager{
		log:           downloadMgrLog,
		ctx:           ctx,
		region:        region,
		updateInfo:    updateInfo,
		bucketUrl:     s3Endpoint,
		manifestURL:   manifestURL, // field is optional
		isNano:        isNano,
		artifactsPath: setupCLIArtifactsPath,
	}
	err = downloadManagerRef.Init()
	if err != nil {
		downloadMgrLog.Errorf("initialization failed: %v", err)
		return nil
	}
	return downloadManagerRef
}

func (d *downloadManager) Init() error {
	s3Url := d.getRegionManifestUrl()

	// downloads manifest based on the URL retrieved above and stores it in local path
	manifestFilePath, err := utilHttpDownload(d.log, s3Url, d.artifactsPath)
	if err != nil || manifestFilePath == "" {
		return fmt.Errorf("error while downloading manifest: %v", err)
	}

	updateManifestObj := updateManifestNew(d.ctx, d.updateInfo, d.region)
	d.manifestInfo = updateManifestObj

	err = updateManifestObj.LoadManifest(manifestFilePath)
	if err != nil {
		return fmt.Errorf("error while loading manifest: %v", err)
	}
	return nil
}

// DownloadArtifacts downloads agent artifacts from S3 bucket
func (d *downloadManager) DownloadArtifacts(installVersion string, manifestUrl string, artifactsStorePath string) error {
	var agentDownloadURL, agentHashInManifest string

	logger := d.log

	var err error
	// generate agent artifacts URL and checksum using the manifest loaded
	if agentDownloadURL, agentHashInManifest, err = d.manifestInfo.GetDownloadURLAndHash(appconfig.DefaultAgentName, installVersion); err != nil {
		return fmt.Errorf("error while getting target location and target hash: %v", err)
	}

	generatedUrl := d.getS3BucketUrl() + "/"
	generatedUrl += appconfig.DefaultAgentName + "/" + installVersion + "/" + d.updateInfo.GenerateCompressedFileName(appconfig.DefaultAgentName)
	if generatedUrl != agentDownloadURL {
		d.log.Warnf("URL does not match %v %v", generatedUrl, agentDownloadURL)
	}

	// download agent artifacts using the generated URL before and store in local path
	agentSetupFilePath, err := utilHttpDownload(logger, generatedUrl, artifactsStorePath)
	if err != nil || agentSetupFilePath == "" {
		return fmt.Errorf("error while downloading agent artifacts file: %v", err)
	}

	// compute checksum of downloaded binary
	agentCheckSum, err := computeAgentChecksumFunc(agentSetupFilePath)
	if err != nil {
		return fmt.Errorf("failed to fetch checksum: %v", err)
	}

	// validate checksum using manifest
	if agentCheckSum != agentHashInManifest {
		return fmt.Errorf("checksum validation failed: %v", err)
	}

	// Un-compress downloaded files
	err = d.fileUnCompress(logger, agentSetupFilePath, artifactsStorePath)

	return err
}

// DownloadLatestSSMSetupCLI downloads latest SSM Setup CLI
func (d *downloadManager) DownloadLatestSSMSetupCLI(artifactsStorePath string, expectedSetupCLICheckSum string) error {
	logger := d.log

	logger.Info("Downloading SSM Setup CLI")
	folderName := d.updateInfo.GeneratePlatformBasedFolderName()

	// generate ssm-setup-cli s3 url
	ssmSetupCLIS3URL, err := d.generateLatestSSMSetupCLIS3Url(folderName)
	if err != nil {
		return fmt.Errorf("error while generating SSM Setup CLI URL: %v", err)
	}

	// Download ssm-setup CLI
	downloadedSSMSetupCLIFilePath, err := utilHttpDownload(logger, ssmSetupCLIS3URL, artifactsStorePath)
	if err != nil || downloadedSSMSetupCLIFilePath == "" {
		return fmt.Errorf("error while downloading SSM Setup CLI: %v", err)
	}

	// compute checksum of downloaded binary
	downloadedCLICheckSum, err := computeAgentChecksumFunc(downloadedSSMSetupCLIFilePath)
	if err != nil {
		return fmt.Errorf("failed to fetch checksum: %v", err)
	}

	if downloadedCLICheckSum != expectedSetupCLICheckSum {
		return fmt.Errorf("checksum validation for ssm-setup-cli fail: %v", err)
	}

	logger.Infof("Downloaded SSM-Setup-CLI successfully")
	return nil
}

// GetStableVersion downloads the stable version file and returns the stable version number
func (d *downloadManager) GetStableVersion() (string, error) {
	stableVersionURL, err := d.getStableVersionURL()
	if err != nil {
		return "", fmt.Errorf("error while generating stable version URL %v", err)
	}
	stableVersion, err := d.readVersionFromURL(stableVersionURL)
	return stableVersion, err
}

// GetLatestVersion downloads the latest version file and returns the latest version number
func (d *downloadManager) GetLatestVersion() (string, error) {
	if hasLowerKernelVersionFunc() {
		return lowerKernelVersionSupportedAgent, nil
	}
	latestVersion, err := d.manifestInfo.GetLatestActiveVersion(appconfig.DefaultAgentName)
	if err != nil {
		return "", fmt.Errorf("error while getting the latest version from manifest: %v", err)
	}
	if latestVersion == testVersion {
		latestVersionURL, err := d.getLatestVersionURL()
		if err != nil {
			return "", fmt.Errorf("error while generating latest version URL %v", err)
		}
		latestVersion, err := d.readVersionFromURL(latestVersionURL)
		return latestVersion, err
	}
	return latestVersion, err
}

// getLatestVersionURL gets the latest version from URL
func (d *downloadManager) getLatestVersionURL() (string, error) {
	latestVersionURL := fmt.Sprintf("%s/%s/%s", d.getS3BucketUrl(), utility.LatestVersionString, utility.VersionFile)
	s3URL, err := url.Parse(latestVersionURL)
	if err != nil {
		return "", fmt.Errorf("error while parsing s3URL: %v", err)
	}
	return s3URL.String(), nil
}

func (d *downloadManager) readVersionFromURL(versionURL string) (string, error) {
	var err error
	d.log.Infof("Retrieving version from: %s", versionURL)
	exponentialBackOff, err := backoffconfig.GetDefaultExponentialBackoff()
	if err != nil {
		return "", fmt.Errorf("failed to initialize backoff module: %v", err)
	}

	var content string
	err = backOffRetry(func() error {
		httpTimeout := 15 * time.Second
		tr := network.GetDefaultTransport(d.log, appconfig.DefaultConfig())
		client := &http.Client{
			Transport: tr,
			Timeout:   httpTimeout,
		}
		// use http client to download
		contentBytes, readErr := fileUtilityReadContent(versionURL, client)
		if readErr != nil {
			return fmt.Errorf("failed to read response from %s: %v", versionURL, readErr)
		}
		if contentBytes == nil {
			return fmt.Errorf("response code is nil")
		}
		content = string(contentBytes)
		return nil
	}, exponentialBackOff)

	if err != nil {
		return "", fmt.Errorf("failed to get version from %s: %v", versionURL, err)
	}
	version := strings.TrimSpace(content)
	if !regexp.MustCompile(`^\d+.\d+.\d+.\d+$`).Match([]byte(version)) {
		return "", fmt.Errorf("invalid version format returned from %s: %s", versionURL, version)
	}

	d.log.Infof("Got version from %v: %s", versionURL, version)
	return version, nil
}

func (d *downloadManager) generateLatestSSMSetupCLIS3Url(folderName string) (string, error) {
	s3BucketUrl := d.getS3BucketUrl()
	ssmSetupCLIURL := fmt.Sprintf("%s/%s/%s/%s", s3BucketUrl, utility.LatestVersionString, folderName, utility.SSMSetupCLIBinary)
	s3URL, err := url.Parse(ssmSetupCLIURL)
	if err != nil {
		return "", fmt.Errorf("error while parsing s3URL")
	}
	return s3URL.String(), nil
}

// getS3BucketUrl returns s3 bucket URL
func (d *downloadManager) getS3BucketUrl() string {
	httpsPrefix := "https://"
	if d.manifestURL != "" {
		url := strings.TrimSpace(d.manifestURL)
		url = strings.TrimRight(url, updateconstants.ManifestFile)
		url = strings.TrimSuffix(url, "/")
		url = strings.TrimSuffix(url, "\\")
		return url
	}
	bucketName := strings.TrimSuffix(updateconstants.BucketPath, "/")
	return strings.Replace(httpsPrefix+d.bucketUrl+bucketName, updateconstants.RegionHolder, d.region, -1)
}

func (d *downloadManager) getStableVersionURL() (string, error) {
	s3BucketUrl := d.getS3BucketUrl()
	stableVersionURL := fmt.Sprintf("%s/%s/%s", s3BucketUrl, utility.StableVersionString, utility.VersionFile)
	s3URL, err := url.Parse(stableVersionURL)
	if err != nil {
		return "", fmt.Errorf("error while parsing stable version S3 URL: %v", err)
	}
	return s3URL.String(), nil
}

// getRegionManifestUrl gets region based manifest URL
func (d *downloadManager) getRegionManifestUrl() string {
	s3BucketUrl := d.getS3BucketUrl()
	s3URL, err := url.Parse(fmt.Sprintf("%s"+"/"+manifestJsonFileName, s3BucketUrl))
	if err != nil {
		return ""
	}
	return s3URL.String()
}
