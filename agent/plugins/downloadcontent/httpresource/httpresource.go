/*
 * Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"). You may not
 * use this file except in compliance with the License. A copy of the
 * License is located at
 *
 * http://aws.amazon.com/apache2.0/
 *
 * or in the "license" file accompanying this file. This file is distributed
 * on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing
 * permissions and limitations under the License.
 */

// Package httpresource provides methods to download resources over HTTP(s)
package httpresource

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/filemanager"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/httpresource/handler"
	"github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/remoteresource"
	"github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/types"
	"github.com/aws/amazon-ssm-agent/agent/ssm/ssmparameterresolver"
)

// HTTPResource represents an HTTP(s) resource
type HTTPResource struct {
	context context.T
	Handler handler.IHTTPHandler
}

// HTTPInfo defines the accepted SourceInfo attributes and their json definition
type HTTPInfo struct {
	URL                   types.TrimmedString `json:"url"`
	AuthMethod            types.TrimmedString `json:"authMethod"`
	Username              types.TrimmedString `json:"username"`
	Password              types.TrimmedString `json:"password"`
	AllowInsecureDownload bool                `json:"allowInsecureDownload"`
}

// NewHTTPResource creates a new HTTP resource
func NewHTTPResource(context context.T, info string, bridge ssmparameterresolver.ISsmParameterResolverBridge) (resource *HTTPResource, err error) {
	var httpInfo HTTPInfo
	if httpInfo, err = parseSourceInfo(info); err != nil {
		return nil, err
	}

	parsedUrl, err := url.Parse(httpInfo.URL.Val())
	if err != nil {
		return nil, fmt.Errorf("Invalid URL format: %s", err.Error())
	}

	httpClient := http.Client{}
	httpClient.CloseIdleConnections()

	return &HTTPResource{
		context: context,
		Handler: handler.NewHTTPHandler(httpClient, *parsedUrl, httpInfo.AllowInsecureDownload, handler.HTTPAuthConfig{
			AuthMethod: httpInfo.AuthMethod,
			Username:   httpInfo.Username,
			Password:   httpInfo.Password,
		}, bridge),
	}, nil
}

// DownloadRemoteResource downloads a HTTP resource into a specific download path
func (resource *HTTPResource) DownloadRemoteResource(fileSystem filemanager.FileSystem, downloadPath string) (err error, result *remoteresource.DownloadResult) {
	log := resource.context.Log()
	downloadPath = resource.adjustDownloadPath(downloadPath, fmt.Sprintf("%d", rand.Int()), fileSystem)

	err = fileSystem.MakeDirs(filepath.Dir(downloadPath))
	if err != nil {
		return fmt.Errorf("Cannot create download path %s: %v", filepath.Dir(downloadPath), err.Error()), nil
	}

	log.Debug("Destination path to download - ", downloadPath)

	downloadedFilepath, err := resource.Handler.Download(log, fileSystem, downloadPath)
	if err != nil {
		return err, nil
	}

	return nil, &remoteresource.DownloadResult{
		Files: []string{downloadedFilepath},
	}
}

// ValidateLocationInfo validates attribute values of an HTTP resource
func (resource *HTTPResource) ValidateLocationInfo() (isValid bool, err error) {
	return resource.Handler.Validate()
}

// parseSourceInfo unmarshalls the provided SourceInfo input
func parseSourceInfo(sourceInfo string) (httpInfo HTTPInfo, err error) {
	if err = jsonutil.Unmarshal(sourceInfo, &httpInfo); err != nil {
		return httpInfo, fmt.Errorf("SourceInfo could not be unmarshalled for source type HTTP: %s", err.Error())
	}

	return httpInfo, nil
}

// adjustDownloadPath sets the download path of the resource based on the user input
func (resource *HTTPResource) adjustDownloadPath(destPath, fileSuffix string, fileSystem filemanager.FileSystem) string {
	if destPath == "" {
		destPath = appconfig.DownloadRoot
	}

	// Mimicking S3 download functionality, which treats the destination path differently based on the following conditions
	if (fileSystem.Exists(destPath) && fileSystem.IsDirectory(destPath)) || os.IsPathSeparator(destPath[len(destPath)-1]) {
		destPath = filepath.Join(destPath, fmt.Sprintf("%s%s", "download", fileSuffix))
	}

	return destPath
}
