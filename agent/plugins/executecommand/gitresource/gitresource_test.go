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

// Package gitresource implements the methods to access resources from git
package gitresource

import (
	githubclientmock "github.com/aws/amazon-ssm-agent/agent/githubclient/mock"
	filemock "github.com/aws/amazon-ssm-agent/agent/plugins/executecommand/filemanager/mock"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/executecommand/remoteresource"
	"github.com/go-github/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"fmt"
	"path/filepath"
	"testing"
)

var logMock = log.NewMockLog()

func NewResourceWithMockedClient(mockClient *githubclientmock.ClientMock) *GitResource {
	gitInfo := GitInfo{
		Owner:      "owner",
		Path:       "path/to/file.ext",
		Repository: "repo",
		GetOptions: "",
	}
	return &GitResource{
		client: mockClient,
		Info:   gitInfo,
	}
}

func TestGitResource_Download(t *testing.T) {
	clientMock := githubclientmock.ClientMock{}

	gitInfo := GitInfo{
		Owner:      "owner",
		Path:       "path/to/file.ext",
		Repository: "repo",
		GetOptions: "",
	}
	opt := &github.RepositoryContentGetOptions{Ref: ""}

	content := "content"
	file := "file"
	gitpath := "path/to/file.json"
	fileMetadata := github.RepositoryContent{
		Content: &content,
		Type:    &file,
		Path:    &gitpath,
	}
	var dirMetadata []*github.RepositoryContent
	dirMetadata = nil

	gitResource := NewResourceWithMockedClient(&clientMock)
	clientMock.On("ParseGetOptions", logMock, gitInfo.GetOptions).Return(opt, nil)
	clientMock.On("GetRepositoryContents", logMock, gitInfo.Owner, gitInfo.Repository, gitInfo.Path, opt).Return(&fileMetadata, dirMetadata, nil).Once()
	clientMock.On("IsFileContentType", mock.AnythingOfType("*github.RepositoryContent")).Return(true)

	fileMock := filemock.FileSystemMock{}
	fileMock.On("MakeDirs", mock.Anything).Return(nil)
	fileMock.On("WriteFile", mock.Anything, mock.Anything).Return(nil)

	err := gitResource.Download(logMock, fileMock, false, "")
	clientMock.AssertExpectations(t)
	fileMock.AssertExpectations(t)
	assert.NoError(t, err)
}

func TestGitResource_DownloadEntireDirFalse(t *testing.T) {
	clientMock := githubclientmock.ClientMock{}

	gitInfo := GitInfo{
		Owner:      "owner",
		Path:       "path/to/file.ext",
		Repository: "repo",
		GetOptions: "",
	}
	opt := &github.RepositoryContentGetOptions{Ref: ""}

	var fileMetadata *github.RepositoryContent
	fileMetadata = nil

	var dirMetadata []*github.RepositoryContent
	dirMetadata = nil

	clientMock.On("ParseGetOptions", logMock, gitInfo.GetOptions).Return(opt, nil)
	clientMock.On("GetRepositoryContents", logMock, gitInfo.Owner, gitInfo.Repository, gitInfo.Path, opt).Return(fileMetadata, dirMetadata, nil).Once()
	clientMock.On("IsFileContentType", mock.AnythingOfType("*github.RepositoryContent")).Return(false)
	fileMock := filemock.FileSystemMock{}

	gitResource := NewResourceWithMockedClient(&clientMock)

	err := gitResource.Download(logMock, fileMock, false, "")

	clientMock.AssertExpectations(t)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Path specified is a directory. Please specify entireDir as true if it is desired to download the entire directory")

}

