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

//go:build e2e
// +build e2e

package s3resource

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	filemock "github.com/aws/amazon-ssm-agent/agent/fileutil/filemanager/mock"
	"github.com/aws/amazon-ssm-agent/agent/s3util"
	"github.com/stretchr/testify/assert"
)

var contextMock = context.NewMockDefault()

func TestS3Resource_ValidateLocationInfoPath(t *testing.T) {

	locationInfo := `{
		"Path": "newpath"
	}`

	s3resource, _ := NewS3Resource(contextMock, locationInfo)
	_, err := s3resource.ValidateLocationInfo()

	assert.NoError(t, err)
}

func TestS3Resource_ValidateLocationInfoNoPath(t *testing.T) {

	locationInfo := `{
		"path": ""
	}`

	s3resource, _ := NewS3Resource(contextMock, locationInfo)
	_, err := s3resource.ValidateLocationInfo()

	assert.Error(t, err)
	assert.Equal(t, err.Error(), "S3 source path in SourceInfo must be specified")
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
		context: contextMock,
		Info:    S3Info{Path: "https://s3.amazonaws.com/ssm-test-agent-bucket/mydummyfolder/myfile.ps"},
		s3Object: s3util.AmazonS3URL{
			Bucket: "ssm-test-agent-bucket",
		},
	}

	res, err := resource.getS3BucketURLString()

	assert.Equal(t, "https://s3.us-east-1.amazonaws.com/ssm-test-agent-bucket", res.String())
	assert.NoError(t, err)
}

func TestS3Resource_GetS3BucketURLString_sameBucketNameFile(t *testing.T) {
	resource := &S3Resource{
		context: contextMock,
		Info:    S3Info{Path: "https://s3.amazonaws.com/ssm-test-agent-bucket/mydummyfolder/my-bucket.ps"},
		s3Object: s3util.AmazonS3URL{
			Bucket: "ssm-test-agent-bucket",
		},
	}

	res, err := resource.getS3BucketURLString()

	assert.Equal(t, "https://s3.us-east-1.amazonaws.com/ssm-test-agent-bucket", res.String())
	assert.NoError(t, err)
}

func TestS3Resource_GetS3BucketURLString_hyphenatedEndpoint(t *testing.T) {
	resource := &S3Resource{
		context: contextMock,
		Info:    S3Info{Path: "https://s3-us-east-1.amazonaws.com/ssm-test-agent-bucket/mydummyfolder/my-bucket.ps"},
		s3Object: s3util.AmazonS3URL{
			Bucket: "ssm-test-agent-bucket",
		},
	}

	res, err := resource.getS3BucketURLString()

	assert.Equal(t, "https://s3.us-east-1.amazonaws.com/ssm-test-agent-bucket", res.String())
	assert.NoError(t, err)
}

func TestS3Resource_GetS3BucketURLString_bucketNameInS3URL(t *testing.T) {
	resource := &S3Resource{
		context: contextMock,
		Info:    S3Info{Path: "https://ssm-test-agent-bucket.s3.amazonaws.com/mydummyfolder/my-bucket.ps"},
		s3Object: s3util.AmazonS3URL{
			Bucket: "ssm-test-agent-bucket",
		},
	}

	res, err := resource.getS3BucketURLString()

	assert.Equal(t, "https://s3.us-east-1.amazonaws.com/ssm-test-agent-bucket", res.String())
	assert.NoError(t, err)
}

func TestS3Resource_Download(t *testing.T) {

	depMock := new(s3DepMock)
	locationInfo := `{
		"path" : "https://s3.amazonaws.com/ssm-test-agent-bucket/mydummyfolder/file.rb"
	}`
	fileMock := filemock.FileSystemMock{}

	fileMock.On("IsDirectory", "destination").Return(true)
	fileMock.On("Exists", "destination").Return(true)
	resource, _ := NewS3Resource(contextMock, locationInfo)

	input := artifact.DownloadInput{
		DestinationDirectory: "destination",
		SourceURL:            "https://s3.us-east-1.amazonaws.com/ssm-test-agent-bucket/mydummyfolder/file.rb",
	}
	output := artifact.DownloadOutput{
		LocalFilePath: input.DestinationDirectory,
	}

	s3Object := s3util.AmazonS3URL{
		IsValidS3URI: true,
		IsPathStyle:  true,
		Bucket:       "ssm-test-agent-bucket",
		Key:          "mydummyfolder/file.rb",
		Region:       "us-east-1",
	}
	var folders []string
	depMock.On("Download", contextMock, input).Return(output, nil)
	depMock.On("ListS3Directory", contextMock, s3Object).Return(folders, nil)

	fileMock.On("MoveAndRenameFile", ".", "destination", ".", "file.rb").Return(true, nil)

	dep = depMock
	err, result := resource.DownloadRemoteResource(fileMock, "destination")

	assert.NoError(t, err)
	depMock.AssertExpectations(t)
	fileMock.AssertExpectations(t)
	assert.NotNil(t, result)
	assert.Equal(t, 1, len(result.Files))
	assert.Equal(t, filepath.Join("destination", "file.rb"), result.Files[0])
}

