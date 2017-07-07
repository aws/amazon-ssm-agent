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

// Package cloudwatch implements cloudwatch plugin and its configuration
package cloudwatch

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/longrunning"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const testRepoRoot = "testdata"
const testCwEnabledEc2ConfigXml = "ec2config_cloudwatch_enabled.xml"
const testCwDisabledEc2ConfigXml = "ec2config_cloudwatch_disabled.xml"
const invalidTestEc2ConfigXml = "ec2config_invalid_format.xml"
const testCwEc2ConfigXmlWithInvalidValue = "ec2config_cloudwatch_invalid_value.xml"
const testCwEc2ConfigXmlWithNoState = "ec2config_cloudwatch_no_state.xml"

var (
	ec2ConfigXmlPath = fileutil.BuildPath(
		appconfig.EC2ConfigSettingPath,
		EC2ServiceConfigFileName)
)

func TestIsCloudWatchEnabled(t *testing.T) {
	var configContent string
	var err error

	isCloudWatchEnabled := false

	configContent, err = fileutil.ReadAllText(filepath.Join(testRepoRoot, testCwEnabledEc2ConfigXml))
	if err != nil {
		fmt.Printf("error: %v", err)
	}

	mockFileSysUtil := MockedFileSysUtil{}
	mockFileSysUtil.On("Exists", ec2ConfigXmlPath).Return(true).Once()
	mockFileSysUtil.On("ReadFile", ec2ConfigXmlPath).Return([]byte(configContent), nil).Once()

	ec2ConfigXmlParser := &Ec2ConfigXmlParserImpl{
		FileSysUtil: &mockFileSysUtil,
	}

	isCloudWatchEnabled, err = ec2ConfigXmlParser.IsCloudWatchEnabled()

	assert.True(t, isCloudWatchEnabled)
	assert.Nil(t, err)
	mockFileSysUtil.AssertExpectations(t)
}

func TestIsCloudWatchEnabled_CloudWatchDisabled(t *testing.T) {
	var configContent string
	var err error

	isCloudWatchEnabled := false

	configContent, err = fileutil.ReadAllText(filepath.Join(testRepoRoot, testCwDisabledEc2ConfigXml))
	if err != nil {
		fmt.Printf("error: %v", err)
	}

	mockFileSysUtil := MockedFileSysUtil{}
	mockFileSysUtil.On("Exists", ec2ConfigXmlPath).Return(true).Once()
	mockFileSysUtil.On("ReadFile", ec2ConfigXmlPath).Return([]byte(configContent), nil).Once()

	ec2ConfigXmlParser := &Ec2ConfigXmlParserImpl{
		FileSysUtil: &mockFileSysUtil,
	}

	isCloudWatchEnabled, err = ec2ConfigXmlParser.IsCloudWatchEnabled()

	assert.False(t, isCloudWatchEnabled)
	assert.Nil(t, err)
	mockFileSysUtil.AssertExpectations(t)
}

func TestIsCloudWatchEnabled_ConfigMissing(t *testing.T) {
	var err error

	isCloudWatchEnabled := false

	mockFileSysUtil := MockedFileSysUtil{}
	mockFileSysUtil.On("Exists", ec2ConfigXmlPath).Return(false).Once()

	ec2ConfigXmlParser := &Ec2ConfigXmlParserImpl{
		FileSysUtil: &mockFileSysUtil,
	}

	isCloudWatchEnabled, err = ec2ConfigXmlParser.IsCloudWatchEnabled()

	assert.False(t, isCloudWatchEnabled)
	assert.Nil(t, err)
	mockFileSysUtil.AssertExpectations(t)
}

