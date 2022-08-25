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

package datastore

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	mockfs "github.com/aws/amazon-ssm-agent/core/workerprovider/longrunningprovider/datastore/filesystem/mocks"
	"github.com/aws/amazon-ssm-agent/core/workerprovider/longrunningprovider/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

var (
	filename    = "filename"
	workprocess = model.Worker{
		Name: "name",
	}
	filepath = "path"
)

type StoreTestSuite struct {
	suite.Suite
	mockFileSystem *mockfs.FileSystem
	data           string
	filename       string
	filepath       string
	dataStore      IStore
}

func (suite *StoreTestSuite) SetupTest() {
	mockFileSystem := &mockfs.FileSystem{}
	suite.mockFileSystem = mockFileSystem

	datajson, _ := json.Marshal(workprocess)
	suite.data = string(datajson)
	suite.filename = filename
	suite.filepath = filepath

	suite.dataStore = &LocalFileStore{
		fileSystem: mockFileSystem,
		log:        log.NewMockLog()}
}

// Execute the test suite
func TestDataStoreTestSuite(t *testing.T) {
	suite.Run(t, new(StoreTestSuite))
}

func (suite *StoreTestSuite) TestWrite_WhenPathExists() {

	suite.mockFileSystem.On("Stat", suite.filepath).Return(nil, nil)
	suite.mockFileSystem.On("WriteFile", suite.filename, mock.Anything, mock.Anything).Return(nil)

	err := suite.dataStore.Write(suite.data, suite.filepath, suite.filename)

	assert.Nil(suite.T(), err)
	suite.mockFileSystem.AssertExpectations(suite.T())
}

func (suite *StoreTestSuite) TestWrite_WhenPathDoesNotExist() {

	suite.mockFileSystem.On("Stat", suite.filepath).Return(nil, errors.New("file does not exist"))
	suite.mockFileSystem.On("IsNotExist", mock.Anything).Return(false)
	suite.mockFileSystem.On("MkdirAll", suite.filepath, mock.Anything).Return(nil)
	suite.mockFileSystem.On("WriteFile", suite.filename, []byte(suite.data), mock.Anything).Return(nil)

	err := suite.dataStore.Write(suite.data, suite.filepath, suite.filename)

	assert.Nil(suite.T(), err)
	suite.mockFileSystem.AssertExpectations(suite.T())
}

func (suite *StoreTestSuite) TestRead() {
	datajson, _ := json.Marshal(workprocess)
	var worker model.Worker

	suite.mockFileSystem.On("Stat", suite.filename).Return(nil, nil)
	suite.mockFileSystem.On("ReadFile", suite.filename).Return(datajson, nil)

	err := suite.dataStore.Read(suite.filename, worker)

	assert.Nil(suite.T(), err)

	suite.mockFileSystem.AssertExpectations(suite.T())
}
