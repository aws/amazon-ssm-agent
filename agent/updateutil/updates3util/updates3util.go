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

// Package updates3util implements the logic for s3 update download
package updates3util

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/backoffconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/network"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updatemanifest"
	"github.com/cenkalti/backoff/v4"
)

var (
	s3FileRead     = artifact.S3FileRead
	https3Download = httpDownload
)

func New(context context.T) T {
	return &updateS3UtilImpl{
		context: context.With("UpdateS3Util"),
	}
}

func (util *updateS3UtilImpl) resolveManifestUrl(manifestUrl string) (string, error) {
	var region string
	var err error

	region, err = util.context.Identity().Region()
	if err != nil {
		util.context.Log().Errorf("Failed to get region: %v", err)
		return "", err
	}

	if manifestUrl == "" {
		bucketURL := updateutil.ResolveAgentReleaseBucketURL(region, util.context.Identity())
		manifestUrl = bucketURL + updateconstants.ManifestFile
	} else {
		manifestUrl = strings.Replace(manifestUrl, updateconstants.RegionHolder, region, -1)
	}

	return manifestUrl, nil
}

// DownloadManifest downloads the agent manifest t
func (util *updateS3UtilImpl) DownloadManifest(manifest updatemanifest.T, manifestUrl string) *UpdateErrorStruct {
	logger := util.context.Log()
	var downloadOutput artifact.DownloadOutput

	manifestUrl, err := util.resolveManifestUrl(manifestUrl)
	if err != nil {
		return &UpdateErrorStruct{Error: err, ErrorCode: string(ResolveManifestURLErrorCode)}
	}
	logger.Infof("manifest download url is %s", manifestUrl)

	// Create temporary folder to download manifest to
	// If there is problem with default temp folder path, use the update artifacts folder to store the manifest
	tmpDownloadDir, err := createTempDir("", "")
	if err != nil {
		if tmpDownloadDir, err = createTempDir(appconfig.UpdaterArtifactsRoot, ""); err != nil {
			return &UpdateErrorStruct{Error: err, ErrorCode: string(TmpDownloadDirCreationErrorCode)}
		}
	}
	defer removeDir(tmpDownloadDir)

	downloadInput := artifact.DownloadInput{
		SourceURL:            manifestUrl,
		DestinationDirectory: tmpDownloadDir,
	}
	downloadOutput, err = fileDownload(util.context, downloadInput)
	if err != nil {
		return &UpdateErrorStruct{
			Error:     fmt.Errorf("failed to download file reliably, %v, %v", downloadInput.SourceURL, err.Error()),
			ErrorCode: string(NetworkFileDownloadErrorCode),
		}
	}
	if downloadOutput.IsHashMatched == false { // this should not happen for manifest file download
		return &UpdateErrorStruct{
			Error:     fmt.Errorf("failed to download file reliably, %v", downloadInput.SourceURL),
			ErrorCode: string(HashMismatchErrorCode),
		}
	}
	if downloadOutput.LocalFilePath == "" {
		return &UpdateErrorStruct{
			Error:     fmt.Errorf("failed to download file reliably, %v", downloadInput.SourceURL),
			ErrorCode: string(LocalFilePathEmptyErrorCode),
		}
	}

	logger.Infof("Succeed to download the manifest")
	logger.Infof("Local file path : %v", downloadOutput.LocalFilePath)

	if err = manifest.LoadManifest(downloadOutput.LocalFilePath); err != nil {
		logger.Errorf("failed to parse manifest: %v", err)
		return &UpdateErrorStruct{
			Error:     fmt.Errorf("failed to download file reliably, %v", downloadInput.SourceURL),
			ErrorCode: string(LoadManifestErrorCode),
		}
	}

	logger.Infof("Successfully parsed the manifest")
	return nil
}

