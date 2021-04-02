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

	"github.com/aws/amazon-ssm-agent/agent/log"
	endpointmocks "github.com/aws/amazon-ssm-agent/common/identity/endpoint/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type GetDefaultEndPointTest struct {
	Region  string
	Service string
	Output  string
}

var (
	getDefaultEndPointTests = []GetDefaultEndPointTest{
		{"", "", ""},
		{"val", "test", "test.val.amazonaws.com"},
		{"us-east-1", "ssm", "ssm.us-east-1.amazonaws.com"},
		{"cn-north-1", "ssm", "ssm.cn-north-1.amazonaws.com.cn"},
		{"unknown-region", "ssmmessages", "ssmmessages.unknown-region.amazonaws.com"},
	}
)

func TestGetDefaultEndPoint(t *testing.T) {
	correctEc2Metadata := ec2Metadata
	ec2MetadataMock := &endpointmocks.IEC2MdsSdkClient{}
	defer func() { ec2Metadata = correctEc2Metadata }()

	ec2MetadataMock.On("GetMetadata", mock.Anything).Return("", fmt.Errorf("SomeError"))
	ec2Metadata = ec2MetadataMock

	for _, test := range getDefaultEndPointTests {
		output := GetDefaultEndpoint(log.NewMockLog(), test.Service, test.Region, "")
		assert.Equal(t, test.Output, output)
	}
}
