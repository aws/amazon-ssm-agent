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

// Package application contains a application gatherer.
package application

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/localpackages"
	repomock "github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/localpackages/mock"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestComponentType(t *testing.T) {
	awsComponents := []string{"amazon-ssm-agent", "aws-apitools-mon", "aws-amitools-ec2", "AWS Tools for Windows", "AWS PV Drivers"}
	nonawsComponents := []string{"Notepad++", "Google Update Helper", "accountsservice", "pcre", "kbd-misc"}

	for _, name := range awsComponents {
		assert.Equal(t, model.AWSComponent, componentType(name))
	}

	for _, name := range nonawsComponents {
		assert.Equal(t, model.ComponentType(0), componentType(name))
	}
}

func MockPackageRepositoryEmpty() localpackages.Repository {
	mockRepo := repomock.MockedRepository{}
	mockRepo.On("GetInventoryData", mock.Anything).Return([]model.ApplicationData{})
	return &mockRepo
}

func MockPackageRepository(result []model.ApplicationData) localpackages.Repository {
	mockRepo := repomock.MockedRepository{}
	mockRepo.On("GetInventoryData", mock.Anything).Return(result)
	return &mockRepo
}
