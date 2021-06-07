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

package handler

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/gitresource"
	gitcoremock "github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/gitresource/privategit/handler/core/mock"
	bridgemock "github.com/aws/amazon-ssm-agent/agent/ssm/ssmparameterresolver/mock"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/ssh"
)

var logMock = log.NewMockLog()

func GeneratePrivateKey() string {
	var key, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		fmt.Printf("Got an error while generating the private key: %v", err)
		return ""
	}

	return string(pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(key),
		},
	))
}

var privateSSHKey = GeneratePrivateKey()
var parameterStoreParameters = map[string]string{
	"{{ssm-secure:username}}":      "admin",
	"{{ssm-secure:password}}":      "pwd",
	"{{ssm-secure:privateSSHKey}}": privateSSHKey,
}

func PlainClone(path string, isBare bool, o *gogit.CloneOptions) (*gogit.Repository, error) {
	switch path {
	case "unknown-host":
		return nil, fmt.Errorf("ssh: handshake failed: knownhosts: key is unknown")
	case "error":
		return nil, fmt.Errorf("random err")
	default:
		return &gogit.Repository{
			Storer: nil,
		}, nil
	}
}

func getTestPublicKeysAuth(user, sshKey string, hostKeyCallback ssh.HostKeyCallback) *gitssh.PublicKeys {
	publicKeysAuthMethod, err := gitssh.NewPublicKeys(user, []byte(sshKey), "")

	if err != nil {
		fmt.Printf(err.Error())
		return nil
	}

	publicKeysAuthMethod.HostKeyCallback = hostKeyCallback
	return publicKeysAuthMethod
}

func getString(obj interface{}) string {
	return fmt.Sprintf("%v", obj)
}

func TestNewGitHandlerURLParsing(t *testing.T) {
	tests := []struct {
		repository string
		url        *transport.Endpoint
		err        error
	}{
		{
			"",
			&transport.Endpoint{
				Protocol: "file",
			},
			nil,
		},
		{
			"http://example.com",
			&transport.Endpoint{
				Protocol: "http",
				Host:     "example.com",
			},
			nil,
		},
		{
			"ssh://git@example.com",
			&transport.Endpoint{
				Protocol: "ssh",
				Host:     "example.com",
				User:     "git",
			},
			nil,
		},
		{
			"contributor@github.com:User/test-repository.git",
			&transport.Endpoint{
				Protocol: "ssh",
				Host:     "github.com",
				User:     "contributor",
				Port:     22,
				Path:     "User/test-repository.git",
			},
			nil,
		},
		{
			"http:// example.com",
			nil,
			errors.New("Invalid repository url format: parse \"http:// example.com\": invalid character \" \" in host name"),
		},
	}

	for _, test := range tests {
		handler, err := NewGitHandler(
			test.repository,
			GitAuthConfig{},
			gitresource.CheckoutOptions{},
			bridgemock.GetSsmParamResolverBridge(parameterStoreParameters),
		)

		if test.err != nil {
			assert.Error(t, err, getString(test))
			assert.EqualError(t, err, test.err.Error())
		} else {
			gitHandler := handler.(*gitHandler)

			assert.NoError(t, err, getString(test))
			assert.Equal(t, test.url, &gitHandler.parsedRepositoryURL)
		}
	}

}

func TestNewGitHandler_isGitTypeRepositoryURL(t *testing.T) {
	tests := []struct {
		handler   gitHandler
		isGitType bool
	}{
		{
			gitHandler{
				parsedRepositoryURL: transport.Endpoint{
					Protocol: "https",
				},
			},
			false,
		},
		{
			gitHandler{
				parsedRepositoryURL: transport.Endpoint{
					Protocol: "http",
				},
			},
			false,
		},
		{
			gitHandler{
				parsedRepositoryURL: transport.Endpoint{
					Protocol: "ssh",
				},
			},
			true,
		},
		{
			gitHandler{
				parsedRepositoryURL: transport.Endpoint{
					Protocol: "bla",
				},
			},
			true,
		},
		{
			gitHandler{
				parsedRepositoryURL: transport.Endpoint{
					Protocol: "",
				},
			},
			true,
		},
	}

	for _, test := range tests {
		isGitType := test.handler.isGitTypeRepositoryURL()
		assert.Equal(t, test.isGitType, isGitType, test)
	}
}

