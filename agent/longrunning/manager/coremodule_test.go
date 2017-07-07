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

// Package manager encapsulates everything related to long running plugin manager that starts, stops & configures long running plugins
package manager

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/longrunning"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const testRepoRoot = "testdata"
const testCloudWatchDocumentName = "cwdocument.json"
const invalidTestCloudWatchDocumentName = "cwdocument_invalid_format1.json"
const invalidTestCloudWatchDocumentName2 = "cwdocument_invalid_format2.json"
const testCloudWatchConfig = "cwconfig.json"

var (
	instanceId = "i-1234567890"

	legacyCwConfigStorePath = fileutil.BuildPath(
		appconfig.EC2ConfigDataStorePath,
		instanceId,
		appconfig.ConfigurationRootDirName,
		appconfig.WorkersRootDirName,
		"aws.cloudWatch.ec2config")

	legacyCwConfigPath = fileutil.BuildPath(
		appconfig.EC2ConfigSettingPath,
		NameOfCloudWatchJsonFile)
)

/*
 *	Tests for TestCheckLegacyCloudWatchRunCommandConfig
 */
func TestCheckLegacyCloudWatchRunCommandConfig(t *testing.T) {
	var hasConfiguration bool
	var testCloudWatchDocument string
	var err error

	testCloudWatchDocument, err = fileutil.ReadAllText(filepath.Join(testRepoRoot, testCloudWatchDocumentName))
	if err != nil {
		fmt.Printf("error: %v", err)
	}

	// Setup mock with expectations
	mockCwcInstance := MockedCwcInstance{}
	mockCwcInstance.On("Enable", mock.Anything).Return(nil).Once()

	mockFileSysUtil := MockedFileSysUtil{}
	mockFileSysUtil.On("Exists", legacyCwConfigStorePath).Return(true).Once()
	mockFileSysUtil.On("ReadFile", legacyCwConfigStorePath).Return([]byte(testCloudWatchDocument), nil).Once()

	hasConfiguration, err = checkLegacyCloudWatchRunCommandConfig(instanceId, &mockCwcInstance, &mockFileSysUtil)

	assert.True(t, hasConfiguration)
	assert.Nil(t, err)
	mockCwcInstance.AssertExpectations(t)
	mockFileSysUtil.AssertExpectations(t)
}

func TestCheckLegacyCloudWatchRunCommandConfig_InvalidFormat(t *testing.T) {
	var hasConfiguration bool
	var testCloudWatchDocument string
	var err error

	testCloudWatchDocument, err = fileutil.ReadAllText(filepath.Join(testRepoRoot, invalidTestCloudWatchDocumentName))
	if err != nil {
		fmt.Printf("error: %v", err)
	}

	// Setup mock with expectations
	mockCwcInstance := MockedCwcInstance{}

	mockFileSysUtil := MockedFileSysUtil{}
	mockFileSysUtil.On("Exists", legacyCwConfigStorePath).Return(true).Once()
	mockFileSysUtil.On("ReadFile", legacyCwConfigStorePath).Return([]byte(testCloudWatchDocument), nil).Once()

	hasConfiguration, err = checkLegacyCloudWatchRunCommandConfig(instanceId, &mockCwcInstance, &mockFileSysUtil)

	assert.False(t, hasConfiguration)
	assert.NotNil(t, err)
	mockCwcInstance.AssertExpectations(t)
	mockFileSysUtil.AssertExpectations(t)
}

func TestCheckLegacyCloudWatchRunCommandConfig_InvalidFormat2(t *testing.T) {
	var hasConfiguration bool
	var testCloudWatchDocument string
	var err error

	testCloudWatchDocument, err = fileutil.ReadAllText(filepath.Join(testRepoRoot, invalidTestCloudWatchDocumentName2))
	if err != nil {
		fmt.Printf("error: %v", err)
	}

	// Setup mock with expectations
	mockCwcInstance := MockedCwcInstance{}

	mockFileSysUtil := MockedFileSysUtil{}
	mockFileSysUtil.On("Exists", legacyCwConfigStorePath).Return(true).Once()
	mockFileSysUtil.On("ReadFile", legacyCwConfigStorePath).Return([]byte(testCloudWatchDocument), nil).Once()

	hasConfiguration, err = checkLegacyCloudWatchRunCommandConfig(instanceId, &mockCwcInstance, &mockFileSysUtil)

	assert.False(t, hasConfiguration)
	assert.NotNil(t, err)
	mockCwcInstance.AssertExpectations(t)
	mockFileSysUtil.AssertExpectations(t)
}

