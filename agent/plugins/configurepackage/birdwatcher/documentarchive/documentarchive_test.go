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
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher/archive"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher/facade"

	"github.com/aws/aws-sdk-go/service/ssm"

	"github.com/stretchr/testify/assert"
)

func TestGetResourceVersion(t *testing.T) {

	packageName := "Test Package"
	version1 := ""
	version2 := "1.2.3.4"
	latest := "latest"

	data := []struct {
		name         string
		packagename  string
		version      string
		facadeClient facade.FacadeStub
	}{
		{
			"ValidDistributionRule",
			packageName,
			latest,
			facade.FacadeStub{},
		},
		{
			"ValidDistributionRule_2",
			packageName,
			version1,
			facade.FacadeStub{},
		},
		{

			"ValidDistributionRule_3",
			packageName,
			version2,
			facade.FacadeStub{},
		},
	}

	for _, testdata := range data {
		t.Run(testdata.name, func(t *testing.T) {

			bwArchive := New(&testdata.facadeClient)

			names, versions := bwArchive.GetResourceVersion(testdata.packagename, testdata.version)
			assert.Equal(t, names, testdata.packagename)
			assert.Equal(t, versions, testdata.version)
		})
	}
}

func TestDownloadArchiveInfo(t *testing.T) {
	packageName := "ABC_package"
	versionName := "NewVersion"
	manifest := "manifest"
	docVersion := "1"
	emptystring := ""
	documentActive := ssm.DocumentStatusActive
	documentInactive := ssm.DocumentStatusCreating
	data := []struct {
		name         string
		isError      bool
		facadeClient facade.FacadeStub
	}{
		{
			"successful api call",
			false,
			facade.FacadeStub{
				GetDocumentOutput: &ssm.GetDocumentOutput{
					Content:         &manifest,
					Status:          &documentActive,
					VersionName:     &versionName,
					DocumentVersion: &docVersion,
					Name:            &packageName,
				},
			},
		},
		{
			"api call returns error",
			true,
			facade.FacadeStub{
				GetDocumentError: errors.New("testerror"),
			},
		},
		{
			"manifest is nil",
			true,
			facade.FacadeStub{
				GetDocumentOutput: &ssm.GetDocumentOutput{
					Content:         nil,
					Status:          &documentActive,
					VersionName:     &versionName,
					DocumentVersion: &docVersion,
					Name:            &packageName,
				},
			},
		},
		{
			"manifest is empty",
			true,
			facade.FacadeStub{
				GetDocumentOutput: &ssm.GetDocumentOutput{
					Content:         &emptystring,
					Status:          &documentActive,
					VersionName:     &versionName,
					DocumentVersion: &docVersion,
					Name:            &packageName,
				},
			},
		},
		{
			"document status is not active",
			true,
			facade.FacadeStub{
				GetDocumentOutput: &ssm.GetDocumentOutput{
					Content:         &emptystring,
					Status:          &documentInactive,
					VersionName:     &versionName,
					DocumentVersion: &docVersion,
					Name:            &packageName,
				},
			},
		},
	}
	for _, testdata := range data {
		t.Run(testdata.name, func(t *testing.T) {

			docArchive := New(&testdata.facadeClient)

			document, err := docArchive.DownloadArchiveInfo(packageName, versionName)
			if testdata.isError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, document, manifest)
			}
		})
	}
}

func TestGetFileDownloadLocation(t *testing.T) {
	packagename := "packagename"
	version := "version"
	filename := "test.zip"
	filename2 := "nottest.zip"
	url := "https://s3.test.com/birdwatcher/package/test.zip"
	noturl := "https://s3.test.com/birdwatcher/package/nottest.zip"
	hash := "abcdefghijklmnopqrstuvwxyz"
	hashtype := "sha256"
	manifest := "manifest"
	documentActive := ssm.DocumentStatusActive

	data := []struct {
		name         string
		isError      bool
		file         *archive.File
		facadeClient facade.FacadeStub
		attachments  []*ssm.AttachmentContent
		err          string
	}{
		{
			"successful api call with one attachment",
			false,
			&archive.File{
				filename,
				birdwatcher.FileInfo{},
			},
			facade.FacadeStub{},
			[]*ssm.AttachmentContent{
				{
					Name:     &filename,
					Url:      &url,
					HashType: &hashtype,
					Hash:     &hash,
				},
			},
			"",
		},
		{
			"successful api call with multiple attachment",
			false,
			&archive.File{
				filename,
				birdwatcher.FileInfo{},
			},
			facade.FacadeStub{},
			[]*ssm.AttachmentContent{
				{
					Name:     &filename2,
					Url:      &noturl,
					HashType: &hashtype,
					Hash:     &hash,
				},
				{
					Name:     &filename2,
					Url:      &noturl,
					HashType: &hashtype,
					Hash:     &hash,
				},
				{
					Name:     &filename,
					Url:      &url,
					HashType: &hashtype,
					Hash:     &hash,
				},
			},
			"",
		},
		{
			"successful api call with attachments not already included",
			false,
			&archive.File{
				filename,
				birdwatcher.FileInfo{},
			},
			facade.FacadeStub{
				GetDocumentOutput: &ssm.GetDocumentOutput{
					Content: &manifest,
					Status:  &documentActive,
					Name:    &packagename,
					AttachmentsContent: []*ssm.AttachmentContent{
						{
							Name:     &filename2,
							Url:      &noturl,
							HashType: &hashtype,
							Hash:     &hash,
						},
						{
							Name:     &filename2,
							Url:      &noturl,
							HashType: &hashtype,
							Hash:     &hash,
						},
						{
							Name:     &filename,
							Url:      &url,
							HashType: &hashtype,
							Hash:     &hash,
						},
					},
				},
			},
			nil,
			"",
		},
		{
			"unsuccessful call because file is nil ",
			true,
			nil,
			facade.FacadeStub{},
			[]*ssm.AttachmentContent{},
			"Could not obtain the file from manifest",
		},
		{
			"unsuccessful call because of attachment retreival error ",
			true,
			&archive.File{
				filename,
				birdwatcher.FileInfo{},
			},
			facade.FacadeStub{
				GetDocumentError: errors.New("testerror"),
			},
			nil,
			"failed to retrieve package document: testerror",
		},
		{
			"unsuccessful because no attachement received ",
			true,
			&archive.File{
				filename,
				birdwatcher.FileInfo{},
			},
			facade.FacadeStub{},
			nil,
			"Failed to retreive document for package installation",
		},
		{
			"unsuccessful call because no attachment has the file required for downlaod",
			true,
			&archive.File{
				filename,
				birdwatcher.FileInfo{},
			},
			facade.FacadeStub{},
			[]*ssm.AttachmentContent{},
			"Install attachments for package does not exist",
		},
	}
	for _, testdata := range data {
		t.Run(testdata.name, func(t *testing.T) {

			docArchive := NewWithAttachments(&testdata.facadeClient, testdata.attachments)

			location, err := docArchive.GetFileDownloadLocation(testdata.file, packagename, version)
			if testdata.isError {
				assert.Error(t, err)
				assert.Equal(t, testdata.err, err.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, url, location)
			}
		})

	}
}