func TestNewGitHandler_isHTTPTypeRepositoryURL(t *testing.T) {
	tests := []struct {
		handler    gitHandler
		isHTTPType bool
	}{
		{
			gitHandler{
				parsedRepositoryURL: transport.Endpoint{
					Protocol: "https",
				},
			},
			true,
		},
		{
			gitHandler{
				parsedRepositoryURL: transport.Endpoint{
					Protocol: "http",
				},
			},
			true,
		},
		{
			gitHandler{
				parsedRepositoryURL: transport.Endpoint{
					Protocol: "bla",
				},
			},
			false,
		},
		{
			gitHandler{
				parsedRepositoryURL: transport.Endpoint{
					Protocol: "",
				},
			},
			false,
		},
	}

	for _, test := range tests {
		isHTTPType := test.handler.isHTTPTypeRepositoryURL()
		assert.Equal(t, test.isHTTPType, isHTTPType, getString(test))
	}
}

func TestGitHandler_ValidateIsValid(t *testing.T) {
	handlers := []gitHandler{
		{
			repositoryURL: "https",
			parsedRepositoryURL: transport.Endpoint{
				Protocol: "https",
			},
		},
		{
			repositoryURL: "http",
			parsedRepositoryURL: transport.Endpoint{
				Protocol: "http",
			},
			authConfig: GitAuthConfig{
				Username: "test",
				Password: "123",
			},
		},
		{
			repositoryURL: "https",
			parsedRepositoryURL: transport.Endpoint{
				Protocol: "https",
			},
			authConfig: GitAuthConfig{
				Password: "123",
			},
		},
		{
			repositoryURL: "http",
			parsedRepositoryURL: transport.Endpoint{
				Protocol: "http",
			},
			authConfig: GitAuthConfig{
				Username: "123",
			},
		},
		{
			repositoryURL: "ssh",
			parsedRepositoryURL: transport.Endpoint{
				Protocol: "ssh",
			},
			authConfig: GitAuthConfig{
				PrivateSSHKey: "test",
			},
		},
	}

	for _, handler := range handlers {
		isValid, err := handler.Validate()
		assert.NoError(t, err, getString(handler))
		assert.True(t, isValid, getString(handler))
	}
}

func TestGitHandler_ValidateIsInvalid(t *testing.T) {

	gitUrlWithUserOrPwdError := errors.New("Neither Username nor Password must not be provided for " +
		"Git type repository URL. Private SSH key for authentication")

	httpUrlWithPrivateSSHKey := errors.New("Private SSH key must not be provided for HTTP type repository URL. " +
		"Username and Password is required for authentication")

	tests := []struct {
		handler gitHandler
		err     error
	}{
		{
			gitHandler{
				repositoryURL: "http",
				parsedRepositoryURL: transport.Endpoint{
					Protocol: "http",
				},
				authConfig: GitAuthConfig{
					PrivateSSHKey: "--key--",
				},
			},
			httpUrlWithPrivateSSHKey,
		},
		{
			gitHandler{
				repositoryURL: "ssh",
				parsedRepositoryURL: transport.Endpoint{
					Protocol: "ssh",
				},
				authConfig: GitAuthConfig{
					Username: "user",
				},
			},
			gitUrlWithUserOrPwdError,
		},
		{
			gitHandler{
				repositoryURL: "sjdhakjf",
				parsedRepositoryURL: transport.Endpoint{
					Protocol: "abc",
				},
				authConfig: GitAuthConfig{
					Password: "123",
				},
			},
			gitUrlWithUserOrPwdError,
		},
		{
			gitHandler{
				repositoryURL: "jkfdgnd",
				parsedRepositoryURL: transport.Endpoint{
					Protocol: "",
				},
				authConfig: GitAuthConfig{
					Username: "user",
					Password: "123",
				},
			},
			gitUrlWithUserOrPwdError,
		},
		{
			gitHandler{
				repositoryURL: "",
			},
			errors.New("Repository for Git resources type must be specified"),
		},
	}

	for _, test := range tests {
		isValid, err := test.handler.Validate()
		assert.False(t, isValid, getString(test))
		assert.Error(t, err, getString(test))
		assert.EqualError(t, err, test.err.Error(), getString(test))

	}
}