func TestCheckLegacyCloudWatchRunCommandConfig_ConfigFileMissing(t *testing.T) {
	var hasConfiguration bool
	var err error

	// Setup mock with expectations
	mockCwcInstance := MockedCwcInstance{}

	mockFileSysUtil := MockedFileSysUtil{}
	mockFileSysUtil.On("Exists", legacyCwConfigStorePath).Return(false).Once()

	hasConfiguration, err = checkLegacyCloudWatchRunCommandConfig(instanceId, &mockCwcInstance, &mockFileSysUtil)

	assert.False(t, hasConfiguration)
	assert.Nil(t, err)
	mockCwcInstance.AssertExpectations(t)
	mockFileSysUtil.AssertExpectations(t)
}

func TestCheckLegacyCloudWatchRunCommandConfig_FileReadFailure(t *testing.T) {
	var hasConfiguration bool
	var err error

	// Setup mock with expectations
	mockCwcInstance := MockedCwcInstance{}

	mockFileSysUtil := MockedFileSysUtil{}
	mockFileSysUtil.On("Exists", legacyCwConfigStorePath).Return(true).Once()
	mockFileSysUtil.On("ReadFile", legacyCwConfigStorePath).Return([]byte(""), fmt.Errorf("Failed to read the file")).Once()

	hasConfiguration, err = checkLegacyCloudWatchRunCommandConfig(instanceId, &mockCwcInstance, &mockFileSysUtil)

	assert.False(t, hasConfiguration)
	assert.NotNil(t, err)
	mockCwcInstance.AssertExpectations(t)
	mockFileSysUtil.AssertExpectations(t)
}

func TestCheckLegacyCloudWatchRunCommandConfig_EnableFailure(t *testing.T) {
	var hasConfiguration bool
	var testCloudWatchDocument string
	var err error

	testCloudWatchDocument, err = fileutil.ReadAllText(filepath.Join(testRepoRoot, testCloudWatchDocumentName))
	if err != nil {
		fmt.Printf("error: %v", err)
	}

	// Setup mock with expectations
	mockCwcInstance := MockedCwcInstance{}
	mockCwcInstance.On("Enable", mock.Anything).Return(fmt.Errorf("Failed to enable cw configuration")).Once()

	mockFileSysUtil := MockedFileSysUtil{}
	mockFileSysUtil.On("Exists", legacyCwConfigStorePath).Return(true).Once()
	mockFileSysUtil.On("ReadFile", legacyCwConfigStorePath).Return([]byte(testCloudWatchDocument), nil).Once()

	hasConfiguration, err = checkLegacyCloudWatchRunCommandConfig(instanceId, &mockCwcInstance, &mockFileSysUtil)

	assert.False(t, hasConfiguration)
	assert.NotNil(t, err)
	mockCwcInstance.AssertExpectations(t)
	mockFileSysUtil.AssertExpectations(t)
}

/*
 *	Tests for checkLegacyCloudWatchLocalConfig
 */
func TestCheckLegacyCloudWatchLocalConfig(t *testing.T) {
	var hasLocalConfiguration bool
	var testCloudWatchDocument string
	var err error

	testCloudWatchDocument, err = fileutil.ReadAllText(filepath.Join(testRepoRoot, testCloudWatchConfig))
	if err != nil {
		fmt.Printf("error: %v", err)
	}

	// Setup mock with expectations
	mockEc2ConfigXmlParser := MockedEc2ConfigXmlParser{}
	mockEc2ConfigXmlParser.On("IsCloudWatchEnabled").Return(true, nil).Once()

	mockFileSysUtil := MockedFileSysUtil{}
	mockFileSysUtil.On("Exists", legacyCwConfigPath).Return(true).Once()
	mockFileSysUtil.On("ReadFile", legacyCwConfigPath).Return([]byte(testCloudWatchDocument), nil).Once()

	mockCwcInstance := MockedCwcInstance{}
	mockCwcInstance.On("Enable", mock.Anything).Return(nil).Once()

	hasLocalConfiguration, err = checkLegacyCloudWatchLocalConfig(&mockCwcInstance, &mockEc2ConfigXmlParser, &mockFileSysUtil)

	assert.True(t, hasLocalConfiguration)
	assert.Nil(t, err)
	mockEc2ConfigXmlParser.AssertExpectations(t)
	mockFileSysUtil.AssertExpectations(t)
	mockCwcInstance.AssertExpectations(t)
}

