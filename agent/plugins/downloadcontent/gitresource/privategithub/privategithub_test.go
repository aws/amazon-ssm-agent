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

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/parameterstore"
	"github.com/aws/amazon-ssm-agent/agent/ssmparameterresolver"
	"github.com/stretchr/testify/assert"

	"net/http"
	"testing"
)

var logMock = log.NewMockLog()

func TestTokenInfoImpl_GetOAuthClient(t *testing.T) {

	var tokenInfoInput []string
	tokenInfoInput = append(tokenInfoInput, `{{ ssm-secure:dummysecureparam }}`)
	oauthclientmock := gitmock.OAuthClientMock{}

	tokenValue := "lskjksjgshfg1234jdskjhgvs"

	var clientVal *http.Client
	oauthclientmock.On("GetGithubOauthClient", tokenValue).Return(clientVal)
	tokenInfo := TokenInfoImpl{
		SsmParameter:   getMockedSecureParam,
		gitoauthclient: oauthclientmock,
	}

	httpout, err := tokenInfo.GetOAuthClient(logMock, `{{ ssm-secure:dummysecureparam }}`)

	assert.NoError(t, err)
	assert.Equal(t, clientVal, httpout)
	oauthclientmock.AssertExpectations(t)

}

func TestTokenInfoImpl_ValidateTokenParameter(t *testing.T) {

	tokenInfoInput := `{ "dummysecureparam" }`
	oauthclientmock := gitmock.OAuthClientMock{}
	tokenInfo := TokenInfoImpl{
		gitoauthclient: oauthclientmock,
	}

	httpout, err := tokenInfo.GetOAuthClient(logMock, tokenInfoInput)

	assert.Error(t, err)
	assert.Nil(t, httpout)
	oauthclientmock.AssertExpectations(t)
	assert.Equal(t, err.Error(), "Format of specifying ssm parameter used for token-parameter-name is incorrect. "+
		"Please specify parameter as '{{ ssm-secure:parameter-name }}'")
}

func TestTokenInfoImpl_ValidateSecureParameter(t *testing.T) {

	tokenInfoInput := `{{ssm-secure:dummysecureparam}}`
	oauthclientmock := gitmock.OAuthClientMock{}

	var clientVal *http.Client

	tokenInfo := TokenInfoImpl{
		SsmParameter:   getMockedParam,
		gitoauthclient: oauthclientmock,
	}

	httpout, err := tokenInfo.GetOAuthClient(logMock, tokenInfoInput)

	assert.Error(t, err)
	assert.Equal(t, err.Error(), "token-parameter-name dummysecureparam must be of secure string type, Current type - String")
	assert.Equal(t, clientVal, httpout)
	oauthclientmock.AssertExpectations(t)
}

func getMockedParam(log log.T, paramService ssmparameterresolver.ISsmParameterService, parameterReferences []string,
	resolverOptions ssmparameterresolver.ResolveOptions) (info map[string]ssmparameterresolver.SsmParameterInfo, err error) {
	tokenValue := "lskjksjgshfg1234jdskjhgvs"
	secureParamOut := ssmparameterresolver.SsmParameterInfo{
		Name:  "dummysecureparam",
		Type:  parameterstore.ParamTypeString,
		Value: tokenValue,
	}
	info = make(map[string]ssmparameterresolver.SsmParameterInfo)
	info["ssm-secure:dummysecureparam"] = secureParamOut

	return info, nil
}

func getMockedSecureParam(log log.T, paramService ssmparameterresolver.ISsmParameterService, parameterReferences []string,
	resolverOptions ssmparameterresolver.ResolveOptions) (info map[string]ssmparameterresolver.SsmParameterInfo, err error) {
	tokenValue := "lskjksjgshfg1234jdskjhgvs"
	secureParamOut := ssmparameterresolver.SsmParameterInfo{
		Name:  "dummysecureparam",
		Type:  parameterstore.ParamTypeSecureString,
		Value: tokenValue,
	}
	info = make(map[string]ssmparameterresolver.SsmParameterInfo)
	info["ssm-secure:dummysecureparam"] = secureParamOut

	return info, nil
}