func TestIsCloudWatchEnabled_InvalidFormat(t *testing.T) {
	var configContent string
	var err error

	isCloudWatchEnabled := false

	configContent, err = fileutil.ReadAllText(filepath.Join(testRepoRoot, invalidTestEc2ConfigXml))
	if err != nil {
		fmt.Printf("error: %v", err)
	}

	mockFileSysUtil := MockedFileSysUtil{}
	mockFileSysUtil.On("Exists", ec2ConfigXmlPath).Return(true).Once()
	mockFileSysUtil.On("ReadFile", ec2ConfigXmlPath).Return([]byte(configContent), nil).Once()

	ec2ConfigXmlParser := &Ec2ConfigXmlParserImpl{
		FileSysUtil: &mockFileSysUtil,
	}

	isCloudWatchEnabled, err = ec2ConfigXmlParser.IsCloudWatchEnabled()

	assert.False(t, isCloudWatchEnabled)
	assert.NotNil(t, err)
	mockFileSysUtil.AssertExpectations(t)
}

func TestIsCloudWatchEnabled_InvalidValue(t *testing.T) {
	var configContent string
	var err error

	isCloudWatchEnabled := false

	configContent, err = fileutil.ReadAllText(filepath.Join(testRepoRoot, testCwEc2ConfigXmlWithInvalidValue))
	if err != nil {
		fmt.Printf("error: %v", err)
	}

	mockFileSysUtil := MockedFileSysUtil{}
	mockFileSysUtil.On("Exists", ec2ConfigXmlPath).Return(true).Once()
	mockFileSysUtil.On("ReadFile", ec2ConfigXmlPath).Return([]byte(configContent), nil).Once()

	ec2ConfigXmlParser := &Ec2ConfigXmlParserImpl{
		FileSysUtil: &mockFileSysUtil,
	}

	isCloudWatchEnabled, err = ec2ConfigXmlParser.IsCloudWatchEnabled()

	assert.False(t, isCloudWatchEnabled)
	assert.NotNil(t, err)
	mockFileSysUtil.AssertExpectations(t)
}

func TestIsCloudWatchEnabled_NoState(t *testing.T) {
	var configContent string
	var err error

	isCloudWatchEnabled := false

	configContent, err = fileutil.ReadAllText(filepath.Join(testRepoRoot, testCwEc2ConfigXmlWithNoState))
	if err != nil {
		fmt.Printf("error: %v", err)
	}

	mockFileSysUtil := MockedFileSysUtil{}
	mockFileSysUtil.On("Exists", ec2ConfigXmlPath).Return(true).Once()
	mockFileSysUtil.On("ReadFile", ec2ConfigXmlPath).Return([]byte(configContent), nil).Once()

	ec2ConfigXmlParser := &Ec2ConfigXmlParserImpl{
		FileSysUtil: &mockFileSysUtil,
	}

	isCloudWatchEnabled, err = ec2ConfigXmlParser.IsCloudWatchEnabled()

	assert.False(t, isCloudWatchEnabled)
	assert.NotNil(t, err)
	mockFileSysUtil.AssertExpectations(t)
}

func TestIsCloudWatchEnabled_ReadFileFailure(t *testing.T) {
	var err error

	isCloudWatchEnabled := false

	mockFileSysUtil := MockedFileSysUtil{}
	mockFileSysUtil.On("Exists", ec2ConfigXmlPath).Return(true).Once()
	mockFileSysUtil.On("ReadFile", ec2ConfigXmlPath).Return([]byte(""), fmt.Errorf("Failed to read the file")).Once()

	ec2ConfigXmlParser := &Ec2ConfigXmlParserImpl{
		FileSysUtil: &mockFileSysUtil,
	}

	isCloudWatchEnabled, err = ec2ConfigXmlParser.IsCloudWatchEnabled()

	assert.False(t, isCloudWatchEnabled)
	assert.NotNil(t, err)
	mockFileSysUtil.AssertExpectations(t)
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
	FileSysUtil longrunning.FileSysUtil
}

func (m *MockedEc2ConfigXmlParser) IsCloudWatchEnabled() (bool, error) {
	args := m.Called()
	return args.Bool(0), args.Error(1)
}