func TestCheckLegacyCloudWatchLocalConfig_CloudWatchDisabled(t *testing.T) {
	var hasLocalConfiguration bool
	var err error

	// Setup mock with expectations
	mockEc2ConfigXmlParser := MockedEc2ConfigXmlParser{}
	mockEc2ConfigXmlParser.On("IsCloudWatchEnabled").Return(false, nil).Once()

	mockFileSysUtil := MockedFileSysUtil{}
	mockCwcInstance := MockedCwcInstance{}

	hasLocalConfiguration, err = checkLegacyCloudWatchLocalConfig(&mockCwcInstance, &mockEc2ConfigXmlParser, &mockFileSysUtil)

	assert.False(t, hasLocalConfiguration)
	assert.Nil(t, err)
	mockEc2ConfigXmlParser.AssertExpectations(t)
	mockFileSysUtil.AssertExpectations(t)
	mockCwcInstance.AssertExpectations(t)
}

func TestCheckLegacyCloudWatchLocalConfig_InvalidEc2ConfigXmlFormat(t *testing.T) {
	var hasLocalConfiguration bool
	var err error

	// Setup mock with expectations
	mockEc2ConfigXmlParser := MockedEc2ConfigXmlParser{}
	mockEc2ConfigXmlParser.On("IsCloudWatchEnabled").Return(false, fmt.Errorf("Invalid format")).Once()

	mockFileSysUtil := MockedFileSysUtil{}
	mockCwcInstance := MockedCwcInstance{}

	hasLocalConfiguration, err = checkLegacyCloudWatchLocalConfig(&mockCwcInstance, &mockEc2ConfigXmlParser, &mockFileSysUtil)

	assert.False(t, hasLocalConfiguration)
	assert.NotNil(t, err)
	mockEc2ConfigXmlParser.AssertExpectations(t)
	mockFileSysUtil.AssertExpectations(t)
	mockCwcInstance.AssertExpectations(t)
}

func TestCheckLegacyCloudWatchLocalConfig_ConfigFileMissing(t *testing.T) {
	var hasLocalConfiguration bool
	var err error

	// Setup mock with expectations
	mockEc2ConfigXmlParser := MockedEc2ConfigXmlParser{}
	mockEc2ConfigXmlParser.On("IsCloudWatchEnabled").Return(true, nil).Once()

	mockFileSysUtil := MockedFileSysUtil{}
	mockFileSysUtil.On("Exists", legacyCwConfigPath).Return(false).Once()

	mockCwcInstance := MockedCwcInstance{}

	hasLocalConfiguration, err = checkLegacyCloudWatchLocalConfig(&mockCwcInstance, &mockEc2ConfigXmlParser, &mockFileSysUtil)

	assert.False(t, hasLocalConfiguration)
	assert.Nil(t, err)
	mockEc2ConfigXmlParser.AssertExpectations(t)
	mockFileSysUtil.AssertExpectations(t)
	mockCwcInstance.AssertExpectations(t)
}

