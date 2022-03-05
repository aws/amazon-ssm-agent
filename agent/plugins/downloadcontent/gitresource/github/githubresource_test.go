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

// Package github implements the methods to access resources from git
package github

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	filemock "github.com/aws/amazon-ssm-agent/agent/fileutil/filemanager/mock"
	"github.com/aws/amazon-ssm-agent/agent/log"
	githubclientmock "github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/gitresource/github/privategithub/githubclient/mock"
	"github.com/go-github/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var logMock = log.NewMockLog()

func NewResourceWithMockedClient(mockClient *githubclientmock.ClientMock) *GitHubResource {
	info := GitHubInfo{
		Owner:      "owner",
		Path:       "path/to/file.ext",
		Repository: "repo",
		GetOptions: "",
	}
	return &GitHubResource{
		client: mockClient,
		Info:   info,
	}
}

func TestGitResource_DownloadFile(t *testing.T) {
	clientMock := githubclientmock.ClientMock{}

	info := GitHubInfo{
		Owner:      "owner",
		Path:       "path/to/file.ext",
		Repository: "repo",
		GetOptions: "",
	}
	opt := &github.RepositoryContentGetOptions{Ref: ""}

	content := "content"
	file := "file"
	gitpath := "path/to/file.ext"
	fileMetadata := github.RepositoryContent{
		Content: &content,
		Type:    &file,
		Path:    &gitpath,
	}
	var dirMetadata []*github.RepositoryContent
	dirMetadata = nil

	resource := NewResourceWithMockedClient(&clientMock)
	clientMock.On("ParseGetOptions", logMock, info.GetOptions).Return(opt, nil)
	clientMock.On("GetRepositoryContents", logMock, info.Owner, info.Repository, info.Path, opt).Return(&fileMetadata, dirMetadata, nil).Once()
	clientMock.On("IsFileContentType", mock.AnythingOfType("*github.RepositoryContent")).Return(true)

	fileMock := filemock.FileSystemMock{}

	fileMock.On("IsDirectory", appconfig.DownloadRoot).Return(true)
	fileMock.On("Exists", appconfig.DownloadRoot).Return(true)
	fileMock.On("MakeDirs", strings.TrimSuffix(appconfig.DownloadRoot, "/")).Return(nil)
	fileMock.On("WriteFile", filepath.Join(appconfig.DownloadRoot, "file.ext"), mock.Anything).Return(nil)

	err, result := resource.DownloadRemoteResource(logMock, fileMock, "")
	clientMock.AssertExpectations(t)
	fileMock.AssertExpectations(t)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, len(result.Files))
	assert.Equal(t, fileutil.BuildPath(appconfig.DownloadRoot, "file.ext"), result.Files[0])
}

func TestGitResource_DownloadDirectory(t *testing.T) {
	clientMock := githubclientmock.ClientMock{}

	info := GitHubInfo{
		Owner:      "owner",
		Path:       "path/to/dir/",
		Repository: "repo",
		GetOptions: "",
	}
	opt := &github.RepositoryContentGetOptions{Ref: ""}

	content := "content"
	file := "file"
	filepath := "path/to/dir/file.rb"
	var fileMetadata, nilFileMetadata github.RepositoryContent

	fileMetadata = github.RepositoryContent{
		Content: &content,
		Type:    &file,
		Path:    &filepath,
	}
	var dirMetadata, nilDirMetadata []*github.RepositoryContent
	dirMetadata = append(dirMetadata, &fileMetadata)
	nilDirMetadata = nil

	resource := &GitHubResource{
		client: &clientMock,
		Info:   info,
	}
	clientMock.On("ParseGetOptions", logMock, info.GetOptions).Return(opt, nil)
	clientMock.On("GetRepositoryContents", logMock, info.Owner, info.Repository, info.Path, opt).Return(&nilFileMetadata, dirMetadata, nil).Once()
	clientMock.On("GetRepositoryContents", logMock, info.Owner, info.Repository, filepath, opt).Return(&fileMetadata, nilDirMetadata, nil).Once()
	clientMock.On("IsFileContentType", mock.AnythingOfType("*github.RepositoryContent")).Return(true)

	fileMock := filemock.FileSystemMock{}
	fileMock.On("MakeDirs", strings.TrimSuffix(appconfig.DownloadRoot, "/")).Return(nil)
	fileMock.On("WriteFile", fileutil.BuildPath(appconfig.DownloadRoot, "file.rb"), mock.Anything).Return(nil)

	err, result := resource.DownloadRemoteResource(logMock, fileMock, "")
	clientMock.AssertExpectations(t)
	fileMock.AssertExpectations(t)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, len(result.Files))
	assert.Equal(t, fileutil.BuildPath(appconfig.DownloadRoot, "file.rb"), result.Files[0])
}