func TestS3Resource_DownloadDirectory(t *testing.T) {
	depMock := new(s3DepMock)
	locationInfo := `{
		"Path" : "https://s3.amazonaws.com/ssm-test-agent-bucket/foldername"
	}`
	downloadsDirectory := strings.TrimSuffix(appconfig.DownloadRoot, string(os.PathSeparator))
	fileMock := filemock.FileSystemMock{}
	resource, _ := NewS3Resource(contextMock, locationInfo)

	input1 := artifact.DownloadInput{
		DestinationDirectory: downloadsDirectory,
		SourceURL:            "https://s3.us-east-1.amazonaws.com/ssm-test-agent-bucket/foldername/filename.ps",
	}
	input2 := artifact.DownloadInput{
		DestinationDirectory: downloadsDirectory,
		SourceURL:            "https://s3.us-east-1.amazonaws.com/ssm-test-agent-bucket/foldername/anotherfile.ps",
	}
	s3Object := s3util.AmazonS3URL{
		IsValidS3URI: true,
		IsPathStyle:  true,
		Bucket:       "ssm-test-agent-bucket",
		Key:          "foldername",
		Region:       "us-east-1",
	}
	output1 := artifact.DownloadOutput{
		LocalFilePath: filepath.Join(input1.DestinationDirectory, "randomfilename"),
	}
	output2 := artifact.DownloadOutput{
		LocalFilePath: filepath.Join(input2.DestinationDirectory, "anotherrandomfile"),
	}
	var folders []string
	folders = append(folders, "foldername/filename.ps")
	folders = append(folders, "foldername/anotherfile.ps")
	depMock.On("Download", contextMock, input1).Return(output1, nil).Once()
	depMock.On("Download", contextMock, input2).Return(output2, nil).Once()
	depMock.On("ListS3Directory", contextMock, s3Object).Return(folders, nil)

	fileMock.On("MoveAndRenameFile", downloadsDirectory, "randomfilename", downloadsDirectory, "filename.ps").Return(true, nil)
	fileMock.On("MoveAndRenameFile", downloadsDirectory, "anotherrandomfile", downloadsDirectory, "anotherfile.ps").Return(true, nil)

	dep = depMock
	err, result := resource.DownloadRemoteResource(fileMock, "")

	assert.NoError(t, err)
	depMock.AssertExpectations(t)
	fileMock.AssertExpectations(t)
	assert.NotNil(t, result)
	assert.Equal(t, 2, len(result.Files))
	assert.Equal(t, filepath.Join(downloadsDirectory, "filename.ps"), result.Files[0])
	assert.Equal(t, filepath.Join(downloadsDirectory, "anotherfile.ps"), result.Files[1])
}

func TestS3Resource_DownloadDirectoryWithSubFolders(t *testing.T) {
	depMock := new(s3DepMock)
	locationInfo := `{
		"Path" : "https://s3.amazonaws.com/ssm-test-agent-bucket/foldername"
	}`
	downloadsDirectory := strings.TrimSuffix(appconfig.DownloadRoot, string(os.PathSeparator))
	fileMock := filemock.FileSystemMock{}
	resource, _ := NewS3Resource(contextMock, locationInfo)

	input1 := artifact.DownloadInput{
		DestinationDirectory: downloadsDirectory,
		SourceURL:            "https://s3.us-east-1.amazonaws.com/ssm-test-agent-bucket/foldername/filename.ps",
	}
	input2 := artifact.DownloadInput{
		DestinationDirectory: downloadsDirectory,
		SourceURL:            "https://s3.us-east-1.amazonaws.com/ssm-test-agent-bucket/foldername/anotherfile.ps",
	}
	input3 := artifact.DownloadInput{
		DestinationDirectory: strings.TrimSuffix(filepath.Join(appconfig.DownloadRoot, "subfolder"), string(os.PathSeparator)),
		SourceURL:            "https://s3.us-east-1.amazonaws.com/ssm-test-agent-bucket/foldername/subfolder/file.ps",
	}
	s3Object := s3util.AmazonS3URL{
		IsValidS3URI: true,
		IsPathStyle:  true,
		Bucket:       "ssm-test-agent-bucket",
		Key:          "foldername",
		Region:       "us-east-1",
	}
	output1 := artifact.DownloadOutput{
		LocalFilePath: filepath.Join(input1.DestinationDirectory, "randomfilename"),
	}
	output2 := artifact.DownloadOutput{
		LocalFilePath: filepath.Join(input2.DestinationDirectory, "anotherrandomfile"),
	}
	output3 := artifact.DownloadOutput{
		LocalFilePath: filepath.Join(input3.DestinationDirectory, "justanumber"),
	}

	var folders []string
	folders = append(folders, "foldername/filename.ps")
	folders = append(folders, "foldername/anotherfile.ps")
	folders = append(folders, "foldername/subfolder/")
	folders = append(folders, "foldername/subfolder/file.ps")
	depMock.On("Download", contextMock, input1).Return(output1, nil).Once()
	depMock.On("Download", contextMock, input2).Return(output2, nil).Once()
	depMock.On("Download", contextMock, input3).Return(output3, nil).Once()
	depMock.On("ListS3Directory", contextMock, s3Object).Return(folders, nil)
	fileMock.On("MoveAndRenameFile", downloadsDirectory, "randomfilename", downloadsDirectory, "filename.ps").Return(true, nil)
	fileMock.On("MoveAndRenameFile", downloadsDirectory, "anotherrandomfile", downloadsDirectory, "anotherfile.ps").Return(true, nil)
	fileMock.On("MoveAndRenameFile", filepath.Join(downloadsDirectory, "subfolder"), "justanumber", filepath.Join(downloadsDirectory, "subfolder"), "file.ps").Return(true, nil)

	dep = depMock
	err, result := resource.DownloadRemoteResource(fileMock, "")

	assert.NoError(t, err)
	depMock.AssertExpectations(t)
	fileMock.AssertExpectations(t)
	assert.NotNil(t, result)
	assert.Equal(t, 3, len(result.Files))
	assert.Equal(t, filepath.Join(downloadsDirectory, "filename.ps"), result.Files[0])
	assert.Equal(t, filepath.Join(downloadsDirectory, "anotherfile.ps"), result.Files[1])
	assert.Equal(t, filepath.Join(downloadsDirectory, "subfolder", "file.ps"), result.Files[2])
}

