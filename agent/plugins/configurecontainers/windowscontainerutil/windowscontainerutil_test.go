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
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var loggerMock = log.NewMockLog()

func successMock() *DepMock {
	depmock := DepMock{}
	depmock.On("PlatformVersion", mock.Anything).Return("10", nil)
	depmock.On("IsPlatformNanoServer", mock.Anything).Return(false, nil)
	depmock.On("SetDaemonConfig", mock.Anything, mock.Anything).Return(nil)
	depmock.On("MakeDirs", mock.Anything).Return(nil)
	depmock.On("TempDir", mock.Anything, mock.Anything).Return("test", nil)
	depmock.On("UpdateUtilExeCommandOutput", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("True", nil)
	depmock.On("ArtifactDownload", mock.Anything, mock.Anything).Return(artifact.DownloadOutput{}, nil)
	depmock.On("LocalRegistryKeySetDWordValue", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	depmock.On("LocalRegistryKeyGetStringValue", mock.Anything, mock.Anything).Return("", 0, nil)
	depmock.On("FileutilUncompress", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	return &depmock
}

func TestInstall(t *testing.T) {
	depOrig := dep
	containerMock := successMock()
	dep = containerMock
	defer func() { dep = depOrig }()

	output := iohandler.DefaultIOHandler{}
	RunInstallCommands(loggerMock, "", &output)

	assert.Equal(t, output.GetExitCode(), 0)
	assert.Contains(t, output.GetStdout(), "Installation complete")
	containerMock.AssertCalled(t, "PlatformVersion", mock.Anything)
	containerMock.AssertCalled(t, "IsPlatformNanoServer", mock.Anything)
	containerMock.AssertNumberOfCalls(t, "UpdateUtilExeCommandOutput", 4)
}
