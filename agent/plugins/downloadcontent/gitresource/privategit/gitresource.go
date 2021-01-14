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

// Package privategit implements the methods to access resources over Git
package privategit

import (
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/filemanager"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/gitresource"
	"github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/gitresource/privategit/handler"
	"github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/gitresource/privategit/handler/core"
	"github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/remoteresource"
	"github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/types"
	"github.com/aws/amazon-ssm-agent/agent/ssm/ssmparameterresolver"
)

var collectFilesAndRebaseFunction = fileutil.CollectFilesAndRebase
var moveFilesFunction = fileutil.MoveFiles

// GitResource represents a git repository
type GitResource struct {
	Handler handler.IGitHandler
	context context.T
}

// GitInfo defines the accepted SourceInfo attributes and their json definition
type GitInfo struct {
	Repository          string              `json:"repository"`
	PrivateSSHKey       string              `json:"privateSSHKey"`
	SkipHostKeyChecking bool                `json:"skipHostKeyChecking"`
	Username            types.TrimmedString `json:"username"`
	Password            types.TrimmedString `json:"password"`
	GetOptions          string              `json:"getOptions"`
}

// NewGitResource creates a new git resource
func NewGitResource(context context.T, info string, bridge ssmparameterresolver.ISsmParameterResolverBridge) (resource *GitResource, err error) {
	var gitInfo GitInfo

	errorPrefix := "SourceInfo could not be unmarshalled for source type Git"
	if gitInfo, err = parseSourceInfo(info); err != nil {
		return nil, fmt.Errorf("%s: %s", errorPrefix, err.Error())
	}

	getOptions, err := gitresource.ParseCheckoutOptions(context.Log(), gitInfo.GetOptions)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", errorPrefix, err.Error())
	}

	authConfig := handler.GitAuthConfig{
		PrivateSSHKey:       gitInfo.PrivateSSHKey,
		SkipHostKeyChecking: gitInfo.SkipHostKeyChecking,
		Username:            gitInfo.Username,
		Password:            gitInfo.Password,
	}

	gitHandler, err := handler.NewGitHandler(gitInfo.Repository, authConfig, *getOptions, bridge)
	if err != nil {
		return nil, err
	}

	return &GitResource{
		context: context,
		Handler: gitHandler,
	}, nil
}

// DownloadRemoteResource clones a git repository into a specific download path
func (resource *GitResource) DownloadRemoteResource(fileSystem filemanager.FileSystem, downloadPath string) (err error, result *remoteresource.DownloadResult) {
	log := resource.context.Log()
	if downloadPath == "" {
		downloadPath = appconfig.DownloadRoot
	}

	err = fileSystem.MakeDirs(downloadPath)
	if err != nil {
		return fmt.Errorf("Cannot create download path %s: %v", downloadPath, err.Error()), nil
	}

	log.Debug("Destination path to download into - ", downloadPath)

	authMethod, err := resource.Handler.GetAuthMethod(log)
	if err != nil {
		return err, nil
	}

	// Clone first into a random directory to safely collect downloaded files. There may already be other files in the
	// download directory which must be avoided
	tempCloneDir, err := fileSystem.CreateTempDir(downloadPath, "tempCloneDir")
	if err != nil {
		return log.Errorf("Cannot create temporary directory to clone into: %s", err.Error()), nil
	}
	defer func() {
		deleteErr := fileSystem.DeleteDirectory(tempCloneDir)
		if deleteErr != nil {
			log.Warnf("Cannot remove temporary directory: %s", deleteErr.Error())
		}
	}()

	repository, err := resource.Handler.CloneRepository(log, authMethod, tempCloneDir)
	if err != nil {
		return err, nil
	}

	if err := resource.Handler.PerformCheckout(core.NewGitRepository(repository)); err != nil {
		return err, nil
	}

	result = &remoteresource.DownloadResult{}
	result.Files, err = collectFilesAndRebaseFunction(tempCloneDir, downloadPath)
	if err != nil {
		return err, nil
	}

	err = moveFilesFunction(tempCloneDir, downloadPath)
	if err != nil {
		return err, nil
	}

	return nil, result
}

// ValidateLocationInfo validates attribute values of a git resource
func (resource *GitResource) ValidateLocationInfo() (valid bool, err error) {
	return resource.Handler.Validate()
}

// parseSourceInfo unmarshalls the provided SourceInfo input
func parseSourceInfo(sourceInfo string) (gitInfo GitInfo, err error) {
	if err = jsonutil.Unmarshal(sourceInfo, &gitInfo); err != nil {
		return gitInfo, err
	}

	return gitInfo, nil
}