func TestGitHandler_GetAuthMethod(t *testing.T) {
	bridge := bridgemock.GetSsmParamResolverBridge(parameterStoreParameters)

	tests := []struct {
		handler    gitHandler
		authMethod transport.AuthMethod
		err        error
	}{
		{
			gitHandler{
				parsedRepositoryURL: transport.Endpoint{
					Protocol: "http",
				},
				ssmParameterResolverBridge: bridge,
			},
			nil,
			nil,
		},
		{
			gitHandler{
				parsedRepositoryURL: transport.Endpoint{
					Protocol: "http",
				},
				authConfig: GitAuthConfig{
					Username: "user",
					Password: "pwd",
				},
				ssmParameterResolverBridge: bridge,
			},
			&http.BasicAuth{},
			nil,
		},
		{
			gitHandler{
				parsedRepositoryURL: transport.Endpoint{
					Protocol: "ssh",
				},
				authConfig: GitAuthConfig{
					PrivateSSHKey: privateSSHKey,
				},
				ssmParameterResolverBridge: bridge,
			},
			&gitssh.PublicKeys{},
			nil,
		},
	}

	for _, test := range tests {
		authMethod, err := test.handler.GetAuthMethod(logMock)

		if test.err != nil {
			assert.Error(t, err, getString(test))
			assert.EqualError(t, test.err, err.Error(), getString(test))
			assert.Nil(t, authMethod, getString(test))
		} else {
			assert.NoError(t, err, getString(test))

			if test.authMethod == nil && test.authMethod == authMethod {
				assert.Nil(t, authMethod, getString(test))
			} else {
				assert.Equal(t, test.authMethod.Name(), authMethod.Name(), getString(test))
			}
		}
	}
}

func TestGitHandler_getAuthMethodPublicKey(t *testing.T) {
	bridge := bridgemock.GetSsmParamResolverBridge(parameterStoreParameters)

	tests := []struct {
		handler    gitHandler
		authMethod *gitssh.PublicKeys
		err        error
	}{
		{
			gitHandler{
				parsedRepositoryURL: transport.Endpoint{
					Protocol: "ssh",
					User:     "admin",
				},
				authConfig: GitAuthConfig{
					PrivateSSHKey:       privateSSHKey,
					SkipHostKeyChecking: true,
				},
				ssmParameterResolverBridge: bridge,
			},
			getTestPublicKeysAuth("admin", privateSSHKey, ssh.InsecureIgnoreHostKey()),
			nil,
		},
		{
			gitHandler{
				parsedRepositoryURL: transport.Endpoint{
					Protocol: "ssh",
				},
				authConfig: GitAuthConfig{
					PrivateSSHKey: "{{ssm-secure:privateSSHKey}}",
				},
				ssmParameterResolverBridge: bridge,
			},
			getTestPublicKeysAuth("git", privateSSHKey, nil),
			nil,
		},
		{
			gitHandler{
				parsedRepositoryURL: transport.Endpoint{
					Protocol: "ssh",
				},
				authConfig: GitAuthConfig{
					PrivateSSHKey: "plain-string",
				},
				ssmParameterResolverBridge: bridge,
			},
			nil,
			errors.New("Cannot authenticate: invalid PEM data"),
		},
		{
			gitHandler{
				parsedRepositoryURL: transport.Endpoint{
					Protocol: "ssh",
				},
				authConfig: GitAuthConfig{
					PrivateSSHKey: "{{ssm-secure:invalid-privateSSHKey-param}}",
				},
				ssmParameterResolverBridge: bridge,
			},
			nil,
			errors.New("parameter does not exist"),
		},
	}

	for _, test := range tests {
		authMethod, err := test.handler.getPublicKeyAuthMethod(logMock)

		if test.err != nil {
			assert.Error(t, err, getString(test))
			assert.EqualError(t, test.err, err.Error(), getString(test))
			assert.Nil(t, authMethod, getString(test))
		} else {
			publicKeyAuth := authMethod.(*gitssh.PublicKeys)

			assert.NoError(t, err, getString(test))
			assert.Equal(t, test.authMethod.Name(), publicKeyAuth.Name(), getString(test))
			assert.Equal(t, test.authMethod.User, publicKeyAuth.User, getString(test))
			assert.Equal(t, test.authMethod.Signer.PublicKey(), publicKeyAuth.Signer.PublicKey(), getString(test))
			assert.IsType(t, test.authMethod.HostKeyCallback, publicKeyAuth.HostKeyCallback, getString(test))
		}
	}
}

