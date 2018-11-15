// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package documentarchive contains the struct that is called when the package information is stored in birdwatcher
package documentarchive

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher/archive"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher/facade"

	"github.com/aws/aws-sdk-go/service/ssm"
)

type PackageArchive struct {
	facadeClient facade.BirdwatcherFacade
	attachments  []*ssm.AttachmentContent
	manifest     string
	archiveType  string
	documentArn  string
}

// New is a constructor for PackageArchive struct
func New(facadeClientSession facade.BirdwatcherFacade) archive.IPackageArchive {
	return &PackageArchive{
		facadeClient: facadeClientSession,
		archiveType:  archive.PackageArchiveDocument,
	}
}

// Name of archive type
func (da *PackageArchive) Name() string {
	return da.archiveType
}

// New is a constructor for PackageArchive struct with attachments. This method is mainly used for testing
func NewWithAttachments(facadeClientSession facade.BirdwatcherFacade, att []*ssm.AttachmentContent) archive.IPackageArchive {
	return &PackageArchive{
		facadeClient: facadeClientSession,
		attachments:  att,
	}
}

// GetResourceVersion makes a call to birdwatcher API to figure the right version of the resource that needs to be installed
func (da *PackageArchive) GetResourceVersion(packageName string, packageVersion string) (name string, version string) {
	// Return the packageVersion as "" if empty and return version if specified.
	return packageName, packageVersion
}

// DownloadArtifactInfo downloads the document using GetDocument and eventually gets the manifest from that and returns it
func (da *PackageArchive) DownloadArchiveInfo(packageName string, version string) (string, error) {
	// return manifest and error
	versionName := &version
	if version == "" {
		versionName = nil
	}
	MaxDelayBeforeCall := 15 //seconds
	// random back off before GetDocument call
	time.Sleep(time.Duration(getRandomBackOffTime(MaxDelayBeforeCall)) * time.Second)
	resp, err := da.facadeClient.GetDocument(
		&ssm.GetDocumentInput{
			Name:        &packageName,
			VersionName: versionName,
		},
	)

	if err != nil {
		return "", fmt.Errorf("failed to retrieve package document: %v", err)
	}

	if resp == nil {
		return "", fmt.Errorf("Failed to retreive document for package installation")
	}

	if *resp.Status != ssm.DocumentStatusActive {
		return "", fmt.Errorf("package document is not currently active, kindly retry when the status is active. Current document status - %v", *resp.Status)
	}

	if resp.Content == nil || *resp.Content == "" {
		return "", fmt.Errorf("failed to retrieve manifest from package document")
	}
	da.manifest = *resp.Content

	// Check if the packageAttachments exist when trying to use them again. In case they do not exist,
	// try get document again during download
	da.attachments = resp.AttachmentsContent

	// GetDocument returns the Name of the document if it belongs to the account of the instance.
	// If it is a shared document, GetDocument returns the document ARN as Name
	if resp.Name == nil || *resp.Name == "" {
		return "", fmt.Errorf("document name cannot be empty")
	}
	da.documentArn = *resp.Name

	return da.manifest, nil
}

// GetFileDownloadLocation obtains the location of the file in the archive
// in the document archive, this information is stored in the attachmentContent
// field in the reult of GetDocument.
func (da *PackageArchive) GetFileDownloadLocation(file *archive.File, packageName string, version string) (string, error) {
	if file == nil {
		return "", errors.New("Could not obtain the file from manifest")
	}
	fileName := file.Name

	//If the attachments are nil, try to get document again.
	if da.attachments == nil {
		// return manifest and error
		versionName := &version
		if version == "" {
			versionName = nil
		}
		resp, err := da.facadeClient.GetDocument(
			&ssm.GetDocumentInput{
				Name:        &packageName,
				VersionName: versionName,
			},
		)

		if err != nil {
			return "", fmt.Errorf("failed to retrieve package document: %v", err)
		}
		if resp == nil {
			return "", fmt.Errorf("Failed to retreive document for package installation")
		}

		if *resp.Status != ssm.DocumentStatusActive {
			return "", fmt.Errorf("package document is not currently active, kindly retry when the status is active. Current document status - %v", *resp.Status)
		}
		if resp.Name == nil || *resp.Name == "" {
			return "", fmt.Errorf("document name cannot be empty")
		}

		da.attachments = resp.AttachmentsContent
		da.documentArn = *resp.Name
	}

	// if the attachments are still nil, return error
	if da.attachments == nil {
		return "", fmt.Errorf("Could not retreive the attachments for installation")
	}

	for _, attachmentContent := range da.attachments {
		if *attachmentContent.Name == fileName {
			return *attachmentContent.Url, nil
		}
	}

	return "", fmt.Errorf("Install attachments for package does not exist")

}

// GetResourceArn returns the document Arn required for storing the file. This is found in the response of GetDocument.
func (da *PackageArchive) GetResourceArn(manifest *birdwatcher.Manifest) string {
	// GetDocument returns the Name of the document if it belongs to the account of the instance.
	// If it is a shared document, GetDocument returns the document ARN as Name
	return da.documentArn
}

func getRandomBackOffTime(timeInSeconds int) int {
	rand.Seed(time.Now().UnixNano())
	delay := rand.Intn(timeInSeconds)

	return delay
}