// DownloadUpdater downloads updater from the s3 bucket
func (util *updateS3UtilImpl) DownloadUpdater(
	manifest updatemanifest.T,
	updaterPackageName string,
	downloadPath string,
) (string, error) {
	logger := util.context.Log()
	var versionStr, hash, source string
	var err error

	if versionStr, err = manifest.GetLatestVersion(updaterPackageName); err != nil {
		return "", err
	}
	logger.Infof("Latest updater version is %s", versionStr)
	if source, hash, err = manifest.GetDownloadURLAndHash(updaterPackageName, versionStr); err != nil {
		return "", err
	}
	logger.Infof("Latest updater url is %s", source)
	logger.Infof("Latest updater hash is %s", hash)

	downloadInput := artifact.DownloadInput{
		SourceURL: source,
		SourceChecksums: map[string]string{
			updateconstants.HashType: hash,
		},
		DestinationDirectory: downloadPath,
	}
	downloadOutput, downloadErr := fileDownload(util.context, downloadInput)
	if downloadErr != nil ||
		downloadOutput.IsHashMatched == false ||
		downloadOutput.LocalFilePath == "" {

		errMessage := fmt.Sprintf("failed to download file reliably, %v", downloadInput.SourceURL)
		if downloadErr != nil {
			errMessage = fmt.Sprintf("%v, %v", errMessage, downloadErr.Error())
		}
		return "", errors.New(errMessage)
	}
	logger.Infof("Successfully downloaded updater, attempting to decompress")

	if decompressErr := fileDecompress(
		util.context.Log(),
		downloadOutput.LocalFilePath,
		updateutil.UpdateArtifactFolder(appconfig.UpdaterArtifactsRoot, updaterPackageName, versionStr)); decompressErr != nil {
		return "", fmt.Errorf("failed to decompress updater package, %v, %v",
			downloadOutput.LocalFilePath,
			decompressErr.Error())
	}

	logger.Infof("Successfully decompressed the updater")

	return versionStr, nil
}

// GetStableVersion get the stable version from s3
func (util *updateS3UtilImpl) GetStableVersion(stableVersionUrl string) (string, error) {
	util.context.Log().Infof("Retrieving stable version from %s", stableVersionUrl)

	exponentialBackOff, err := backoffconfig.GetDefaultExponentialBackoff()
	if err != nil {
		return "", fmt.Errorf("failed to initialize backoff module: %v", err)
	}

	var content string
	err = backoff.Retry(func() error {
		// read from s3
		contentBytes, readErr := s3FileRead(util.context, stableVersionUrl)
		if contentBytes == nil || readErr != nil {
			util.context.Log().Infof("falling back to http download for stable version")
			httpTimeout := 15 * time.Second
			tr := network.GetDefaultTransport(util.context.Log(), util.context.AppConfig())
			client := &http.Client{
				Transport: tr,
				Timeout:   httpTimeout,
			}

			// use http client to download
			contentBytes, readErr = https3Download(stableVersionUrl, client)
			if readErr != nil {
				return fmt.Errorf("failed to read response from %s: %v", stableVersionUrl, readErr)
			}
			if contentBytes == nil {
				return fmt.Errorf("response code is nil")
			}
		}
		content = string(contentBytes)
		return nil
	}, exponentialBackOff)

	if err != nil {
		return "", fmt.Errorf("failed to get stable version from %s: %v", stableVersionUrl, err)
	}
	version := strings.TrimSpace(content)
	if !regexp.MustCompile(`^\d+.\d+.\d+.\d+$`).Match([]byte(version)) {
		return "", fmt.Errorf("invalid version format returned from %s: %s", stableVersionUrl, version)
	}

	util.context.Log().Infof("Got stable version: %s", version)
	return version, nil
}

func httpDownload(stableVersionUrl string, client *http.Client) ([]byte, error) {
	resp, err := client.Get(stableVersionUrl)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("response code is nil")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unsuccessful request: response code: %v", resp.StatusCode)
	}
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return content, fmt.Errorf("failed to read response from %s: %v", stableVersionUrl, err)
	}
	return content, nil
}
