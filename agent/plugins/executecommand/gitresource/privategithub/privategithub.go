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
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/parameterstore"
	"github.com/aws/amazon-ssm-agent/agent/parameterstore/securestringaccess"

	"errors"
	"fmt"
	"net/http"
	"strings"
)

type PrivateGithubAccess interface {
	GetOAuthClient(log log.T, tokenInfo string) (*http.Client, error)
}

type TokenInfoImpl struct {
	tokenInfoVal   TokenInfoParamVal
	ParamAccess    securestringaccess.SecureStringAccess
	gitoauthclient githubclient.IOAuthClient
}

type TokenInfoParamVal struct {
	TokenParameterName string `json:"token-parameter-name"`
	OauthAccessType    string `json:"oauth-access-type"`
}

// GetOAuthClient is the only method from privategithub package that is accessible to gitresource
func (t TokenInfoImpl) GetOAuthClient(log log.T, tokenInfo string) (*http.Client, error) {
	// Validate the format of the token info parameter
	// Validate the information obtained in the JSON file.
	// Make a call to secure string (disable logging) and obtain the token
	// Create StaticTokenSource and create oauth client and return it

	var err error
	if err = jsonutil.Unmarshal(tokenInfo, &t.tokenInfoVal); err != nil {
		log.Error("Unmarshalling token value parameter failed - ", err)
		return nil, err
	}

	if valid, err := validateTokenInfoJson(t.tokenInfoVal); !valid {
		return nil, err
	}

	// Validate the format of token information
	if valid, err := validateTokenParameter(t.tokenInfoVal.TokenParameterName); !valid {
		return nil, err
	}

	tokenValue, err := t.ParamAccess.GetSecureParameter(log, t.tokenInfoVal.TokenParameterName)
	if err != nil {
		return nil, err
	}
	if tokenValue.Type != parameterstore.ParamTypeSecureString {
		return nil, fmt.Errorf("token-parameter-name must be of secure string type - %v, %v", tokenValue.Name, tokenValue.Type)
	}
	return t.gitoauthclient.GetGithubOauthClient(tokenValue.Value), nil
}

// validateTokenParameter validates the format of tokenInfo
func validateTokenParameter(tokenInfo string) (valid bool, err error) {

	if strings.HasPrefix(tokenInfo, "ssm:") {
		return true, nil
	}
	return false, errors.New("Format of specifying ssm parameter used for token-parameter-name is incorrect. " +
		"Please specify parameter as \"ssm:parameter-name\"")
}

// NewTokenInfoImpl returns an object of type TokenInfoImpl
func NewTokenInfoImpl() TokenInfoImpl {
	return TokenInfoImpl{
		ParamAccess:    securestringaccess.SecureParamImpl{},
		gitoauthclient: githubclient.OAuthClient{},
	}
}

func validateTokenInfoJson(tokenInfoJson TokenInfoParamVal) (valid bool, err error) {
	if tokenInfoJson.TokenParameterName == "" {
		return false, errors.New("Token parameter name must be specified. " +
			"It is the name of the secure string parameter that contains the personal access token.")
	}

	if tokenInfoJson.OauthAccessType == "" || tokenInfoJson.OauthAccessType != "Github" {
		return false, errors.New("Oath Access type must by specified to be 'Github'.")
	}
	return true, nil
}
