// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package ssm

import (
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/stretchr/testify/mock"
)

// Note: This code is used in the test files. However, this code is not in a _test.go file
// because then we would have to copy it in every test package that needs the mock.

// Mock stands for a mocked SSM service.
type Mock struct {
	mock.Mock
}

// NewMockDefault returns an instance of Mock with default expectations set.
func NewMockDefault() *Mock {
	return new(Mock)
}

// ListAssociations mocks the ListAssociations function.
func (m *Mock) ListAssociations(log log.T, instanceID string) (response *ssm.ListAssociationsOutput, err error) {
	args := m.Called(log, instanceID)
	return args.Get(0).(*ssm.ListAssociationsOutput), args.Error(1)
}

// SendCommand mocks the SendCommand function.
func (m *Mock) SendCommand(log log.T,
	documentName string,
	instanceIDs []string,
	parameters map[string][]*string,
	timeoutSeconds *int64,
	outputS3BucketName *string,
	outputS3KeyPrefix *string) (response *ssm.SendCommandOutput, err error) {

	args := m.Called(documentName, instanceIDs, parameters, timeoutSeconds, outputS3BucketName, outputS3KeyPrefix)
	return args.Get(0).(*ssm.SendCommandOutput), args.Error(1)
}

// ListCommands mocks the ListCommands function.
func (m *Mock) ListCommands(log log.T, instanceID string) (response *ssm.ListCommandsOutput, err error) {
	args := m.Called(log, instanceID)
	return args.Get(0).(*ssm.ListCommandsOutput), args.Error(1)
}

// ListCommandInvocations mocks the ListCommandInvocations function.
func (m *Mock) ListCommandInvocations(log log.T, instanceID string, commandID string) (response *ssm.ListCommandInvocationsOutput, err error) {
	args := m.Called(log, instanceID, commandID)
	return args.Get(0).(*ssm.ListCommandInvocationsOutput), args.Error(1)
}

// CancelCommand mocks the CancelCommand function.
func (m *Mock) CancelCommand(log log.T, commandID string, instanceIDs []string) (response *ssm.CancelCommandOutput, err error) {
	args := m.Called(log, commandID, instanceIDs)
	return args.Get(0).(*ssm.CancelCommandOutput), args.Error(1)
}

// CreateDocument mocks the CreateDocument function.
func (m *Mock) CreateDocument(log log.T, docName string, docContent string) (response *ssm.CreateDocumentOutput, err error) {
	args := m.Called(log, docName, docContent)
	return args.Get(0).(*ssm.CreateDocumentOutput), args.Error(1)
}

// DeleteDocument mocks the DeleteDocument function.
func (m *Mock) DeleteDocument(log log.T, instanceID string) (response *ssm.DeleteDocumentOutput, err error) {
	args := m.Called(log, instanceID)
	return args.Get(0).(*ssm.DeleteDocumentOutput), args.Error(1)
}

// UpdateInstanceInformation mocks the UpdateInstanceInformation function.
func (m *Mock) UpdateInstanceInformation(log log.T, agentVersion string, agentStatus string) (response *ssm.UpdateInstanceInformationOutput, err error) {
	args := m.Called(log, agentVersion, agentStatus)
	return args.Get(0).(*ssm.UpdateInstanceInformationOutput), args.Error(1)
}
