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

package windowscontainerutil

import (
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/mock"
)

func init() {
	dep = &DepMock{}
}

type DepMock struct {
	mock.Mock
}

func (m *DepMock) PlatformVersion(log log.T) (version string, err error) {
	args := m.Called(log)
	return args.String(0), args.Error(1)
}

func (m *DepMock) IsPlatformNanoServer(log log.T) (bool, error) {
	args := m.Called(log)
	return args.Bool(0), args.Error(1)
}

func (m *DepMock) SetDaemonConfig(daemonConfigPath string, daemonConfigContent string) (err error) {
	args := m.Called(daemonConfigPath, daemonConfigContent)
	return args.Error(0)
}

func (m *DepMock) FileutilUncompress(log log.T, src, dest string) error {
	args := m.Called(log, src, dest)
	return args.Error(0)
}

func (m *DepMock) MakeDirs(destinationDir string) (err error) {
	args := m.Called(destinationDir)
	return args.Error(0)
}

func (m *DepMock) TempDir(dir, prefix string) (name string, err error) {
	args := m.Called(dir, prefix)
	return args.String(0), args.Error(1)
}

func (m *DepMock) LocalRegistryKeySetDWordValue(path string, name string, value uint32) error {
	args := m.Called(name, value)
	return args.Error(0)
}

func (m *DepMock) LocalRegistryKeyGetStringValue(path string, name string) (val string, valtype uint32, err error) {
	args := m.Called(name)
	return args.String(0), uint32(args.Int(1)), args.Error(2)
}

func (m *DepMock) UpdateUtilExeCommandOutput(
	customUpdateExecutionTimeoutInSeconds int,
	log log.T,
	cmd string,
	parameters []string,
	workingDir string,
	outputRoot string,
	stdOut string,
	stdErr string,
	usePlatformSpecificCommand bool) (output string, err error) {
	args := m.Called(log, cmd, parameters, workingDir, outputRoot, stdOut, stdErr, usePlatformSpecificCommand)
	return args.String(0), args.Error(1)
}

func (m *DepMock) ArtifactDownload(log log.T, input artifact.DownloadInput) (output artifact.DownloadOutput, err error) {
	args := m.Called(log, input)
	return args.Get(0).(artifact.DownloadOutput), args.Error(1)
}