func TestGitHandler_getAuthMethodHTTPBasic(t *testing.T) {
	bridge := bridgemock.GetSsmParamResolverBridge(parameterStoreParameters)

	tests := []struct {
		handler    gitHandler
		authMethod *http.BasicAuth
		err        error
	}{
		{
			gitHandler{
				parsedRepositoryURL: transport.Endpoint{
					Protocol: "http",
				},
				authConfig: GitAuthConfig{
					Username: "admin",
					Password: "pwd",
				},
				ssmParameterResolverBridge: bridge,
			},
			&http.BasicAuth{
				Username: "admin",
				Password: "pwd",
			},
			nil,
		},
		{
			gitHandler{
				parsedRepositoryURL: transport.Endpoint{
					Protocol: "http",
				},
				authConfig: GitAuthConfig{
					Username: "{{ssm-secure:username}}",
					Password: "{{ssm-secure:password}}",
				},
				ssmParameterResolverBridge: bridge,
			},
			&http.BasicAuth{
				Username: "admin",
				Password: "pwd",
			},
			nil,
		},
		{
			gitHandler{
				parsedRepositoryURL: transport.Endpoint{
					Protocol: "http",
				},
				authConfig: GitAuthConfig{
					Username: "{{ssm-secure:invalid-username-param}}",
					Password: "{{ssm-secure:password}}",
				},
				ssmParameterResolverBridge: bridge,
			},
			nil,
			errors.New("parameter does not exist"),
		},
		{
			gitHandler{
				parsedRepositoryURL: transport.Endpoint{
					Protocol: "http",
				},
				authConfig: GitAuthConfig{
					Username: "{{ssm-secure:username}}",
					Password: "{{ssm-secure:invalid-password-param}}",
				},
				ssmParameterResolverBridge: bridge,
			},
			nil,
			errors.New("parameter does not exist"),
		},
	}

	for _, test := range tests {
		authMethod, err := test.handler.getHttpBasicAuthMethod(logMock)

		if test.err != nil {
			assert.Error(t, err, getString(test))
			assert.EqualError(t, test.err, err.Error(), getString(test))
			assert.Nil(t, authMethod, getString(test))
		} else {
			httpBasicAuth := authMethod.(*http.BasicAuth)

			assert.NoError(t, err, getString(test))
			assert.Equal(t, test.authMethod, httpBasicAuth, getString(test))
		}
	}
}

