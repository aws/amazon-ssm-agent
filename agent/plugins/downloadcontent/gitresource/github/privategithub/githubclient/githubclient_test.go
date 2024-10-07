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

// Package githubclient contains methods for interacting with git
package githubclient

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/google/go-github/v61/github"
	"github.com/stretchr/testify/assert"
)

var logMock = log.NewMockLog()

//TODO: Add tests for GetRepositoryContent

func Test_isFileContentTypeTrue(t *testing.T) {
	file := contentTypeFile
	fileMetada := github.RepositoryContent{
		Type: &file,
	}
	client := NewClient(nil)

	isFile := client.IsFileContentType(&fileMetada)

	assert.True(t, isFile)
}

func Test_isFileContentTypeFalse(t *testing.T) {
	dir := contentTypeDirectory
	dirMetada := github.RepositoryContent{
		Type: &dir,
	}
	client := NewClient(nil)
	isFile := client.IsFileContentType(&dirMetada)

	assert.False(t, isFile)
}

func Test_isFileContentTypeNil(t *testing.T) {
	var fileMetadata *github.RepositoryContent
	fileMetadata = nil
	client := NewClient(nil)
	isFile := client.IsFileContentType(fileMetadata)

	assert.False(t, isFile)
}

func TestGetRepositoryContents_non_nil_success(t *testing.T) {
	owner := "owner"
	repo := "repo"
	path := "path1"
	httpClient := &http.Client{}
	httpClient.Transport = roundTripFunc(func(req *http.Request) *http.Response {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioutil.NopCloser(bytes.NewReader([]byte("[\n  {\n    \"type\": \"file\",\n    \"size\": 625,\n    \"name\": \"octokit.rb\",\n    \"path\": \"lib/octokit.rb\",\n    \"sha\": \"fff6fe3a23bf1c8ea0692b4a883af99bee26fd3b\",\n    \"url\": \"https://api.github.com/repos/octokit/octokit.rb/contents/lib/octokit.rb\",\n    \"git_url\": \"https://api.github.com/repos/octokit/octokit.rb/git/blobs/fff6fe3a23bf1c8ea0692b4a883af99bee26fd3b\",\n    \"html_url\": \"https://github.com/octokit/octokit.rb/blob/master/lib/octokit.rb\",\n    \"download_url\": \"https://raw.githubusercontent.com/octokit/octokit.rb/master/lib/octokit.rb\",\n    \"_links\": {\n      \"self\": \"https://api.github.com/repos/octokit/octokit.rb/contents/lib/octokit.rb\",\n      \"git\": \"https://api.github.com/repos/octokit/octokit.rb/git/blobs/fff6fe3a23bf1c8ea0692b4a883af99bee26fd3b\",\n      \"html\": \"https://github.com/octokit/octokit.rb/blob/master/lib/octokit.rb\"\n    }\n  },\n  {\n    \"type\": \"dir\",\n    \"size\": 0,\n    \"name\": \"octokit\",\n    \"path\": \"lib/octokit\",\n    \"sha\": \"a84d88e7554fc1fa21bcbc4efae3c782a70d2b9d\",\n    \"url\": \"https://api.github.com/repos/octokit/octokit.rb/contents/lib/octokit\",\n    \"git_url\": \"https://api.github.com/repos/octokit/octokit.rb/git/trees/a84d88e7554fc1fa21bcbc4efae3c782a70d2b9d\",\n    \"html_url\": \"https://github.com/octokit/octokit.rb/tree/master/lib/octokit\",\n    \"download_url\": null,\n    \"_links\": {\n      \"self\": \"https://api.github.com/repos/octokit/octokit.rb/contents/lib/octokit\",\n      \"git\": \"https://api.github.com/repos/octokit/octokit.rb/git/trees/a84d88e7554fc1fa21bcbc4efae3c782a70d2b9d\",\n      \"html\": \"https://github.com/octokit/octokit.rb/tree/master/lib/octokit\"\n    }\n  }\n]"))),
			Header:     http.Header{},
		}
	})
	client := NewClient(httpClient)
	mockLog := log.NewMockLog()
	opt := &github.RepositoryContentGetOptions{Ref: ""}
	_, dirContent, err := client.GetRepositoryContents(mockLog, owner, repo, path, opt)
	assert.Equal(t, 2, len(dirContent))
	assert.Nil(t, err)
}

func TestGetRepositoryContents_nil_fail(t *testing.T) {
	owner := "owner"
	repo := "repo"
	path := "path1"
	httpClient := &http.Client{}
	httpClient.Transport = roundTripFunc(func(req *http.Request) *http.Response {
		return nil
	})
	client := NewClient(httpClient)
	mockLog := log.NewMockLog()
	opt := &github.RepositoryContentGetOptions{Ref: ""}
	_, dirContent, err := client.GetRepositoryContents(mockLog, owner, repo, path, opt)
	assert.Equal(t, 0, len(dirContent))
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "no response received when calling git API")
}

type roundTripFunc func(req *http.Request) *http.Response

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}
