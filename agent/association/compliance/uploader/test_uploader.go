// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package compliance

import (
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/mock"
)

// ComplianceUploaderMock stands for a mocked compliance uploader.
type ComplianceUploaderMock struct {
	mock.Mock
}

// NewMockDefault returns an instance of Mock with default expectations set.
func NewMockDefault() *ComplianceUploaderMock {
	return new(ComplianceUploaderMock)
}

func (m *ComplianceUploaderMock) CreateNewServiceIfUnHealthy(log log.T) {
	m.Called(log)
}

func (m *ComplianceUploaderMock) UpdateAssociationCompliance(associationId string, instanceId string, documentName string, documentVersion string, associationStatus string, executionTime time.Time) error {
	args := m.Called(associationId, instanceId, documentName, documentVersion, associationStatus, executionTime)
	return args.Error(0)
}