func TestGitResource_DownloadFileMissing(t *testing.T) {
	clientMock := githubclientmock.ClientMock{}

	info := GitHubInfo{
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
	fileMock := filemock.FileSystemMock{}

	clientMock.On("ParseGetOptions", logMock, info.GetOptions).Return(opt, nil)
	clientMock.On("GetRepositoryContents", logMock, info.Owner, info.Repository, info.Path, opt).Return(fileMetadata, dirMetadata, nil).Once()
	clientMock.On("IsFileContentType", mock.AnythingOfType("*github.RepositoryContent")).Return(false)

	resource := NewResourceWithMockedClient(&clientMock)

	err, result := resource.DownloadRemoteResource(logMock, fileMock, "")

	clientMock.AssertExpectations(t)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Could not download from GitHub repository")
	assert.Nil(t, result)
}

func TestGitResource_DownloadParseGetOptionFail(t *testing.T) {
	clientMock := githubclientmock.ClientMock{}

	info := GitHubInfo{
		Owner:      "owner",
		Path:       "path/to/file.ext",
		Repository: "repo",
		GetOptions: "",
	}
	opt := &github.RepositoryContentGetOptions{Ref: ""}

	clientMock.On("ParseGetOptions", logMock, info.GetOptions).Return(opt, fmt.Errorf("Option for retrieving GitHub content is empty")).Once()

	resource := NewResourceWithMockedClient(&clientMock)

	fileMock := filemock.FileSystemMock{}
	err, result := resource.DownloadRemoteResource(logMock, fileMock, "")

	clientMock.AssertExpectations(t)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Option for retrieving GitHub content is empty")
	assert.Nil(t, result)
}

func TestGitResource_DownloadGetRepositoryContentsFail(t *testing.T) {
	clientMock := githubclientmock.ClientMock{}

	info := GitHubInfo{
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
	var mockErr error
	mockErr = fmt.Errorf("Rate limit exceeded")

	fileMock := filemock.FileSystemMock{}
	clientMock.On("ParseGetOptions", logMock, info.GetOptions).Return(opt, nil).Once()
	clientMock.On("GetRepositoryContents", logMock, info.Owner, info.Repository, info.Path, opt).Return(fileMetadata, dirMetadata, mockErr).Once()

	resource := NewResourceWithMockedClient(&clientMock)

	err, result := resource.DownloadRemoteResource(logMock, fileMock, "")

	clientMock.AssertExpectations(t)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Rate limit exceeded")
	assert.Nil(t, result)
}

func TestGitResource_ValidateLocationInfoOwner(t *testing.T) {
	locationInfo := `{
		"repository": "repo",
		"path":"path/to/file.rb",
		"getOptions": ""
	}`

	token := TokenMock{}
	resource, _ := NewGitHubResource(logMock, locationInfo, token)
	_, err := resource.ValidateLocationInfo()

	assert.Error(t, err)
	assert.Equal(t, "Owner for GitHub SourceType must be specified", err.Error())
}

func TestGitResource_ValidateLocationInfoRepo(t *testing.T) {
	locationInfo := `{
		"owner": "owner",
		"path":"path/to/file.rb",
		"getOptions": ""
	}`
	token := TokenMock{}
	resource, _ := NewGitHubResource(logMock, locationInfo, token)
	_, err := resource.ValidateLocationInfo()

	assert.Error(t, err)
	assert.Equal(t, "Repository for GitHub SourceType must be specified", err.Error())
}

func TestGitResource_ValidateLocationInfo(t *testing.T) {

	locationInfo := `{
		"owner": "owner",
		"repository": "repo",
		"path":"path/to/file.rb",
		"getOptions": ""
	}`
	token := TokenMock{}
	resource, _ := NewGitHubResource(logMock, locationInfo, token)
	_, err := resource.ValidateLocationInfo()

	assert.NoError(t, err)
}

func TestNewGitResource_parseLocationInfoFail(t *testing.T) {

	token := TokenMock{}
	_, err := NewGitHubResource(nil, "", token)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Source Info could not be unmarshalled for source type GitHub. Please check JSON format of sourceInfo")
}

func TestNewGitResource_GithubTokenInfo(t *testing.T) {
	locationInfo := `{
		"owner": "owner",
		"repository": "repository",
		"path" : "path",
		"tokenInfo" : "ssm:token"
	}`

	token := TokenMock{}
	httpclient := http.Client{}
	token.On("GetOAuthClient", logMock, "ssm:token").Return(&httpclient, nil)

	resource, err := NewGitHubResource(logMock, locationInfo, token)
	assert.NoError(t, err)
	assert.Equal(t, "path", resource.Info.Path)
	assert.Equal(t, "repository", resource.Info.Repository)
	assert.Equal(t, "owner", resource.Info.Owner)
	assert.Equal(t, "ssm:token", resource.Info.TokenInfo)
}

func TestGitResource_DownloadFileToDifferentName(t *testing.T) {
	clientMock := githubclientmock.ClientMock{}

	info := GitHubInfo{
		Owner:      "owner",
		Path:       "path/to/file.ext",
		Repository: "repo",
		GetOptions: "",
	}
	opt := &github.RepositoryContentGetOptions{Ref: ""}

	content := "content"
	file := "file"
	gitpath := "path/to/file.ext"
	fileMetadata := github.RepositoryContent{
		Content: &content,
		Type:    &file,
		Path:    &gitpath,
	}
	var dirMetadata []*github.RepositoryContent
	dirMetadata = nil

	destPath := `/var/temp/my/filename`

	resource := NewResourceWithMockedClient(&clientMock)
	clientMock.On("ParseGetOptions", logMock, info.GetOptions).Return(opt, nil)
	clientMock.On("GetRepositoryContents", logMock, info.Owner, info.Repository, info.Path, opt).Return(&fileMetadata, dirMetadata, nil).Once()
	clientMock.On("IsFileContentType", mock.AnythingOfType("*github.RepositoryContent")).Return(true)

	fileMock := filemock.FileSystemMock{}
	fileMock.On("IsDirectory", destPath).Return(false)
	fileMock.On("Exists", destPath).Return(true)
	fileMock.On("MakeDirs", filepath.Dir(destPath)).Return(nil)
	fileMock.On("WriteFile", destPath, mock.Anything).Return(nil)

	err, result := resource.DownloadRemoteResource(logMock, fileMock, destPath)
	clientMock.AssertExpectations(t)
	fileMock.AssertExpectations(t)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, len(result.Files))
	assert.Equal(t, destPath, result.Files[0])
}

type TokenMock struct {
	mock.Mock
}

func (m TokenMock) GetOAuthClient(log log.T, tokenInfo string) (*http.Client, error) {
	args := m.Called(log, tokenInfo)
	return args.Get(0).(*http.Client), args.Error(1)
}
