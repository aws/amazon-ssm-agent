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

// package securestringaccess is used to access the secure string parameters from parameterstore
package securestringaccess

import (
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/parameterstore"
	"github.com/stretchr/testify/assert"

	"testing"
)

func TestSecureParamImpl_GetSecureParameters(t *testing.T) {
	log := log.NewMockLog()
	secureParam := SecureParamImpl{}
	ssmParams := "ssm:dummyParam"
	CallSecureParameters = callStubCorrectParameterService
	out, err := secureParam.GetSecureParameter(log, ssmParams)

	assert.NoError(t, err)
	assert.Equal(t, "parameterResponse", out.Value)
	assert.Equal(t, "dummyParam", out.Name)
}

func TestSecureParamImpl_GetSecureParametersParenthesis(t *testing.T) {
	log := log.NewMockLog()
	secureParam := SecureParamImpl{}
	ssmParams := "{{ ssm:dummyParam }}"
	CallSecureParameters = callStubCorrectParameterService
	out, err := secureParam.GetSecureParameter(log, ssmParams)

	assert.NoError(t, err)
	assert.Equal(t, "parameterResponse", out.Value)
	assert.Equal(t, "dummyParam", out.Name)
}

func TestSecureParamImpl_GetSecureParametersIncorrect(t *testing.T) {
	log := log.NewMockLog()
	secureParam := SecureParamImpl{}
	ssmParams := "{{ dummyParam }}"
	CallSecureParameters = callStubCorrectParameterService
	_, err := secureParam.GetSecureParameter(log, ssmParams)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Encountered errors while parsing secure parmeter. "+
		"Parameter format is incorrect")
}

func callStubCorrectParameterService(log log.T, paramNames string) (*parameterstore.Parameter, error) {
	param := parameterstore.Parameter{
		Value: "parameterResponse",
		Name:  "dummyParam",
		Type:  parameterstore.ParamTypeSecureString,
	}

	return &param, nil
}
