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

// Package handler provides methods for accessing git repositories over SSH or HTTP(s)
package handler

import (
	"errors"
	"fmt"
	"os"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/gitresource"
	"github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/gitresource/privategit/handler/core"
	"github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/types"
	"github.com/aws/amazon-ssm-agent/agent/ssm/ssmparameterresolver"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"golang.org/x/crypto/ssh"
)

var plainCloneMethod = gogit.PlainClone

// GitAuthConfig defines the attributes used to perform authentication over SSH or HTTP
type GitAuthConfig struct {
	PrivateSSHKey       string
	SkipHostKeyChecking bool
	Username            types.TrimmedString
	Password            types.TrimmedString
}

// IGitHandler defines methods to interact with git repositories
type IGitHandler interface {
	GetAuthMethod(log log.T) (transport.AuthMethod, error)
	CloneRepository(log log.T, authMethod transport.AuthMethod, destPath string) (repository *gogit.Repository, err error)
	PerformCheckout(repository core.IGitRepository) error
	Validate() (bool, error)
}

// gitHandler is used to manipulate and interact with specific git repositories
type gitHandler struct {
	repositoryURL              types.TrimmedString
	parsedRepositoryURL        transport.Endpoint
	authConfig                 GitAuthConfig
	getOptions                 gitresource.CheckoutOptions
	ssmParameterResolverBridge ssmparameterresolver.ISsmParameterResolverBridge
}

// NewGitHandler creates a new git handler object
func NewGitHandler(
	repository string,
	authConfig GitAuthConfig,
	options gitresource.CheckoutOptions,
	bridge ssmparameterresolver.ISsmParameterResolverBridge) (IGitHandler, error) {
	parsedURL, err := transport.NewEndpoint(repository)
	if err != nil {
		return nil, fmt.Errorf("Invalid repository url format: %s", err.Error())
	}

	return &gitHandler{
		repositoryURL:              types.TrimmedString(repository),
		parsedRepositoryURL:        *parsedURL,
		authConfig:                 authConfig,
		getOptions:                 options,
		ssmParameterResolverBridge: bridge,
	}, nil
}

// GetAuthMethod identifies the required auth method based on the given input
func (handler *gitHandler) GetAuthMethod(log log.T) (transport.AuthMethod, error) {
	if handler.authConfig.PrivateSSHKey != "" && handler.isGitTypeRepositoryURL() {
		return handler.getPublicKeyAuthMethod(log)
	} else if handler.authConfig.Username != "" && handler.authConfig.Password != "" && handler.isHTTPTypeRepositoryURL() {
		return handler.getHttpBasicAuthMethod(log)
	}

	return nil, nil
}

// CloneRepository performs the git clone operation
func (handler *gitHandler) CloneRepository(log log.T, authMethod transport.AuthMethod, destPath string) (repository *gogit.Repository, err error) {
	cloneOptions := gogit.CloneOptions{
		URL:      handler.repositoryURL.Val(),
		Progress: os.Stdout,
		Auth:     authMethod,
	}

	repository, err = plainCloneMethod(destPath, false, &cloneOptions)
	if err != nil {
		log.Errorf(err.Error())
		if err.Error() == "ssh: handshake failed: knownhosts: key is unknown" {
			err = fmt.Errorf("Unknown host key. Please add remote host key known_hosts file or set SourceInfo " +
				"'skipHostKeyChecking' parameter to true in order to skip host key validation")
		}

		return nil, fmt.Errorf("Cannot clone repository %s: %s", handler.repositoryURL.Val(), err.Error())
	}

	return repository, nil
}