func TestGitHandler_CloneRepository(t *testing.T) {
	plainCloneMethod = PlainClone

	tests := []struct {
		handler  gitHandler
		destPath string
		err      error
	}{
		{
			gitHandler{
				repositoryURL: "git@private-git-repo",
			},
			"unknown-host",
			errors.New("Cannot clone repository git@private-git-repo: Unknown host key. Please add remote " +
				"host key known_hosts file or set SourceInfo 'skipHostKeyChecking' parameter to true in order to skip " +
				"host key validation"),
		},
		{
			gitHandler{
				repositoryURL: "git@private-git-repo",
			},
			"error",
			errors.New("Cannot clone repository git@private-git-repo: random err"),
		},
		{
			gitHandler{
				repositoryURL: "git@private-git-repo",
			},
			"/tmp",
			nil,
		},
	}

	for _, test := range tests {
		repository, err := test.handler.CloneRepository(logMock, nil, test.destPath)

		if test.err != nil {
			assert.Error(t, err, getString(test))
			assert.EqualError(t, test.err, err.Error(), getString(test))
			assert.Nil(t, repository, getString(test))
		} else {
			assert.NoError(t, err, getString(test))
			assert.NotNil(t, repository)
		}
	}

	plainCloneMethod = gogit.PlainClone
}

func TestGitHandler_performCheckout_FailedWorktreeRetrieval(t *testing.T) {
	handler := gitHandler{
		getOptions: gitresource.CheckoutOptions{
			Branch: "master",
		},
	}

	expectedError := fmt.Errorf("error")
	gitRepoMock := new(gitcoremock.GitRepositoryMock)
	gitRepoMock.On("Worktree").Return(new(gitcoremock.GitWorktreeMock), expectedError)

	err := handler.PerformCheckout(gitRepoMock)
	assert.Error(t, err)
	assert.Equal(t, fmt.Sprintf("Cannot retrieve repository's working tree: %v", expectedError.Error()), err.Error())
	gitRepoMock.AssertExpectations(t)
}

func TestGitHandler_performCheckout_FailedCheckoutOperation(t *testing.T) {
	handler := gitHandler{
		getOptions: gitresource.CheckoutOptions{
			CommitID: "commit123",
		},
	}

	expectedErr := fmt.Errorf("try again")
	gitRepoMock := new(gitcoremock.GitRepositoryMock)
	gitWorktreeMock := new(gitcoremock.GitWorktreeMock)

	gitRepoMock.On("Worktree").Return(gitWorktreeMock, nil)
	gitWorktreeMock.On("Checkout", mock.Anything).Return(expectedErr)

	err := handler.PerformCheckout(gitRepoMock)
	assert.Error(t, err)
	assert.EqualError(t, err, fmt.Sprintf("Cannot checkout: %v", expectedErr.Error()))
	gitWorktreeMock.AssertExpectations(t)
}

func TestGitHandler_performCheckout_CommitID(t *testing.T) {
	handler := gitHandler{
		getOptions: gitresource.CheckoutOptions{
			CommitID: "commit123",
		},
	}

	gitRepoMock := new(gitcoremock.GitRepositoryMock)
	gitWorktreeMock := new(gitcoremock.GitWorktreeMock)

	gitRepoMock.On("Worktree").Return(gitWorktreeMock, nil)
	gitWorktreeMock.On("Checkout", &gogit.CheckoutOptions{
		Hash: plumbing.NewHash("commit123"),
	}).Return(nil)

	err := handler.PerformCheckout(gitRepoMock)
	assert.NoError(t, err)
	gitWorktreeMock.AssertExpectations(t)
}

func TestGitHandler_performCheckout_Branch(t *testing.T) {
	handler := gitHandler{
		getOptions: gitresource.CheckoutOptions{
			Branch: "master",
		},
	}

	gitRepoMock := new(gitcoremock.GitRepositoryMock)
	gitWorktreeMock := new(gitcoremock.GitWorktreeMock)

	gitRepoMock.On("Worktree").Return(gitWorktreeMock, nil)
	gitWorktreeMock.On("Checkout", &gogit.CheckoutOptions{
		Branch: "master",
	}).Return(nil)

	err := handler.PerformCheckout(gitRepoMock)
	assert.NoError(t, err)
	gitWorktreeMock.AssertExpectations(t)
}
