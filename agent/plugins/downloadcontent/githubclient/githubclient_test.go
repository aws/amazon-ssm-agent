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
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/go-github/github"
	"github.com/stretchr/testify/assert"

	"testing"
)

var logMock = log.NewMockLog()

//TODO: Add tests for GetRepositoryContent

func TestGitClient_ParseGetOptions(t *testing.T) {
	client := NewClient(nil)
	expected := &github.RepositoryContentGetOptions{
		Ref: "blah",
	}

	opt, err := client.ParseGetOptions(logMock, "branch:blah")

	assert.Equal(t, expected, opt)
	assert.NoError(t, err)

}

func TestGitClient_ParseGetOptionsNoGetOptions(t *testing.T) {
	client := NewClient(nil)
	expected := &github.RepositoryContentGetOptions{
		Ref: "master",
	}

	opt, err := client.ParseGetOptions(logMock, "")

	assert.Equal(t, expected, opt)
	assert.NoError(t, err)

}

func TestGitClient_ParseGetOptionsInvalidFormat(t *testing.T) {
	client := NewClient(nil)
	var expected *github.RepositoryContentGetOptions
	expected = nil

	opt, err := client.ParseGetOptions(logMock, "commit:qwertaydhfdig")

	assert.Error(t, err)
	assert.Equal(t, expected, opt)

}

func TestGitClient_ParseGetOptionsTooManyOptions(t *testing.T) {
	client := NewClient(nil)
	var expected *github.RepositoryContentGetOptions
	expected = nil

	opt, err := client.ParseGetOptions(logMock, "branch:master; commitID:qwertaydhfdig")

	assert.Error(t, err)
	assert.Equal(t, expected, opt)

}

func TestGitClient_ParseGetOptionsInvalidOptions(t *testing.T) {
	client := NewClient(nil)
	var expected *github.RepositoryContentGetOptions
	expected = nil

	opt, err := client.ParseGetOptions(logMock, "branch")

	assert.Error(t, err)
	assert.Equal(t, expected, opt)

}

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
