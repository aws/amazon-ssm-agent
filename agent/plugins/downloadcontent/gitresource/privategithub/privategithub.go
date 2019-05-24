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

// Package privategithub deals with all the authorization invocations to access private github
package privategithub

import (
	"github.com/aws/amazon-ssm-agent/agent/githubclient"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/parameterstore"
	"github.com/aws/amazon-ssm-agent/agent/ssmparameterresolver"

	"errors"
	"fmt"
	"net/http"
	"regexp"
)

const (
	ssmSecurePrefix = "ssm-secure:"
)

type PrivateGithubAccess interface {
	GetOAuthClient(log log.T, token string) (*http.Client, error)
}

type TokenInfoImpl struct {
	SsmParameter func(log log.T, paramService ssmparameterresolver.ISsmParameterService, parameterReferences []string,
		resolverOptions ssmparameterresolver.ResolveOptions) (info map[string]ssmparameterresolver.SsmParameterInfo, err error)
	paramAccess    ssmparameterresolver.SsmParameterService
	gitoauthclient githubclient.IOAuthClient
}

// GetOAuthClient is the only method from privategithub package that is accessible to gitresource
func (t TokenInfoImpl) GetOAuthClient(log log.T, tokenInfo string) (client *http.Client, err error) {
	// Validate the format of the secure parameter
	// Make a call to secure string (disable logging) and obtain the token
	// Create StaticTokenSource and create oauth client and return it

	// Validate the format of token information
	if valid, err := validateTokenParameter(tokenInfo); !valid {
		return nil, err
	}

	var tokenVal ssmparameterresolver.SsmParameterInfo
	var tokenMap map[string]ssmparameterresolver.SsmParameterInfo
	var parameterReferences []string

	// Regex to extract the contents of the parameter from within {{ }} to get parameter value
	// for. e.g. {{ ssm-secure:parameter-name }} will extract ssm-secure:parameter-name
	subParam := regexp.MustCompile(`\{\{(.*?)\}\}`).FindStringSubmatch(tokenInfo)
	if len(subParam) > 1 {
		parameterReferences = []string{subParam[1]}
	} else {
		return client, errors.New("Something went wrong when trying to extract ssm-secure parameter")
	}

	resolverOptions := ssmparameterresolver.ResolveOptions{
		IgnoreSecureParameters: false,
	}

	// Get the parameter value from parameter store.
	// NOTE: Do not log the parameter value
	if tokenMap, err = t.SsmParameter(log, &t.paramAccess, parameterReferences, resolverOptions); err != nil {
		return nil, fmt.Errorf("Could not resolve ssm parameter - %v. Error - %v", parameterReferences, err)
	}

	// Parameter output must be of size 1. Any other number of tokens returned can lead to undesired behavior
	if len(tokenMap) != 1 {
		return nil, fmt.Errorf("Invalid number of tokens returned - %v", len(tokenMap))
	}

	//Extracting single value of token contained within tokenMap
	for _, token := range tokenMap {
		tokenVal = token
	}

	// Validating to check if the parameter obtained is a secure string
	if tokenVal.Type != parameterstore.ParamTypeSecureString {
		return nil, fmt.Errorf("token-parameter-name %v must be of secure string type, Current type - %v", tokenVal.Name, tokenVal.Type)
	}
	return t.gitoauthclient.GetGithubOauthClient(tokenVal.Value), nil
}

func getSSMParameter(log log.T, paramService ssmparameterresolver.ISsmParameterService, parameterReferences []string,
	resolverOptions ssmparameterresolver.ResolveOptions) (info map[string]ssmparameterresolver.SsmParameterInfo, err error) {

	return ssmparameterresolver.ResolveParameterReferenceList(paramService, log, parameterReferences, resolverOptions)
}

// validateTokenParameter validates the format of tokenInfo
func validateTokenParameter(tokenInfo string) (valid bool, err error) {

	// Regex to check the pattern of the secure parameter required.
	// pattern must be equal to {{ ssm-secure:parameter-name }}
	var ssmSecureStringPattern = regexp.MustCompile("{{\\s*(" + ssmSecurePrefix + "[\\w-./]+)\\s*}}")
	if ssmSecureStringPattern.Match([]byte(tokenInfo)) {
		return true, nil
	}
	return false, errors.New("Format of specifying ssm parameter used for token-parameter-name is incorrect. " +
		"Please specify parameter as '{{ ssm-secure:parameter-name }}'")
}

// NewTokenInfoImpl returns an object of type TokenInfoImpl
func NewTokenInfoImpl() TokenInfoImpl {
	parameterService := ssmparameterresolver.NewService()
	return TokenInfoImpl{
		SsmParameter:   getSSMParameter,
		paramAccess:    parameterService,
		gitoauthclient: githubclient.OAuthClient{},
	}
}
