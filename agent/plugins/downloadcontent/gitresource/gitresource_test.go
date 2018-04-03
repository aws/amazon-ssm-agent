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
	filemock "github.com/aws/amazon-ssm-agent/agent/fileutil/filemanager/mock"
	githubclientmock "github.com/aws/amazon-ssm-agent/agent/githubclient/mock"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/go-github/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"fmt"
	"net/http"
	"path/filepath"
	"strings"
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

func TestGitResource_DownloadFile(t *testing.T) {
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
	gitpath := "path/to/file.ext"
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

	fileMock.On("IsDirectory", appconfig.DownloadRoot).Return(true)
	fileMock.On("Exists", appconfig.DownloadRoot).Return(true)
	fileMock.On("MakeDirs", strings.TrimSuffix(appconfig.DownloadRoot, "/")).Return(nil)
	fileMock.On("WriteFile", filepath.Join(appconfig.DownloadRoot, "file.ext"), mock.Anything).Return(nil)

	err, result := gitResource.DownloadRemoteResource(logMock, fileMock, "")
	clientMock.AssertExpectations(t)
	fileMock.AssertExpectations(t)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, len(result.Files))
	assert.Equal(t, fileutil.BuildPath(appconfig.DownloadRoot, "file.ext"), result.Files[0])
}

func TestGitResource_DownloadDirectory(t *testing.T) {
	clientMock := githubclientmock.ClientMock{}

	gitInfo := GitInfo{
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

	gitResource := &GitResource{
		client: &clientMock,
		Info:   gitInfo,
	}
	clientMock.On("ParseGetOptions", logMock, gitInfo.GetOptions).Return(opt, nil)
	clientMock.On("GetRepositoryContents", logMock, gitInfo.Owner, gitInfo.Repository, gitInfo.Path, opt).Return(&nilFileMetadata, dirMetadata, nil).Once()
	clientMock.On("GetRepositoryContents", logMock, gitInfo.Owner, gitInfo.Repository, filepath, opt).Return(&fileMetadata, nilDirMetadata, nil).Once()
	clientMock.On("IsFileContentType", mock.AnythingOfType("*github.RepositoryContent")).Return(true)

	fileMock := filemock.FileSystemMock{}
	fileMock.On("MakeDirs", strings.TrimSuffix(appconfig.DownloadRoot, "/")).Return(nil)
	fileMock.On("WriteFile", fileutil.BuildPath(appconfig.DownloadRoot, "file.rb"), mock.Anything).Return(nil)

	err, result := gitResource.DownloadRemoteResource(logMock, fileMock, "")
	clientMock.AssertExpectations(t)
	fileMock.AssertExpectations(t)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, len(result.Files))
	assert.Equal(t, fileutil.BuildPath(appconfig.DownloadRoot, "file.rb"), result.Files[0])
}

func TestGitResource_DownloadFileMissing(t *testing.T) {
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
	fileMock := filemock.FileSystemMock{}

	clientMock.On("ParseGetOptions", logMock, gitInfo.GetOptions).Return(opt, nil)
	clientMock.On("GetRepositoryContents", logMock, gitInfo.Owner, gitInfo.Repository, gitInfo.Path, opt).Return(fileMetadata, dirMetadata, nil).Once()
	clientMock.On("IsFileContentType", mock.AnythingOfType("*github.RepositoryContent")).Return(false)

	gitResource := NewResourceWithMockedClient(&clientMock)

	err, result := gitResource.DownloadRemoteResource(logMock, fileMock, "")

	clientMock.AssertExpectations(t)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Could not download from GitHub repository")
	assert.Nil(t, result)
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

	clientMock.On("ParseGetOptions", logMock, gitInfo.GetOptions).Return(opt, fmt.Errorf("Option for retrieving GitHub content is empty")).Once()

	gitResource := NewResourceWithMockedClient(&clientMock)

	fileMock := filemock.FileSystemMock{}
	err, result := gitResource.DownloadRemoteResource(logMock, fileMock, "")

	clientMock.AssertExpectations(t)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Option for retrieving GitHub content is empty")
	assert.Nil(t, result)
}

func TestGitResource_DownloadGetRepositoryContentsFail(t *testing.T) {
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
	var mockErr error
	mockErr = fmt.Errorf("Rate limit exceeded")

	fileMock := filemock.FileSystemMock{}
	clientMock.On("ParseGetOptions", logMock, gitInfo.GetOptions).Return(opt, nil).Once()
	clientMock.On("GetRepositoryContents", logMock, gitInfo.Owner, gitInfo.Repository, gitInfo.Path, opt).Return(fileMetadata, dirMetadata, mockErr).Once()

	gitResource := NewResourceWithMockedClient(&clientMock)

	err, result := gitResource.DownloadRemoteResource(logMock, fileMock, "")

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
	gitresource, _ := NewGitResource(logMock, locationInfo, token)
	_, err := gitresource.ValidateLocationInfo()

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
	gitresource, _ := NewGitResource(logMock, locationInfo, token)
	_, err := gitresource.ValidateLocationInfo()

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
	gitresource, _ := NewGitResource(logMock, locationInfo, token)
	_, err := gitresource.ValidateLocationInfo()

	assert.NoError(t, err)
}

func TestNewGitResource_parseLocationInfoFail(t *testing.T) {

	token := TokenMock{}
	_, err := NewGitResource(nil, "", token)

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

	gitresource, err := NewGitResource(logMock, locationInfo, token)
	assert.NoError(t, err)
	assert.Equal(t, "path", gitresource.Info.Path)
	assert.Equal(t, "repository", gitresource.Info.Repository)
	assert.Equal(t, "owner", gitresource.Info.Owner)
	assert.Equal(t, "ssm:token", gitresource.Info.TokenInfo)
}

func TestGitResource_DownloadFileToDifferentName(t *testing.T) {
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
	gitpath := "path/to/file.ext"
	fileMetadata := github.RepositoryContent{
		Content: &content,
		Type:    &file,
		Path:    &gitpath,
	}
	var dirMetadata []*github.RepositoryContent
	dirMetadata = nil

	destPath := `/var/temp/my/filename`

	gitResource := NewResourceWithMockedClient(&clientMock)
	clientMock.On("ParseGetOptions", logMock, gitInfo.GetOptions).Return(opt, nil)
	clientMock.On("GetRepositoryContents", logMock, gitInfo.Owner, gitInfo.Repository, gitInfo.Path, opt).Return(&fileMetadata, dirMetadata, nil).Once()
	clientMock.On("IsFileContentType", mock.AnythingOfType("*github.RepositoryContent")).Return(true)

	fileMock := filemock.FileSystemMock{}
	fileMock.On("IsDirectory", destPath).Return(false)
	fileMock.On("Exists", destPath).Return(true)
	fileMock.On("MakeDirs", filepath.Dir(destPath)).Return(nil)
	fileMock.On("WriteFile", destPath, mock.Anything).Return(nil)

	err, result := gitResource.DownloadRemoteResource(logMock, fileMock, destPath)
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