func TestCheckLegacyCloudWatchLocalConfig_FileReadFailure(t *testing.T) {
	var hasLocalConfiguration bool
	var err error

	// Setup mock with expectations
	mockEc2ConfigXmlParser := MockedEc2ConfigXmlParser{}
	mockEc2ConfigXmlParser.On("IsCloudWatchEnabled").Return(true, nil).Once()

	mockFileSysUtil := MockedFileSysUtil{}
	mockFileSysUtil.On("Exists", legacyCwConfigPath).Return(true).Once()
	mockFileSysUtil.On("ReadFile", legacyCwConfigPath).Return([]byte(""), fmt.Errorf("Failed to read the file")).Once()

	mockCwcInstance := MockedCwcInstance{}

	hasLocalConfiguration, err = checkLegacyCloudWatchLocalConfig(&mockCwcInstance, &mockEc2ConfigXmlParser, &mockFileSysUtil)

	assert.False(t, hasLocalConfiguration)
	assert.NotNil(t, err)
	mockEc2ConfigXmlParser.AssertExpectations(t)
	mockFileSysUtil.AssertExpectations(t)
	mockCwcInstance.AssertExpectations(t)
}

func TestCheckLegacyCloudWatchLocalConfig_EnableFailure(t *testing.T) {
	var hasLocalConfiguration bool
	var testCloudWatchDocument string
	var err error

	testCloudWatchDocument, err = fileutil.ReadAllText(filepath.Join(testRepoRoot, testCloudWatchConfig))
	if err != nil {
		fmt.Printf("error: %v", err)
	}
	// Setup mock with expectations
	mockEc2ConfigXmlParser := MockedEc2ConfigXmlParser{}
	mockEc2ConfigXmlParser.On("IsCloudWatchEnabled").Return(true, nil).Once()

	mockFileSysUtil := MockedFileSysUtil{}
	mockFileSysUtil.On("Exists", legacyCwConfigPath).Return(true).Once()
	mockFileSysUtil.On("ReadFile", legacyCwConfigPath).Return([]byte(testCloudWatchDocument), nil).Once()

	mockCwcInstance := MockedCwcInstance{}
	mockCwcInstance.On("Enable", mock.Anything).Return(fmt.Errorf("Failed to enable cw configuration")).Once()

	hasLocalConfiguration, err = checkLegacyCloudWatchLocalConfig(&mockCwcInstance, &mockEc2ConfigXmlParser, &mockFileSysUtil)

	assert.False(t, hasLocalConfiguration)
	assert.NotNil(t, err)
	mockEc2ConfigXmlParser.AssertExpectations(t)
	mockFileSysUtil.AssertExpectations(t)
	mockCwcInstance.AssertExpectations(t)
}

/*
 *	Mocks
 */
type MockedCwcInstance struct {
	mock.Mock
	IsEnabled           bool        `json:"IsEnabled"`
	EngineConfiguration interface{} `json:"EngineConfiguration"`
}

func (m *MockedCwcInstance) GetIsEnabled() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockedCwcInstance) ParseEngineConfiguration() (config string, err error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockedCwcInstance) Update(log log.T) error {
	args := m.Called(log)
	return args.Error(0)
}

func (m *MockedCwcInstance) Write() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockedCwcInstance) Enable(engineConfiguration interface{}) error {
	args := m.Called(engineConfiguration)
	return args.Error(0)
}

func (m *MockedCwcInstance) Disable() error {
	args := m.Called()
	return args.Error(0)
}

type MockedFileSysUtil struct {
	mock.Mock
}

func (m *MockedFileSysUtil) Exists(filePath string) bool {
	args := m.Called(filePath)
	return args.Bool(0)
}

func (m *MockedFileSysUtil) MakeDirs(destinationDir string) error {
	args := m.Called(destinationDir)
	return args.Error(0)
}

func (m *MockedFileSysUtil) WriteIntoFileWithPermissions(absolutePath, content string, perm os.FileMode) (bool, error) {
	args := m.Called(absolutePath, content, perm)
	return args.Bool(0), args.Error(1)
}

func (m *MockedFileSysUtil) ReadFile(filename string) ([]byte, error) {
	args := m.Called(filename)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockedFileSysUtil) ReadAll(r io.Reader) ([]byte, error) {
	args := m.Called(r)
	return args.Get(0).([]byte), args.Error(1)
}

type MockedEc2ConfigXmlParser struct {
	mock.Mock
	FileUtilWrapper longrunning.FileSysUtil
}

func (m *MockedEc2ConfigXmlParser) IsCloudWatchEnabled() (bool, error) {
	args := m.Called()
	return args.Bool(0), args.Error(1)
}
