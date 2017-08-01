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

// Package ssmdocresource implements the methods to access resources from ssm
package ssmdocresource

import (
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/executecommand/filemanager"
	"github.com/aws/amazon-ssm-agent/agent/plugins/executecommand/remoteresource"
	"github.com/aws/aws-sdk-go/service/ssm"

	"errors"
	"fmt"
	"path/filepath"
)

// S3Resource is a struct for the remote resource of type git
type SSMDocResource struct {
	Info SSMDocInfo
}

// S3Info represents the locationInfo type sent by runcommand
type SSMDocInfo struct {
	DocName    string `json:"Name"`
	DocVersion string `json:"Version"`
}

// NewS3Resource is a constructor of type GitResource
func NewSSMDocResource(info string) (*SSMDocResource, error) {
	ssmDocInfo, err := parseLocationInfo(info)
	if err != nil {
		return nil, fmt.Errorf("ssmdoc url parsing failed. %v", err)
	}

	return &SSMDocResource{
		Info: ssmDocInfo,
	}, nil
}

// parseLocationInfo unmarshals the information in locationInfo of type GitInfo and returns it
func parseLocationInfo(locationInfo string) (ssmdoc SSMDocInfo, err error) {

	if err = jsonutil.Unmarshal(locationInfo, &ssmdoc); err != nil {
		return ssmdoc, fmt.Errorf("Location Info could not be unmarshalled for location type S3. Please check JSON format of locationInfo")
	}

	return ssmdoc, nil
}

// Download calls download to pull down files or directory from s3
func (ssmdoc *SSMDocResource) Download(log log.T, filesys filemanager.FileSystem, entireDir bool, destinationDir string) (err error) {
	if entireDir {
		return errors.New("EntireDirectory option is not supported for SSMDocument location type.")
	}

	if destinationDir == "" {
		destinationDir = appconfig.DownloadRoot
	}

	log.Debug("Making a call to get document", ssmdoc.Info.DocName, ssmdoc.Info.DocVersion)
	var docResponse *ssm.GetDocumentOutput
	if docResponse, err = ssmdocdep.GetDocument(log, ssmdoc.Info.DocName, ssmdoc.Info.DocVersion); err != nil {
		log.Errorf("Unable to get ssm document. %v", err)
		return err
	}

	destinationFilePath := filepath.Join(ssmdoc.Info.DocName, ssmdoc.Info.DocName+".json")
	if err = filemanager.SaveFileContent(log, filesys, destinationDir, *docResponse.Content, destinationFilePath); err != nil {
		log.Errorf("Error saving file - %v", err)
		return
	}

	return
}

// PopulateResourceInfo set the member variables of ResourceInfo
func (ssmdoc *SSMDocResource) PopulateResourceInfo(log log.T, destinationDir string, entireDir bool) remoteresource.ResourceInfo {
	var resourceInfo remoteresource.ResourceInfo

	//if destination directory is not specified, specify the directory
	if destinationDir == "" {
		destinationDir = appconfig.DownloadRoot
	}
	localDocName := ssmdoc.Info.DocName + ".json"
	resourceInfo.LocalDestinationPath = fileutil.BuildPath(destinationDir, ssmdoc.Info.DocName, localDocName)
	resourceInfo.StarterFile = filepath.Base(resourceInfo.LocalDestinationPath)
	resourceInfo.TypeOfResource = remoteresource.Document
	resourceInfo.EntireDir = entireDir

	return resourceInfo
}

// ValidateLocationInfo ensures that the required parameters of Location Info are specified
func (s3 *SSMDocResource) ValidateLocationInfo() (valid bool, err error) {
	if s3.Info.DocName == "" {
		return false, errors.New("SSM Document name in LocationType must be specified")
	}
	return true, nil
}
