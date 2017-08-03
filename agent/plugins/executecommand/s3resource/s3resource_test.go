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
	filemock "github.com/aws/amazon-ssm-agent/agent/plugins/executecommand/filemanager/mock"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/executecommand/remoteresource"
	"github.com/aws/amazon-ssm-agent/agent/s3util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"errors"
	"path/filepath"
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

func TestS3Resource_GetDirectoryURLString(t *testing.T) {
	resource := &S3Resource{
		Info: S3Info{Path: "https://s3.amazonaws.com/my-bucket/mydummyfolder/myfile.ps"},
	}

	res := resource.getDirectoryURLString()

	assert.Equal(t, "https://s3.amazonaws.com/my-bucket/mydummyfolder/", res)
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

func TestS3Resource_GetSourceURL_IsFolder(t *testing.T) {
	resource := &S3Resource{
		Info: S3Info{Path: "https://s3.amazonaws.com/my-bucket/mydummyfolder/myfile.ps"},
	}

	url, err := resource.getSourceURL(logMock, true)

	assert.NoError(t, err)
	assert.Equal(t, "https://s3.amazonaws.com/my-bucket/mydummyfolder/", url.String())
}

func TestS3Resource_GetSourceURL_NotFolder(t *testing.T) {
	resource := &S3Resource{
		Info: S3Info{Path: "https://s3.amazonaws.com/my-bucket/mydummyfolder/myfile.ps"},
	}

	url, err := resource.getSourceURL(logMock, false)

	assert.NoError(t, err)
	assert.Equal(t, "https://s3.amazonaws.com/my-bucket/mydummyfolder/myfile.ps", url.String())
}

func TestS3Resource_PopulateResourceInfoEntireDirFalseJSON(t *testing.T) {
	locationInfo := `{
		"path" : "https://s3.amazonaws.com/my-bucket/mydummyfolder/file.json"
	}`

	resourceInfo := remoteresource.ResourceInfo{}
	s3resource, _ := NewS3Resource(logMock, locationInfo)
	s3resource.s3Object.Bucket = "my-bucket"
	s3resource.s3Object.Key = "mydummyfolder/file.json"

	resourceInfo = s3resource.PopulateResourceInfo(logMock, "", false)

	assert.Equal(t, remoteresource.Document, resourceInfo.TypeOfResource)
	assert.False(t, resourceInfo.EntireDir)
	assert.Equal(t, "file.json", resourceInfo.StarterFile)
	assert.Equal(t, filepath.Join(appconfig.DownloadRoot, "my-bucket/mydummyfolder/file.json"), resourceInfo.LocalDestinationPath)
}

func TestS3Resource_PopulateResourceInfoEntireDirTrue(t *testing.T) {
	locationInfo := `{
		"path" : "https://s3.amazonaws.com/my-bucket/mydummyfolder/file.rb"
	}`

	resourceInfo := remoteresource.ResourceInfo{}
	s3resource, _ := NewS3Resource(logMock, locationInfo)

	s3resource.s3Object.Bucket = "my-bucket"
	s3resource.s3Object.Key = "mydummyfolder/file.rb"
	resourceInfo = s3resource.PopulateResourceInfo(logMock, "", true)

	assert.True(t, resourceInfo.EntireDir)
	assert.Equal(t, remoteresource.Script, resourceInfo.TypeOfResource)
	assert.NotEqual(t, "mydummyfolder/file.rb", resourceInfo.StarterFile)
	assert.Equal(t, resourceInfo.LocalDestinationPath, filepath.Join(appconfig.DownloadRoot, "my-bucket/mydummyfolder/file.rb"))
}

func TestS3Resource_PopulateResourceInfoEntireDirTrueInvalidStarter(t *testing.T) {
	locationInfo := `{
		"path" : "https://s3.amazonaws.com/my-bucket/path/to/"
	}`

	resourceInfo := remoteresource.ResourceInfo{}
	s3resource, _ := NewS3Resource(logMock, locationInfo)

	s3resource.s3Object.Bucket = "my-bucket"
	s3resource.s3Object.Key = "path/to/"
	resourceInfo = s3resource.PopulateResourceInfo(logMock, "", true)

	assert.True(t, resourceInfo.EntireDir)
	assert.Equal(t, remoteresource.Script, resourceInfo.TypeOfResource)
	assert.Equal(t, "to", resourceInfo.StarterFile)
	assert.Equal(t, resourceInfo.LocalDestinationPath, filepath.Join(appconfig.DownloadRoot, "my-bucket/path/to/"))

}

func TestS3Resource_PopulateResourceInfoEntireDirFalseScript(t *testing.T) {

	locationInfo := `{
		"path" : "https://s3.amazonaws.com/my-bucket/mydummyfolder/file.rb"
	}`

	resourceInfo := remoteresource.ResourceInfo{}
	s3resource, _ := NewS3Resource(logMock, locationInfo)

	s3resource.s3Object.Bucket = "my-bucket"
	s3resource.s3Object.Key = "mydummyfolder/file.rb"
	resourceInfo = s3resource.PopulateResourceInfo(logMock, "destination", false)

	assert.False(t, resourceInfo.EntireDir)
	assert.Equal(t, remoteresource.Script, resourceInfo.TypeOfResource)
	assert.Equal(t, "file.rb", resourceInfo.StarterFile)
	assert.Equal(t, resourceInfo.LocalDestinationPath, filepath.Join("destination", "my-bucket", "mydummyfolder/file.rb"))

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
	err := resource.Download(logMock, fileMock, false, "destination")

	assert.NoError(t, err)
	depMock.AssertExpectations(t)
	fileMock.AssertExpectations(t)
}

func TestS3Resource_DownloadEntireDirFalse(t *testing.T) {
	depMock := new(s3DepMock)
	locationInfo := `{
		"Path" : "https://s3.amazonaws.com/my-bucket/mydummyfolder"
	}`
	fileMock := filemock.FileSystemMock{}
	resource, _ := NewS3Resource(logMock, locationInfo)

	input := artifact.DownloadInput{
		DestinationDirectory: fileutil.BuildPath("destination", "my-bucket"),
		SourceURL:            "https://s3.amazonaws.com/my-bucket/mydummyfolder",
	}
	output := artifact.DownloadOutput{
		LocalFilePath: input.DestinationDirectory,
	}

	depMock.On("Download", logMock, input).Return(output, nil)
	fileMock.On("MoveAndRenameFile", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(true, errors.New("unexpected error encountered while moving the file. Error details -"))

	dep = depMock
	err := resource.Download(logMock, fileMock, false, "destination")

	assert.Error(t, err)
	depMock.AssertExpectations(t)
	fileMock.AssertExpectations(t)
	assert.Contains(t, err.Error(), "Something went wrong when trying to access downloaded content. It is "+
		"possible that the content was not downloaded because the path provided is wrong.")
}

func TestS3Resource_DownloadEntireDirTrue(t *testing.T) {
	depMock := new(s3DepMock)
	locationInfo := `{
		"path" : "https://s3.amazonaws.com/my-bucket/mydummyfolder/filename.ps"
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
	err := resource.Download(logMock, fileMock, true, "destination")

	assert.NoError(t, err)
	depMock.AssertExpectations(t)
	fileMock.AssertExpectations(t)
}

func TestS3Resource_DownloadEntireDirTruePathInvalid(t *testing.T) {
	depMock := new(s3DepMock)
	locationInfo := `{
		"path" : "https://s3.amazonaws.com/my-bucket/mydummyfolder/"
	}`
	fileMock := filemock.FileSystemMock{}
	resource, _ := NewS3Resource(logMock, locationInfo)

	dep = depMock
	err := resource.Download(logMock, fileMock, true, "")

	assert.Error(t, err)
	assert.Equal(t, "Could not download from S3. Please provide path to file with extention.", err.Error())
}
