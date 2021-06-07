/*
 * Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"). You may not
 * use this file except in compliance with the License. A copy of the
 * License is located at
 *
 * http://aws.amazon.com/apache2.0/
 *
 * or in the "license" file accompanying this file. This file is distributed
 * on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing
 * permissions and limitations under the License.
 */

package privategit

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	filemock "github.com/aws/amazon-ssm-agent/agent/fileutil/filemanager/mock"
	"github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/gitresource"
	"github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/gitresource/privategit/handler"
	"github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/gitresource/privategit/handler/core"
	handlermock "github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/gitresource/privategit/handler/mock"
	bridgemock "github.com/aws/amazon-ssm-agent/agent/ssm/ssmparameterresolver/mock"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/stretchr/testify/assert"
)

var contextMock = context.NewMockDefault()
var logMock = contextMock.Log()
var bm = bridgemock.GetSsmParamResolverBridge(map[string]string{})

var downloadRemoteResourceDestPath = os.TempDir()
var downloadRemoteResourceTempCloneDir = filepath.Join(downloadRemoteResourceDestPath, "tempCloneDir")
var downloadRemoteResourceTestFile = filepath.Join(downloadRemoteResourceDestPath, "testFile")

func CollectFilesAndRebaseTest(sourceDir, rebaseToDir string) (files []string, err error) {
	return []string{downloadRemoteResourceTestFile}, nil
}

func MoveFilesTest(sourceDir, destDir string) error {
	return nil
}

func getString(obj interface{}) string {
	return fmt.Sprintf("%v", obj)
}

func TestNewGitResource(t *testing.T) {
	testGitHandler, err := handler.NewGitHandler("private-git-repo", handler.GitAuthConfig{
		PrivateSSHKey:       "private-ssh-key",
		SkipHostKeyChecking: true,
		Username:            "admin",
		Password:            "pwd",
	}, gitresource.CheckoutOptions{
		Branch:   "master",
		CommitID: "",
	}, bm)

	assert.NoError(t, err)

	tests := []struct {
		sourceInfo  string
		gitResource *GitResource
		err         error
	}{
		{
			`{
				"repository": "private-git-repo,
			}`,
			nil,
			errors.New("SourceInfo could not be unmarshalled for source type Git: " +
				"invalid character '\\n' in string literal"),
		},
		{
			`{
					"repository": "private-git-repo",
					"getOptions": "test"
				}`,
			nil,
			errors.New("SourceInfo could not be unmarshalled for source type Git: " +
				"getOptions is not specified in the right format"),
		},
		{
			`{
					"repository": "private-git-repo",
					"getOptions": "test"
				}`,
			nil,
			errors.New("SourceInfo could not be unmarshalled for source type Git: " +
				"getOptions is not specified in the right format"),
		},
		{
			`{
					"repository": "http:// test"
				}`,
			nil,
			errors.New("Invalid repository url format: parse \"http:// test\": invalid character \" \" in host name"),
		},
		{
			`{
				"repository": "private-git-repo",
				"privateSSHKey": "private-ssh-key",
				"skipHostKeyChecking": true,
				"username": "admin",
				"password": "pwd",
				"getOptions": "branch:master"
			}`,
			&GitResource{
				context: contextMock,
				Handler: testGitHandler,
			},
			nil,
		},
	}

	for _, test := range tests {
		gitResource, err := NewGitResource(contextMock, test.sourceInfo, bm)

		if test.err != nil {
			assert.Nil(t, gitResource)
			assert.Error(t, err, getString(test))
			assert.EqualError(t, err, test.err.Error())
		} else {
			assert.NoError(t, err, getString(test))
			assert.Equal(t, test.gitResource, gitResource, getString(test))
		}
	}
}

func TestNewGitResource_parseSourceInfo(t *testing.T) {
	tests := []struct {
		sourceInfo string
		gitInfo    GitInfo
		err        error
	}{
		{
			`{
				"repository": "private-git-repo",
				"privateSSHKey": "--key--",
				"skipHostKeyChecking": true,
				"username": "admin",
				"password": "pwd",
				"getOptions": "branch:master"
			}`,
			GitInfo{
				Repository:          "private-git-repo",
				PrivateSSHKey:       "--key--",
				SkipHostKeyChecking: true,
				Username:            "admin",
				Password:            "pwd",
				GetOptions:          "branch:master",
			},
			nil,
		},
		{
			`{
				"repository": "git://"
			}`,
			GitInfo{
				Repository:          "git://",
				SkipHostKeyChecking: false,
			},
			nil,
		},
		{
			fmt.Sprintf(`{
				"repository": "git://",
				"privateSSHKey": "%s"

			}`, "private-ssh-key"),
			GitInfo{
				Repository:          "git://",
				PrivateSSHKey:       "private-ssh-key",
				SkipHostKeyChecking: false,
			},
			nil,
		},
		{
			`{
				"repository": "git://
			}`,
			GitInfo{},
			errors.New("invalid character '\\n' in string literal"),
		},
	}

	for _, test := range tests {
		gitInfo, err := parseSourceInfo(test.sourceInfo)

		if test.err != nil {
			assert.Equal(t, GitInfo{}, gitInfo)
			assert.Error(t, err, getString(test))
			assert.EqualError(t, err, test.err.Error())
		} else {
			assert.NoError(t, err, getString(test))
			assert.Equal(t, test.gitInfo, gitInfo, getString(test))
		}
	}
}

func TestGitResource_ValidateLocationInfo(t *testing.T) {
	gitHandlerMock := handlermock.GitHandlerMock{}
	gitHandlerMock.On("Validate").Return(true, nil).Once()

	resource := GitResource{
		context: contextMock,
		Handler: &gitHandlerMock,
	}

	_, _ = resource.ValidateLocationInfo()
	gitHandlerMock.AssertExpectations(t)
}

func TestGitResource_DownloadRemoteResource(t *testing.T) {
	authMethod := &http.BasicAuth{}
	repository := &gogit.Repository{}

	fileSysMock := filemock.FileSystemMock{}
	fileSysMock.On("MakeDirs", os.TempDir()).Return(nil).Once()
	fileSysMock.On("CreateTempDir", os.TempDir(), "tempCloneDir").Return(downloadRemoteResourceTempCloneDir, nil).Once()
	fileSysMock.On("DeleteDirectory", downloadRemoteResourceTempCloneDir).Return(nil).Once()

	collectFilesAndRebaseFunction = CollectFilesAndRebaseTest
	moveFilesFunction = MoveFilesTest

	gitHandlerMock := handlermock.GitHandlerMock{}
	gitHandlerMock.On("GetAuthMethod", logMock).Return(authMethod, nil).Once()
	gitHandlerMock.On("CloneRepository", logMock, authMethod, downloadRemoteResourceTempCloneDir).Return(repository, nil).Once()
	gitHandlerMock.On("PerformCheckout", core.NewGitRepository(repository)).Return(nil).Once()

	resource := GitResource{
		context: contextMock,
		Handler: &gitHandlerMock,
	}

	err, result := resource.DownloadRemoteResource(fileSysMock, downloadRemoteResourceDestPath)

	assert.NoError(t, err)
	assert.Equal(t, []string{downloadRemoteResourceTestFile}, result.Files)

	gitHandlerMock.AssertExpectations(t)
	fileSysMock.AssertExpectations(t)

	collectFilesAndRebaseFunction = fileutil.CollectFilesAndRebase
	moveFilesFunction = fileutil.MoveFiles
}