func TestGitResource_DownloadFileMissing(t *testing.T) {
	clientMock := githubclientmock.ClientMock{}

	gitInfo := GitInfo{
		Owner:      "owner",
		Path:       "path/to",
		Repository: "repo",
		GetOptions: "",
	}
	opt := &github.RepositoryContentGetOptions{Ref: ""}

	var fileMetadata *github.RepositoryContent
	fileMetadata = nil

	var dirMetadata []*github.RepositoryContent
	dirMetadata = nil
	fileMock := filemock.FileSystemMock{}

	clientMock.On("ParseGetOptions", logMock, gitInfo.GetOptions).Return(opt, nil)
	clientMock.On("GetRepositoryContents", logMock, gitInfo.Owner, gitInfo.Repository, gitInfo.Path, opt).Return(fileMetadata, dirMetadata, nil).Once()
	clientMock.On("IsFileContentType", mock.AnythingOfType("*github.RepositoryContent")).Return(false)

	gitResource := NewResourceWithMockedClient(&clientMock)

	err := gitResource.Download(logMock, fileMock, true, "")

	clientMock.AssertExpectations(t)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Could not download from github repository")
}

func TestGitResource_DownloadParseGetOptionFail(t *testing.T) {
	clientMock := githubclientmock.ClientMock{}

	gitInfo := GitInfo{
		Owner:      "owner",
		Path:       "path/to/file.ext",
		Repository: "repo",
		GetOptions: "",
	}
	opt := &github.RepositoryContentGetOptions{Ref: ""}

	clientMock.On("ParseGetOptions", logMock, gitInfo.GetOptions).Return(opt, fmt.Errorf("Option for retreiving git content is empty")).Once()

	gitResource := NewResourceWithMockedClient(&clientMock)

	fileMock := filemock.FileSystemMock{}
	err := gitResource.Download(logMock, fileMock, true, "")

	clientMock.AssertExpectations(t)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Option for retreiving git content is empty")
}

func TestGitResource_DownloadGetRepositoryContentsFail(t *testing.T) {
	clientMock := githubclientmock.ClientMock{}

	gitInfo := GitInfo{
		Owner:      "owner",
		Path:       "path/to",
		Repository: "repo",
		GetOptions: "",
	}
	opt := &github.RepositoryContentGetOptions{Ref: ""}

	var fileMetadata *github.RepositoryContent
	fileMetadata = nil

	var dirMetadata []*github.RepositoryContent
	dirMetadata = nil
	var mockErr error
	mockErr = fmt.Errorf("Rate limit exceeded")

	fileMock := filemock.FileSystemMock{}
	clientMock.On("ParseGetOptions", logMock, gitInfo.GetOptions).Return(opt, nil).Once()
	clientMock.On("GetRepositoryContents", logMock, gitInfo.Owner, gitInfo.Repository, gitInfo.Path, opt).Return(fileMetadata, dirMetadata, mockErr).Once()

	gitResource := NewResourceWithMockedClient(&clientMock)

	err := gitResource.Download(logMock, fileMock, true, "")

	clientMock.AssertExpectations(t)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Rate limit exceeded")
}

func TestGitResource_PopulateResourceInfoEntireDirFalseJSON(t *testing.T) {
	locationInfo := `{
		"owner": "owner",
		"repository": "repo",
		"path": "path/to/file.json",
		"getOptions": ""
	}`

	resourceInfo := remoteresource.ResourceInfo{}
	gitresource, _ := NewGitResource(nil, locationInfo)

	resourceInfo = gitresource.PopulateResourceInfo(logMock, "", false)

	assert.Equal(t, remoteresource.Document, resourceInfo.TypeOfResource)
	assert.False(t, resourceInfo.EntireDir)
	assert.Equal(t, "file.json", resourceInfo.StarterFile)
	assert.Equal(t, filepath.Join(appconfig.DownloadRoot, "path/to/file.json"), resourceInfo.LocalDestinationPath)
}

