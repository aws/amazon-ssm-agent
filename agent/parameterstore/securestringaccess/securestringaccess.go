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
// NOTE:This package cannot under any circumstance log the value of the parameter obtained
package securestringaccess

import (
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/parameterstore"
	"github.com/aws/amazon-ssm-agent/agent/ssm"

	"errors"
	"fmt"
	"regexp"
)

var CallSecureParameters = callSecureParameterService

type SecureStringAccess interface {
	GetSecureParameter(log log.T, ssmParams string) (out parameterstore.Parameter, err error)
}

type SecureParamImpl struct{}

//TODO: meloniam@ 08/23/2017 Refactor this code with parameterstore to remove duplication.
// GetSecureParameters takes a list of strings and resolves them by calling GetParameter API
// This parameter allows calls to secure string parameters
// NOTE: This function cannot under any circumstance log the value of the parameter obtained
func (param SecureParamImpl) GetSecureParameter(log log.T, ssmParams string) (out parameterstore.Parameter, err error) {
	var result *parameterstore.Parameter
	validParamRegex := ":([/\\w.-]+)*"
	validParam, err := regexp.Compile(validParamRegex)
	if err != nil {
		return out, fmt.Errorf("%v", parameterstore.ErrorMsg)
	}
	var paramNames string
	temp := validParam.FindString(ssmParams)
	if temp == "" {
		return out, errors.New("Encountered errors while parsing secure parmeter. " +
			"Parameter format is incorrect")
	}

	paramNames = temp[1:]

	if result, err = CallSecureParameters(log, paramNames); err != nil {
		return out, err
	}

	return *result, nil
}

// callSecureParameterService makes a GetParameters API call to the service
// NOTE: This function cannot under any circumstance log the value of the parameter obtained
func callSecureParameterService(log log.T, paramName string) (*parameterstore.Parameter, error) {

	ssmSvc := ssm.NewService()

	result, err := ssmSvc.GetSecureParameter(log, paramName, true)
	if err != nil {
		return nil, err
	}

	var response parameterstore.Parameter
	err = jsonutil.Remarshal(result, &response)
	if err != nil {
		log.Debug(err)
		return nil, fmt.Errorf("%v", parameterstore.ErrorMsg)
	}

	return &response, nil
}
