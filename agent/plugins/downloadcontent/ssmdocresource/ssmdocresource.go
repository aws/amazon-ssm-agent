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
	"github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/remoteresource"
	"github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/system"
	ssmsvc "github.com/aws/amazon-ssm-agent/agent/ssm"
	"github.com/aws/aws-sdk-go/service/ssm"

	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// S3Resource is a struct for the remote resource of type git
type SSMDocResource struct {
	Info      SSMDocInfo
	ssmdocdep ssmdeps
}

// S3Info represents the sourceInfo type sent by runcommand
type SSMDocInfo struct {
	DocName string `json:"name"`
}

// NewS3Resource is a constructor of type GitResource
func NewSSMDocResource(info string) (*SSMDocResource, error) {
	ssmDocInfo, err := parseSourceInfo(info)
	if err != nil {
		return nil, fmt.Errorf("SSMDocument SourceInfo parsing failed. %v", err)
	}

	return &SSMDocResource{
		Info: ssmDocInfo,
		ssmdocdep: &ssmDocDepImpl{
			ssmSvc: ssmsvc.NewService(),
		},
	}, nil
}

// parseSourceInfo unmarshals the information in sourceInfo of type GitInfo and returns it
func parseSourceInfo(sourceInfo string) (ssmdoc SSMDocInfo, err error) {

	if err = jsonutil.Unmarshal(sourceInfo, &ssmdoc); err != nil {
		return ssmdoc, errors.New("SourceInfo could not be unmarshalled for SourceType SSMDocument. Please check JSON format of SourceInfo")
	}

	return ssmdoc, nil
}

// DownloadRemoteResource calls download to pull down files or directory from s3
func (ssmdoc *SSMDocResource) DownloadRemoteResource(log log.T, filesys filemanager.FileSystem, destinationPath string) (err error, result *remoteresource.DownloadResult) {
	if destinationPath == "" {
		destinationPath = appconfig.DownloadRoot
	}

	result = &remoteresource.DownloadResult{}

	docName, docVersion := docparser.ParseDocumentNameAndVersion(ssmdoc.Info.DocName)
	log.Debug("Making a call to get document", docName, docVersion)
	var docResponse *ssm.GetDocumentOutput
	if docResponse, err = ssmdoc.ssmdocdep.GetDocument(log, docName, docVersion); err != nil {
		log.Errorf("Unable to get ssm document. %v", err)
		return err, nil
	}

	var destinationFilePath string
	if filesys.Exists(destinationPath) && filesys.IsDirectory(destinationPath) || os.IsPathSeparator(destinationPath[len(destinationPath)-1]) {
		destinationFilePath = filepath.Join(destinationPath, filepath.Base(docName)+remoteresource.JSONExtension)

	} else {
		destinationFilePath = destinationPath
	}
	if err = system.SaveFileContent(log, filesys, destinationFilePath, *docResponse.Content); err != nil {
		log.Errorf("Error saving file - %v", err)
		return err, nil
	}

	result.Files = append(result.Files, destinationFilePath)

	return nil, result
}

// ValidateLocationInfo ensures that the required parameters of SourceInfo are specified
func (s3 *SSMDocResource) ValidateLocationInfo() (valid bool, err error) {
	if s3.Info.DocName == "" {
		return false, errors.New("SSM Document name in SourceType must be specified")
	}
	return true, nil
}
