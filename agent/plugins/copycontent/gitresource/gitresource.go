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
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/filemanager"
	"github.com/aws/amazon-ssm-agent/agent/githubclient"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/copycontent/gitresource/privategithub"
	"github.com/aws/amazon-ssm-agent/agent/plugins/copycontent/system"

	"errors"
	"fmt"
	"net/http"
	"path/filepath"
)

// GitResource is a struct for the remote resource of type git
type GitResource struct {
	client githubclient.IGitClient
	Info   GitInfo
}

// GitInfo represents the locationInfo type sent by runcommand
type GitInfo struct {
	Owner      string `json:"owner"`
	Repository string `json:"repository"`
	Path       string `json:"path"`
	GetOptions string `json:"getOptions"`
	TokenInfo  string `json:"tokenInfo"`
}

// NewGitResource is a constructor of type GitResource
func NewGitResource(log log.T, info string, token privategithub.PrivateGithubAccess) (git *GitResource, err error) {
	var gitInfo GitInfo
	if gitInfo, err = parseLocationInfo(info); err != nil {
		return nil, err
	}
	// Get the access token from Parameter store - GetAccessToken
	// Create https client - https://github.com/google/go-github#authentication
	var httpClient *http.Client

	if gitInfo.TokenInfo != "" {
		if httpClient, err = token.GetOAuthClient(log, gitInfo.TokenInfo); err != nil {
			return nil, err
		}
	}
	return &GitResource{
		client: githubclient.NewClient(httpClient),
		Info:   gitInfo,
	}, nil
}

// parseLocationInfo unmarshals the information in locationInfo of type GitInfo and returns it
func parseLocationInfo(locationInfo string) (gitInfo GitInfo, err error) {

	if err = jsonutil.Unmarshal(locationInfo, &gitInfo); err != nil {
		return gitInfo, fmt.Errorf("Location Info could not be unmarshalled for location type Git. Please check JSON format of locationInfo - %v", err.Error())
	}

	return gitInfo, nil
}

// Download calls download to pull down files or directory from github
func (git *GitResource) Download(log log.T, filesys filemanager.FileSystem, destinationDir string) (err error) {
	// call download that has object of type GitInfo that keeps changing recursively for directory download
	return git.download(log, filesys, git.Info, destinationDir)
}

//download pulls down either the file or directory specified and stores it on disk
func (git *GitResource) download(log log.T, filesys filemanager.FileSystem, info GitInfo, destinationDir string) (err error) {

	opt, err := git.client.ParseGetOptions(log, info.GetOptions)
	if err != nil {
		return err
	}
	fileMetadata, directoryMetadata, err := git.client.GetRepositoryContents(log, info.Owner, info.Repository, info.Path, opt)
	if err != nil {
		log.Error("Error occured when trying to get repository contents - ", err)
		return err
	}

	// if destination directory is not specified, specifCoy the directory
	if destinationDir == "" {
		destinationDir = appconfig.DownloadRoot
	}

	// If the resource is a directory, the content will be empty and the directoryMetadata is an array of all the files, directories.
	// Each directory type needs to make a recursive call to Download to pull down the files within them.
	if directoryMetadata != nil { // path received was of directory type
		for _, dirContent := range directoryMetadata {

			dirInput := GitInfo{
				Owner:      info.Owner,
				Repository: info.Repository,
				Path:       dirContent.GetPath(),
				GetOptions: info.GetOptions,
			}
			destDir := filepath.Join(destinationDir, filepath.Base(dirContent.GetPath()))
			if err = git.download(log, filesys, dirInput, destDir); err != nil {
				log.Error("Error retrieving file from directory", destinationDir)
				return err
			}
		}
	} else if git.client.IsFileContentType(fileMetadata) { // If content returned is by GetRepositoryContents is a file, it needs to be saved on disk.
		var content string
		if content, err = fileMetadata.GetContent(); err != nil {
			log.Error("File content could not be retrieved - ", err)
			return err
		}
		if filepath.Base(destinationDir) != filepath.Base(fileMetadata.GetPath()) {
			destinationDir = filepath.Join(destinationDir, filepath.Base(fileMetadata.GetPath()))
		}
		if err = system.SaveFileContent(log, filesys, destinationDir, content); err != nil {
			log.Errorf("Error obtaining file content from git file - %v, %v", fileMetadata.GetPath(), err)
			return err
		}
	} else {
		return fmt.Errorf("Could not download from github repository")
	}

	return err
}

// ValidateLocationInfo ensures that the required parameters of Location Info are specified
func (git *GitResource) ValidateLocationInfo() (valid bool, err error) {
	// source not yet supported
	if git.Info.Owner == "" {
		return false, errors.New("Owner for Git LocationType must be specified")
	}

	if git.Info.Repository == "" {
		return false, errors.New("Repository for Git LocationType must be specified")
	}

	return true, nil
}
