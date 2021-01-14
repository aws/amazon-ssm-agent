// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.
package endpoint

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
)

type metadataMock struct {
	mock.Mock
}

func (m *metadataMock) GetMetadata(val string) (string, error) {
	ret := m.Called(val)

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type GetDefaultEndPointTest struct {
	Region  string
	Service string
	Output  string
}

var (
getDefaultEndPointTests = []GetDefaultEndPointTest{
	{"", "", ""},
	{"val", "test", ""},
	{"us-east-1", "ssm", ""},
	{"cn-north-1", "ssm", "ssm.cn-north-1.amazonaws.com.cn"},
}
)

func TestGetDefaultEndPoint(t *testing.T) {
	correctEc2Metadata := ec2Metadata
	ec2MetadataMock := &metadataMock{}
	defer func() {ec2Metadata = correctEc2Metadata} ()

	ec2MetadataMock.On("GetMetadata", mock.Anything).Return("", fmt.Errorf("SomeError"))
	ec2Metadata = ec2MetadataMock

	for _, test := range getDefaultEndPointTests {
		output := GetDefaultEndpoint(log.NewMockLog(), test.Service, test.Region, "")
		assert.Equal(t, test.Output, output)
	}
}