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

package mock

import (
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/parameterstore"

	"github.com/stretchr/testify/mock"
)

type SecureParamMock struct {
	mock.Mock
}

func (m SecureParamMock) GetSecureParameter(log log.T, ssmParams string) (out parameterstore.Parameter, err error) {
	args := m.Called(log, ssmParams)
	return args.Get(0).(parameterstore.Parameter), args.Error(1)
}
