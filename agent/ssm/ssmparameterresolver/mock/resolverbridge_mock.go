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

package ssmparameterresolver

import (
	"errors"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/ssm/ssmparameterresolver"
	"github.com/stretchr/testify/mock"
)

type SsmParameterResolverBridgeMock struct {
	mock.Mock
}

func (mock *SsmParameterResolverBridgeMock) IsValidParameterStoreReference(value string) bool {
	args := mock.Called(value)
	return args.Bool(0)
}

func (mock *SsmParameterResolverBridgeMock) GetParameterFromSsmParameterStore(log log.T, parameter string) (string, error) {
	args := mock.Called(log, parameter)
	return args.String(0), args.Error(1)
}

func GetSsmParamResolverBridge(parameterStoreParameters map[string]string) ssmparameterresolver.ISsmParameterResolverBridge {
	bridgeMock := &SsmParameterResolverBridgeMock{}
	for k, v := range parameterStoreParameters {
		bridgeMock.On("GetParameterFromSsmParameterStore", mock.Anything, k).Return(v, nil)
	}
	bridgeMock.On("GetParameterFromSsmParameterStore", mock.Anything, mock.Anything).Return("", errors.New("parameter does not exist"))

	bridgeMock.On("IsValidParameterStoreReference", mock.MatchedBy(func(reference string) bool {
		return strings.HasPrefix(reference, "{{ssm-secure:")
	})).Return(true)
	bridgeMock.On("IsValidParameterStoreReference", mock.MatchedBy(func(reference string) bool {
		return !strings.HasPrefix(reference, "{{ssm-secure:")
	})).Return(false)

	return bridgeMock
}
