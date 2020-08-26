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

// Package mock_githubclient contains methods to mock githubclient package
package mock_githubclient

import (
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/go-github/github"
	"github.com/stretchr/testify/mock"

	"net/http"
)

type ClientMock struct {
	mock.Mock
}

func (git_mock *ClientMock) GetRepositoryContents(log log.T, owner, repo, path string, opt *github.RepositoryContentGetOptions) (fileContent *github.RepositoryContent, directoryContent []*github.RepositoryContent, err error) {
	args := git_mock.Called(log, owner, repo, path, opt)
	return args.Get(0).(*github.RepositoryContent), args.Get(1).([]*github.RepositoryContent), args.Error(2)
}

func (git_mock *ClientMock) ParseGetOptions(log log.T, getOptions string) (*github.RepositoryContentGetOptions, error) {
	args := git_mock.Called(log, getOptions)
	return args.Get(0).(*github.RepositoryContentGetOptions), args.Error(1)
}

func (git_mock *ClientMock) IsFileContentType(content *github.RepositoryContent) bool {
	args := git_mock.Called(content)
	return args.Bool(0)
}

type OAuthClientMock struct {
	mock.Mock
}

func (git_mock OAuthClientMock) GetGithubOauthClient(token string) *http.Client {
	args := git_mock.Called(token)
	return args.Get(0).(*http.Client)
}
