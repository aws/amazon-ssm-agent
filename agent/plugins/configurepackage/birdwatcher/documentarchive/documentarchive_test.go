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

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher/archive"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher/facade"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/packageservice"
	cache_mock "github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/packageservice/mock"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/trace"

	"github.com/aws/aws-sdk-go/service/ssm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGetRandomBackOffTime(t *testing.T) {
	delay := getRandomBackOffTime(15)
	errorInDuration := false
	if delay > 15 || delay < 1 {
		t.Log("Value of delay is ", delay)
		errorInDuration = true
	}
	assert.False(t, errorInDuration)
}

func TestArchiveName(t *testing.T) {
	facadeSession := facade.FacadeStub{}
	testArchive := New(&facadeSession)

	assert.Equal(t, archive.PackageArchiveDocument, testArchive.Name())

}

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

func TestSetAndGetResources(t *testing.T) {
	manifest := birdwatcher.Manifest{}
	packageName := "packagename"
	docDescription := createDefaultDocumentDescription(packageName, "hash", ssm.DocumentStatusActive)
	packageArchive := PackageArchive{
		documentDesc: &docDescription,
		docVersion:   "abc",
		documentArn:  packageName,
	}

	// TODO: Fix this - For document archive, the value of version that is given to Set and Get resource is "don't care"
	packageArchive.SetResource(packageName, "version", &manifest)
	arn := packageArchive.GetResourceArn(packageName, "version")

	assert.Equal(t, arn, *docDescription.Name)
	assert.Equal(t, packageArchive.docVersion, *docDescription.DocumentVersion)
	assert.Equal(t, packageArchive.documentArn, *docDescription.Name)
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
	documentDescription := createDefaultDocumentDescription(packagename, "hash", documentActive)
	memcache := createMemCache("hash", manifest)
	docVersion := "2"

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
					Content:         &manifest,
					Status:          &documentActive,
					Name:            &packagename,
					DocumentVersion: &docVersion,
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

			docArchive := NewDocumentArchive(&testdata.facadeClient, testdata.attachments, &documentDescription, memcache, manifest)

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

func TestDownloadArchiveInfo(t *testing.T) {
	tracer := trace.NewTracer(log.NewMockLog())
	packageName := "ABC_package"
	documentArn := "arn:aws:ssm:us-east-1:1234567890:document/NameOfDoc"
	versionName := "NewVersion"
	manifest := "manifest"
	docVersion := "1"
	emptystring := ""
	documentActive := ssm.DocumentStatusActive
	documentInactive := ssm.DocumentStatusCreating
	myPrettyHash := "myPrettHash"
	data := []struct {
		name                string
		version             string
		isError             bool
		facadeClient        facade.FacadeStub
		documentDescription ssm.DocumentDescription
		manifestCache       packageservice.ManifestCache
	}{
		{
			"successful api call, Document already exists, no GetDocument happy case",
			versionName,
			false,
			facade.FacadeStub{},
			createDefaultDocumentDescription(packageName, myPrettyHash, documentActive),
			createMemCache(myPrettyHash, manifest),
		},
		{
			"successful api call returning the document ARN, no GetDocument happy case",
			versionName,
			false,
			facade.FacadeStub{},
			createDefaultDocumentDescription(documentArn, myPrettyHash, documentActive),
			createMemCache(myPrettyHash, manifest),
		},
		{
			"describe document call returns error",
			versionName,
			true,
			facade.FacadeStub{
				DescribeDocumentError: errors.New("testerror"),
			},
			createDefaultDocumentDescription(packageName, myPrettyHash, documentActive),
			createMemCache(myPrettyHash, manifest),
		},
		{
			"hash mismatch, get manifest happy path",
			versionName,
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
			createDefaultDocumentDescription(packageName, "hash123", documentActive),
			createMemCache(myPrettyHash, manifest),
		},
		{
			"hash mismatch, get document, manifest is nil",
			versionName,
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
			createDefaultDocumentDescription(packageName, "hash123", documentActive),
			createMemCache(myPrettyHash, manifest),
		},
		{
			"hash mismatch, get document call returns empty manifest",
			versionName,
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
			createDefaultDocumentDescription(packageName, "hash123", documentActive),
			createMemCache(myPrettyHash, manifest),
		},
		{
			"hash mismatch, get document status inactive",
			versionName,
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
			createDefaultDocumentDescription(packageName, "hash123", documentActive),
			createMemCache(myPrettyHash, manifest),
		},
		{
			"describe document returns document status not active",
			versionName,
			true,
			facade.FacadeStub{},
			createDefaultDocumentDescription(packageName, myPrettyHash, documentInactive),
			createMemCache(myPrettyHash, manifest),
		},
		{
			"version is empty",
			emptystring,
			false,
			facade.FacadeStub{
				GetDocumentOutput: &ssm.GetDocumentOutput{
					Content:         &manifest,
					Status:          &documentInactive,
					VersionName:     &versionName,
					DocumentVersion: &docVersion,
					Name:            &packageName,
				},
			},
			createDefaultDocumentDescription(packageName, myPrettyHash, documentActive),
			createMemCache(myPrettyHash, manifest),
		},
		{
			"version name is not provided",
			emptystring,
			false,
			facade.FacadeStub{
				GetDocumentOutput: &ssm.GetDocumentOutput{
					Content:         &manifest,
					Status:          &documentInactive,
					VersionName:     nil,
					DocumentVersion: &docVersion,
					Name:            &packageName,
				},
			},
			createDefaultDocumentDescription(packageName, myPrettyHash, documentActive),
			createMemCache(myPrettyHash, manifest),
		},
	}
	for _, testdata := range data {
		t.Run(testdata.name, func(t *testing.T) {
			testdata.facadeClient.DescribeDocumentOutput = &ssm.DescribeDocumentOutput{
				Document: &testdata.documentDescription,
			}
			docArchive := NewDocumentArchive(&testdata.facadeClient, nil, &testdata.documentDescription, testdata.manifestCache, manifest)

			document, err := docArchive.DownloadArchiveInfo(tracer, packageName, testdata.version)
			if testdata.isError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, document, manifest)
			}
		})
	}
}

// helpers
func createDefaultDocumentDescription(packageName string, hash string, docStatus string) ssm.DocumentDescription {
	docVersion := "2"
	versionName := "version-name"
	latest := "3"
	defaultVersion := "1"
	return ssm.DocumentDescription{
		Name:            &packageName,
		DocumentVersion: &docVersion,
		VersionName:     &versionName,
		LatestVersion:   &latest,
		DefaultVersion:  &defaultVersion,
		Hash:            &hash,
		Status:          &docStatus,
	}

}

func createMemCache(hash string, manifest string) cache_mock.ManifestCache {
	cache := cache_mock.ManifestCache{}
	cache.On("ReadManifestHash", mock.Anything, mock.Anything).Return([]byte(hash), nil)
	cache.On("ReadManifest", mock.Anything, mock.Anything).Return([]byte(manifest), nil)
	cache.On("WriteManifestHash", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	return cache
}