func TestS3Resource_DownloadAbsPath(t *testing.T) {
	depMock := new(s3DepMock)
	locationInfo := `{
		"path" : "https://s3.amazonaws.com/ssm-test-agent-bucket/mydummyfolder/filename.ps"
	}`
	fileMock := filemock.FileSystemMock{}

	fileMock.On("IsDirectory", "/var/tmp/foldername").Return(true)
	fileMock.On("Exists", "/var/tmp/foldername").Return(true)
	resource, _ := NewS3Resource(contextMock, locationInfo)
	resource.s3Object.Bucket = "ssm-test-agent-bucket"
	resource.s3Object.Key = "mydummyfolder/filename.ps"
	resource.s3Object.Region = "us-east-1"
	resource.s3Object.IsPathStyle = true
	resource.s3Object.IsValidS3URI = true

	input := artifact.DownloadInput{
		DestinationDirectory: "/var/tmp/foldername",
		SourceURL:            "https://s3.us-east-1.amazonaws.com/ssm-test-agent-bucket/mydummyfolder/filename.ps",
	}
	output := artifact.DownloadOutput{
		LocalFilePath: filepath.Join(input.DestinationDirectory, "justanumber"),
	}

	var folders []string

	depMock.On("ListS3Directory", contextMock, resource.s3Object).Return(folders, nil).Once()
	depMock.On("Download", contextMock, input).Return(output, nil).Once()

	fileMock.On("MoveAndRenameFile", filepath.Join("/var", "tmp", "foldername"), "justanumber", filepath.Join("/var", "tmp", "foldername"), "filename.ps").Return(true, nil)

	dep = depMock
	err, result := resource.DownloadRemoteResource(fileMock, "/var/tmp/foldername")

	assert.NoError(t, err)
	depMock.AssertExpectations(t)
	fileMock.AssertExpectations(t)
	assert.NotNil(t, result)
	assert.Equal(t, 1, len(result.Files))
	assert.Equal(t, filepath.Join("/var", "tmp", "foldername", "filename.ps"), result.Files[0])
}

func TestS3Resource_DownloadRelativePathNameChange(t *testing.T) {

	depMock := new(s3DepMock)
	locationInfo := `{
		"path" : "https://s3.amazonaws.com/ssm-test-agent-bucket/mydummyfolder/file.rb"
	}`
	fileMock := filemock.FileSystemMock{}

	fileMock.On("Exists", "destination").Return(false)
	resource, _ := NewS3Resource(contextMock, locationInfo)

	input := artifact.DownloadInput{
		DestinationDirectory: ".",
		SourceURL:            "https://s3.us-east-1.amazonaws.com/ssm-test-agent-bucket/mydummyfolder/file.rb",
	}
	output := artifact.DownloadOutput{
		LocalFilePath: filepath.Join(input.DestinationDirectory, "random"),
	}

	s3Object := s3util.AmazonS3URL{
		IsValidS3URI: true,
		IsPathStyle:  true,
		Bucket:       "ssm-test-agent-bucket",
		Key:          "mydummyfolder/file.rb",
		Region:       "us-east-1",
	}
	var folders []string
	depMock.On("Download", contextMock, input).Return(output, nil)
	depMock.On("ListS3Directory", contextMock, s3Object).Return(folders, nil)

	fileMock.On("MoveAndRenameFile", ".", "random", ".", "destination").Return(true, nil)

	dep = depMock
	err, result := resource.DownloadRemoteResource(fileMock, "destination")

	assert.NoError(t, err)
	depMock.AssertExpectations(t)
	fileMock.AssertExpectations(t)
	assert.NotNil(t, result)
	assert.Equal(t, 1, len(result.Files))
	assert.Equal(t, "destination", result.Files[0])
}