// PerformCheckout performs the git checkout operation
func (handler *gitHandler) PerformCheckout(repository core.IGitRepository) error {
	worktree, err := repository.Worktree()
	if err != nil {
		return fmt.Errorf("Cannot retrieve repository's working tree: %v", err.Error())
	}

	var performCheckout = false
	var checkoutOptions gogit.CheckoutOptions

	// Either commit id or the branch is allowed. The validation is done by ParseCheckoutOptions()
	if handler.getOptions.CommitID != "" {
		performCheckout = true
		checkoutOptions.Hash = plumbing.NewHash(handler.getOptions.CommitID.Val())
	}

	if handler.getOptions.Branch != "" {
		performCheckout = true
		checkoutOptions.Branch = plumbing.ReferenceName(handler.getOptions.Branch.Val())
	}

	if performCheckout {
		if err = worktree.Checkout(&checkoutOptions); err != nil {
			return fmt.Errorf("Cannot checkout: %v", err.Error())
		}
	}

	return nil
}

// Validate validates handler's attributes values
func (handler *gitHandler) Validate() (bool, error) {
	if handler.repositoryURL == "" {
		return false, errors.New("Repository for Git resources type must be specified")
	}

	if handler.isGitTypeRepositoryURL() && (handler.authConfig.Username != "" || handler.authConfig.Password != "") {
		return false, errors.New("Neither Username nor Password must not be provided for Git type repository URL. " +
			"Private SSH key for authentication")
	}

	if handler.isHTTPTypeRepositoryURL() && handler.authConfig.PrivateSSHKey != "" {
		return false, errors.New("Private SSH key must not be provided for HTTP type repository URL. " +
			"Username and Password is required for authentication")
	}

	return true, nil
}

// isGitTypeRepositoryURL determines whether the provided repository url if of type SSH
func (handler *gitHandler) isGitTypeRepositoryURL() bool {
	return !handler.isHTTPTypeRepositoryURL()
}

// isGitTypeRepositoryURL determines whether the provided repository url if of type HTTP(s)
func (handler *gitHandler) isHTTPTypeRepositoryURL() bool {
	return handler.parsedRepositoryURL.Protocol == "http" || handler.parsedRepositoryURL.Protocol == "https"
}

// getHttpBasicAuthMethod returns the HTTP basic authentication object
func (handler *gitHandler) getHttpBasicAuthMethod(log log.T) (transport.AuthMethod, error) {
	var err error

	var username = handler.authConfig.Username.Val()
	if handler.ssmParameterResolverBridge.IsValidParameterStoreReference(username) {
		username, err = handler.ssmParameterResolverBridge.GetParameterFromSsmParameterStore(log, username)
		if err != nil {
			return nil, err
		}
	}

	var password = handler.authConfig.Password.Val()
	if handler.ssmParameterResolverBridge.IsValidParameterStoreReference(password) {
		password, err = handler.ssmParameterResolverBridge.GetParameterFromSsmParameterStore(log, password)
		if err != nil {
			return nil, err
		}
	}

	return &http.BasicAuth{
		Username: username,
		Password: password,
	}, nil
}

// getHttpBasicAuthMethod returns the public key type authentication object
func (handler *gitHandler) getPublicKeyAuthMethod(log log.T) (transport.AuthMethod, error) {
	var err error

	var privateSSHKey = handler.authConfig.PrivateSSHKey
	if handler.ssmParameterResolverBridge.IsValidParameterStoreReference(privateSSHKey) {
		privateSSHKey, err = handler.ssmParameterResolverBridge.GetParameterFromSsmParameterStore(log, privateSSHKey)
		if err != nil {
			return nil, err
		}
	}

	user := handler.parsedRepositoryURL.User
	if user == "" {
		user = "git"
	}

	publicKeysAuth, err := gitssh.NewPublicKeys(user, []byte(privateSSHKey), "")
	if err != nil {
		return nil, fmt.Errorf("Cannot authenticate: %s", err.Error())
	}

	if handler.authConfig.SkipHostKeyChecking {
		publicKeysAuth.HostKeyCallback = ssh.InsecureIgnoreHostKey()
	}

	return publicKeysAuth, nil
}
