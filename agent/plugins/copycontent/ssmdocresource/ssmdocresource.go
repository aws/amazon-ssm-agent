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
	"github.com/aws/amazon-ssm-agent/agent/docparser"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/filemanager"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/copycontent/remoteresource"
	"github.com/aws/amazon-ssm-agent/agent/plugins/copycontent/system"
	"github.com/aws/aws-sdk-go/service/ssm"

	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// S3Resource is a struct for the remote resource of type git
type SSMDocResource struct {
	Info SSMDocInfo
}

// S3Info represents the locationInfo type sent by runcommand
type SSMDocInfo struct {
	DocName string `json:"name"`
}

// NewS3Resource is a constructor of type GitResource
func NewSSMDocResource(info string) (*SSMDocResource, error) {
	ssmDocInfo, err := parseLocationInfo(info)
	if err != nil {
		return nil, fmt.Errorf("SSMDocument LocationInfo parsing failed. %v", err)
	}

	return &SSMDocResource{
		Info: ssmDocInfo,
	}, nil
}

// parseLocationInfo unmarshals the information in locationInfo of type GitInfo and returns it
func parseLocationInfo(locationInfo string) (ssmdoc SSMDocInfo, err error) {

	if err = jsonutil.Unmarshal(locationInfo, &ssmdoc); err != nil {
		return ssmdoc, errors.New("Location Info could not be unmarshalled for location type SSMDocument. Please check JSON format of locationInfo")
	}

	return ssmdoc, nil
}

// Download calls download to pull down files or directory from s3
func (ssmdoc *SSMDocResource) Download(log log.T, filesys filemanager.FileSystem, destinationPath string) (err error) {

	if destinationPath == "" {
		destinationPath = appconfig.DownloadRoot
	}

	//This gets the document name if the fullARN is provided
	docNameWithVersion := filepath.Base(ssmdoc.Info.DocName)
	docName, docVersion := docparser.ParseDocumentNameAndVersion(docNameWithVersion)
	log.Debug("Making a call to get document", docName, docVersion)
	var docResponse *ssm.GetDocumentOutput
	if docResponse, err = ssmdocdep.GetDocument(log, docName, docVersion); err != nil {
		log.Errorf("Unable to get ssm document. %v", err)
		return err
	}

	var destinationFilePath string
	if filesys.Exists(destinationPath) && filesys.IsDirectory(destinationPath) || os.IsPathSeparator(destinationPath[len(destinationPath)-1]) {
		destinationFilePath = filepath.Join(destinationPath, docName+remoteresource.JSONExtension)

	} else {
		destinationFilePath = destinationPath
	}
	if err = system.SaveFileContent(log, filesys, destinationFilePath, *docResponse.Content); err != nil {
		log.Errorf("Error saving file - %v", err)
		return
	}

	return
}

// ValidateLocationInfo ensures that the required parameters of Location Info are specified
func (s3 *SSMDocResource) ValidateLocationInfo() (valid bool, err error) {
	if s3.Info.DocName == "" {
		return false, errors.New("SSM Document name in LocationType must be specified")
	}
	return true, nil
}
