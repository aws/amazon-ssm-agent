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

package s3resource

import (
	filemock "github.com/aws/amazon-ssm-agent/agent/filemanager/mock"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/s3util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"errors"
	"testing"
)

var logMock = log.NewMockLog()

func TestS3Resource_ValidateLocationInfoPath(t *testing.T) {

	locationInfo := `{
		"Path": "newpath"
	}`

	s3resource, _ := NewS3Resource(logMock, locationInfo)
	_, err := s3resource.ValidateLocationInfo()

	assert.NoError(t, err)
}

func TestS3Resource_ValidateLocationInfoNoPath(t *testing.T) {

	locationInfo := `{
		"path": ""
	}`

	s3resource, _ := NewS3Resource(logMock, locationInfo)
	_, err := s3resource.ValidateLocationInfo()

	assert.Error(t, err)
	assert.Equal(t, err.Error(), "S3 source path in LocationType must be specified")
}

func TestIsFolder_JSON(t *testing.T) {
	res := isPathType("nameOfFolder/nameOfFile.json")

	assert.False(t, res)
}

func TestIsFolder_Folder(t *testing.T) {
	res := isPathType("nameOfFolder/someOtherFolder/")

	assert.True(t, res)
}

func TestS3Resource_GetS3BucketURLString(t *testing.T) {
	resource := &S3Resource{
		Info: S3Info{Path: "https://s3.amazonaws.com/my-bucket/mydummyfolder/myfile.ps"},
		s3Object: s3util.AmazonS3URL{
			Bucket: "my-bucket",
		},
	}

	res := resource.getS3BucketURLString()

	assert.Equal(t, "https://s3.amazonaws.com/my-bucket/", res)
}

func TestS3Resource_GetS3BucketURLString_sameBucketNameFile(t *testing.T) {
	resource := &S3Resource{
		Info: S3Info{Path: "https://s3.amazonaws.com/my-bucket/mydummyfolder/my-bucket.ps"},
		s3Object: s3util.AmazonS3URL{
			Bucket: "my-bucket",
		},
	}

	res := resource.getS3BucketURLString()

	assert.Equal(t, "https://s3.amazonaws.com/my-bucket/", res)
}

func TestS3Resource_Download(t *testing.T) {

	depMock := new(s3DepMock)
	locationInfo := `{
		"path" : "https://s3.amazonaws.com/my-bucket/mydummyfolder/file.rb"
	}`
	fileMock := filemock.FileSystemMock{}
	resource, _ := NewS3Resource(logMock, locationInfo)

	input := artifact.DownloadInput{
		DestinationDirectory: fileutil.BuildPath("destination", "my-bucket", "mydummyfolder"),
		SourceURL:            "https://s3.amazonaws.com/my-bucket/mydummyfolder/file.rb",
	}
	output := artifact.DownloadOutput{
		LocalFilePath: input.DestinationDirectory,
	}

	depMock.On("Download", logMock, input).Return(output, nil)
	fileMock.On("MoveAndRenameFile", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	dep = depMock
	err := resource.Download(logMock, fileMock, "destination")

	assert.NoError(t, err)
	depMock.AssertExpectations(t)
	fileMock.AssertExpectations(t)
}

func TestS3Resource_DownloadWrongPath(t *testing.T) {
	depMock := new(s3DepMock)
	locationInfo := `{
		"Path" : "https://s3.amazonaws.com/my-bucket/filename.ps"
	}`
	fileMock := filemock.FileSystemMock{}
	resource, _ := NewS3Resource(logMock, locationInfo)

	input := artifact.DownloadInput{
		DestinationDirectory: fileutil.BuildPath(appconfig.DownloadRoot, "my-bucket"),
		SourceURL:            "https://s3.amazonaws.com/my-bucket/filename.ps",
	}
	output := artifact.DownloadOutput{
		LocalFilePath: input.DestinationDirectory,
	}

	depMock.On("Download", logMock, input).Return(output, nil)
	fileMock.On("MoveAndRenameFile", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(true, errors.New("unexpected error encountered while moving the file. Error details -"))

	dep = depMock
	err := resource.Download(logMock, fileMock, "")

	assert.Error(t, err)
	depMock.AssertExpectations(t)
	fileMock.AssertExpectations(t)
	assert.Contains(t, err.Error(), "Something went wrong when trying to access downloaded content. It is "+
		"possible that the content was not downloaded because the path provided is wrong.")
}

func TestS3Resource_DownloadDirectory(t *testing.T) {
	depMock := new(s3DepMock)
	locationInfo := `{
		"path" : "https://s3.amazonaws.com/my-bucket/mydummyfolder/"
	}`
	fileMock := filemock.FileSystemMock{}
	resource, _ := NewS3Resource(logMock, locationInfo)
	resource.s3Object.Bucket = "my-bucket"
	resource.s3Object.Key = "mydummyfolder/"
	resource.s3Object.Region = "us-east-1"
	resource.s3Object.IsPathStyle = true
	resource.s3Object.IsValidS3URI = true

	input1 := artifact.DownloadInput{
		DestinationDirectory: fileutil.BuildPath("destination", "my-bucket/mydummyfolder/newFolder"),
		SourceURL:            "https://s3.amazonaws.com/my-bucket/mydummyfolder/newFolder/file.ps",
	}
	input2 := artifact.DownloadInput{
		DestinationDirectory: fileutil.BuildPath("destination", "my-bucket/mydummyfolder"),
		SourceURL:            "https://s3.amazonaws.com/my-bucket/mydummyfolder/filename.ps",
	}
	output := artifact.DownloadOutput{
		LocalFilePath: input1.DestinationDirectory,
	}

	var folders []string
	folders = append(folders, "mydummyfolder/newFolder/")
	folders = append(folders, "mydummyfolder/filename.ps")
	folders = append(folders, "mydummyfolder/newFolder/file.ps")

	depMock.On("ListS3Objects", logMock, resource.s3Object).Return(folders, nil).Once()
	depMock.On("Download", logMock, input2).Return(output, nil).Once()
	depMock.On("Download", logMock, input1).Return(output, nil).Once()
	fileMock.On("MoveAndRenameFile", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(true, nil).Twice()

	dep = depMock
	err := resource.Download(logMock, fileMock, "destination")

	assert.NoError(t, err)
	depMock.AssertExpectations(t)
	fileMock.AssertExpectations(t)
}
