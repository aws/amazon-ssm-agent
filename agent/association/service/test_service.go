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

// Package service wraps SSM service
package service

import (
	"github.com/aws/amazon-ssm-agent/agent/association/model"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/stretchr/testify/mock"
)

// AssociationServiceMock stands for a mocked association service.
type AssociationServiceMock struct {
	mock.Mock
}

// NewMockDefault returns an instance of Mock with default expectations set.
func NewMockDefault() *AssociationServiceMock {
	return new(AssociationServiceMock)
}

// ListInstanceAssociations mocks implementation for ListAssociations
func (m *AssociationServiceMock) ListInstanceAssociations(log log.T, instanceID string) ([]*model.InstanceAssociation, error) {
	args := m.Called(log, instanceID)
	return args.Get(0).([]*model.InstanceAssociation), args.Error(1)
}

// CreateNewServiceIfUnHealthy mocks implementation for CreateNewServiceIfUnHealthy
func (m *AssociationServiceMock) CreateNewServiceIfUnHealthy(log log.T) {
	m.Called(log)
}

// LoadAssociationDetail mocks implementation for LoadAssociationDetail
func (m *AssociationServiceMock) LoadAssociationDetail(log log.T, assoc *model.InstanceAssociation) error {
	args := m.Called(log, assoc)
	return args.Error(0)
}

// UpdateInstanceAssociationStatus mocks implementation for UpdateInstanceAssociationStatus
func (m *AssociationServiceMock) UpdateInstanceAssociationStatus(
	log log.T,
	associationID string,
	associationName string,
	instanceID string,
	status string,
	errorCode string,
	executionDate string,
	executionSummary string,
	outputUrl string) {
	executionResult := ssm.InstanceAssociationExecutionResult{}

	m.Called(log, associationID, associationName, instanceID, &executionResult)
	return
}

// UpdateAssociationStatus mocks implementation for UpdateAssociationStatus
func (m *AssociationServiceMock) UpdateAssociationStatus(
	log log.T,
	associationName string,
	instanceID string,
	status string,
	executionSummary string) {
	m.Called(log, associationName, instanceID, status, executionSummary)
	return
}

// UsesInstanceAssociationApi mocks implementation for UsesInstanceAssociationApi
func (m *AssociationServiceMock) IsInstanceAssociationApiMode() bool {
	args := m.Called()
	return args.Get(0).(bool)
}

// DescribeAssociation mocks implementation for DescribeAssociation
func (m *AssociationServiceMock) DescribeAssociation(log log.T, instanceID string, docName string) (response *ssm.DescribeAssociationOutput, err error) {
	args := m.Called(log, instanceID, docName)
	return args.Get(0).(*ssm.DescribeAssociationOutput), args.Error(1)
}
