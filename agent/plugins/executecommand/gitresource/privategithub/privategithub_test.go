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
	gitmock "github.com/aws/amazon-ssm-agent/agent/githubclient/mock"
	securestringmock "github.com/aws/amazon-ssm-agent/agent/parameterstore/securestringaccess/mock"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/parameterstore"
	"github.com/stretchr/testify/assert"

	"net/http"
	"testing"
)

var logMock = log.NewMockLog()

func TestTokenInfoImpl_GetOAuthClient(t *testing.T) {

	tokenInfoInput := `{
		"token-parameter-name": "ssm:dummysecureparam",
		"oauth-access-type" : "Github"
	}`
	oauthclientmock := gitmock.OAuthClientMock{}

	tokenValue := "lskjksjgshfg1234jdskjhgvs"
	secureParamOut := parameterstore.Parameter{
		Name:  "dummysecureparam",
		Type:  parameterstore.ParamTypeSecureString,
		Value: tokenValue,
	}
	var clientVal *http.Client
	secureStringMock := securestringmock.SecureParamMock{}
	secureStringMock.On("GetSecureParameter", logMock, "ssm:dummysecureparam").Return(secureParamOut, nil).Once()
	oauthclientmock.On("GetGithubOauthClient", tokenValue).Return(clientVal)
	tokenInfo := TokenInfoImpl{
		ParamAccess:    secureStringMock,
		gitoauthclient: oauthclientmock,
	}

	httpout, err := tokenInfo.GetOAuthClient(logMock, tokenInfoInput)

	assert.NoError(t, err)
	assert.Equal(t, clientVal, httpout)
	secureStringMock.AssertExpectations(t)
	oauthclientmock.AssertExpectations(t)

}

func TestTokenInfoImpl_GetOAuthClientNoTokenParam(t *testing.T) {

	tokenInfoInput := `{
		"token-parameter": "ssm:dummysecureparam",
		"oauth-access-type" : "Github"
	}`
	oauthclientmock := gitmock.OAuthClientMock{}

	tokenInfo := TokenInfoImpl{
		ParamAccess:    securestringmock.SecureParamMock{},
		gitoauthclient: oauthclientmock,
	}

	httpout, err := tokenInfo.GetOAuthClient(logMock, tokenInfoInput)

	assert.Error(t, err)
	assert.Nil(t, httpout)
	assert.Contains(t, err.Error(), "Token parameter name must be specified. It is the name of the secure string parameter that contains the personal access token.")
}

func TestTokenInfoImpl_GetOAuthClientNoOauthAccess(t *testing.T) {

	tokenInfoInput := `{
		"token-parameter-name": "ssm:dummysecureparam"
	}`
	oauthclientmock := gitmock.OAuthClientMock{}

	tokenInfo := TokenInfoImpl{
		ParamAccess:    securestringmock.SecureParamMock{},
		gitoauthclient: oauthclientmock,
	}

	httpout, err := tokenInfo.GetOAuthClient(logMock, tokenInfoInput)

	assert.Error(t, err)
	assert.Nil(t, httpout)
	assert.Contains(t, err.Error(), "Oath Access type must by specified to be 'Github'.")
}

func TestTokenInfoImpl_GetOAuthClientIncorrectOauthAccess(t *testing.T) {

	tokenInfoInput := `{
		"token-parameter-name": "ssm:dummysecureparam",
		"oauth-access-type" : "Facebook"
	}`
	oauthclientmock := gitmock.OAuthClientMock{}

	tokenInfo := TokenInfoImpl{
		ParamAccess:    securestringmock.SecureParamMock{},
		gitoauthclient: oauthclientmock,
	}

	httpout, err := tokenInfo.GetOAuthClient(logMock, tokenInfoInput)

	assert.Error(t, err)
	assert.Nil(t, httpout)
	assert.Contains(t, err.Error(), "Oath Access type must by specified to be 'Github'.")
}

func TestTokenInfoImpl_ValidateTokenParameter(t *testing.T) {

	tokenInfoInput := `{
		"token-parameter-name": "dummysecureparam",
		"oauth-access-type" : "Github"
	}`
	oauthclientmock := gitmock.OAuthClientMock{}

	tokenInfo := TokenInfoImpl{
		ParamAccess:    securestringmock.SecureParamMock{},
		gitoauthclient: oauthclientmock,
	}

	httpout, err := tokenInfo.GetOAuthClient(logMock, tokenInfoInput)

	assert.Error(t, err)
	assert.Nil(t, httpout)
	assert.Equal(t, err.Error(), "Format of specifying ssm parameter used for token-parameter-name is incorrect. "+
		"Please specify parameter as \"ssm:parameter-name\"")
}

func TestTokenInfoImpl_ValidateSecureParameter(t *testing.T) {

	tokenInfoInput := `{
		"token-parameter-name": "ssm:dummysecureparam",
		"oauth-access-type" : "Github"
	}`
	oauthclientmock := gitmock.OAuthClientMock{}

	tokenValue := "lskjksjgshfg1234jdskjhgvs"
	secureParamOut := parameterstore.Parameter{
		Name:  "dummysecureparam",
		Type:  parameterstore.ParamTypeString,
		Value: tokenValue,
	}
	var clientVal *http.Client
	secureStringMock := securestringmock.SecureParamMock{}
	secureStringMock.On("GetSecureParameter", logMock, "ssm:dummysecureparam").Return(secureParamOut, nil).Once()
	tokenInfo := TokenInfoImpl{
		ParamAccess:    secureStringMock,
		gitoauthclient: oauthclientmock,
	}

	httpout, err := tokenInfo.GetOAuthClient(logMock, tokenInfoInput)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token-parameter-name must be of secure string type")
	assert.Equal(t, clientVal, httpout)
	secureStringMock.AssertExpectations(t)
	oauthclientmock.AssertExpectations(t)
}