func TestGitResource_PopulateResourceInfoEntireDirTrue(t *testing.T) {
	locationInfo := `{
		"owner": "owner",
		"repository": "repo",
		"path": "path/to/file.rb",
		"getOptions": ""
	}`

	gitresource, _ := NewGitResource(nil, locationInfo)

	resourceInfo := gitresource.PopulateResourceInfo(logMock, "", true)

	assert.True(t, resourceInfo.EntireDir)
	assert.Equal(t, remoteresource.Script, resourceInfo.TypeOfResource)
	assert.NotEqual(t, "path/to/file.rb", resourceInfo.StarterFile)
	assert.Equal(t, resourceInfo.LocalDestinationPath, filepath.Join(appconfig.DownloadRoot, "path/to/file.rb"))

}

func TestGitResource_PopulateResourceInfoEntireDirTrueInvalidStarter(t *testing.T) {
	locationInfo := `{
		"owner": "owner",
		"repository": "repo",
		"path": "path/to/",
		"getOptions": ""
	}`

	gitresource, _ := NewGitResource(nil, locationInfo)

	resourceInfo := gitresource.PopulateResourceInfo(logMock, "", true)

	assert.True(t, resourceInfo.EntireDir)
	assert.Equal(t, remoteresource.Script, resourceInfo.TypeOfResource)
	assert.Equal(t, "to", resourceInfo.StarterFile)
	assert.Equal(t, resourceInfo.LocalDestinationPath, filepath.Join(appconfig.DownloadRoot, "path/to/"))

}

func TestGitResource_PopulateResourceInfoEntireDirFalseScript(t *testing.T) {

	locationInfo := `{
		"owner": "owner",
		"repository": "repo",
		"path":"path/to/file.rb",
		"getOptions": ""
	}`

	gitresource, _ := NewGitResource(nil, locationInfo)
	resourceInfo := gitresource.PopulateResourceInfo(logMock, "destination", false)

	assert.False(t, resourceInfo.EntireDir)
	assert.Equal(t, remoteresource.Script, resourceInfo.TypeOfResource)
	assert.Equal(t, "file.rb", resourceInfo.StarterFile)
	assert.Equal(t, resourceInfo.LocalDestinationPath, filepath.Join("destination", "path/to/file.rb"))

}

func TestGitResource_ValidateLocationInfoOwner(t *testing.T) {
	locationInfo := `{
		"repository": "repo",
		"path":"path/to/file.rb",
		"getOptions": ""
	}`

	gitresource, _ := NewGitResource(nil, locationInfo)
	_, err := gitresource.ValidateLocationInfo()

	assert.Error(t, err)
	assert.Equal(t, "Owner for Git LocationType must be specified", err.Error())
}

func TestGitResource_ValidateLocationInfoRepo(t *testing.T) {
	locationInfo := `{
		"owner": "owner",
		"path":"path/to/file.rb",
		"getOptions": ""
	}`
	gitresource, _ := NewGitResource(nil, locationInfo)
	_, err := gitresource.ValidateLocationInfo()

	assert.Error(t, err)
	assert.Equal(t, "Repository for Git LocationType must be specified", err.Error())
}
func TestGitResource_ValidateLocationInfoPath(t *testing.T) {

	locationInfo := `{
		"owner": "owner",
		"repository": "repo",
		"getOptions": ""
	}`

	gitresource, _ := NewGitResource(nil, locationInfo)
	_, err := gitresource.ValidateLocationInfo()

	assert.Error(t, err)
	assert.Equal(t, "Path for Git LocationType must be specified", err.Error())

}
func TestGitResource_ValidateLocationInfo(t *testing.T) {

	locationInfo := `{
		"owner": "owner",
		"repository": "repo",
		"path":"path/to/file.rb",
		"getOptions": ""
	}`

	gitresource, _ := NewGitResource(nil, locationInfo)
	_, err := gitresource.ValidateLocationInfo()

	assert.NoError(t, err)
}

func TestNewGitResource_parseLocationInfoFail(t *testing.T) {

	_, err := NewGitResource(nil, "")

	assert.Error(t, err)
	assert.Equal(t, "Location Info could not be unmarshalled for location type Git. Please check JSON format of locationInfo", err.Error())
}
