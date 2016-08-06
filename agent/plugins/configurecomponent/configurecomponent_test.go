// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Amazon Software License (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/asl/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package configurecomponents implements the ConfigureComponent plugin.
package configurecomponent

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
)

var logger = log.NewMockLog()

func TestMarkAsSucceeded(t *testing.T) {
	output := ConfigureComponentPluginOutput{}

	output.MarkAsSucceeded()

	assert.Equal(t, output.ExitCode, 0)
	assert.Equal(t, output.Status, contracts.ResultStatusSuccess)
}

func TestMarkAsFailed(t *testing.T) {
	output := ConfigureComponentPluginOutput{}

	output.MarkAsFailed(logger, fmt.Errorf("Error message"))

	assert.Equal(t, output.ExitCode, 1)
	assert.Equal(t, output.Status, contracts.ResultStatusFailed)
	assert.Contains(t, output.Stderr, "Error message")
}

func TestAppendInfo(t *testing.T) {
	output := ConfigureComponentPluginOutput{}

	output.AppendInfo(logger, "Info message")

	assert.Contains(t, output.Stdout, "Info message")
}

func TestDownloadManifest(t *testing.T) {
	pluginInformation := createStubPluginInput()
	context := createStubInstanceContext()

	output := ConfigureComponentPluginOutput{}
	manager := configureManager{}
	util := mockUtility{}

	fileDownload = func(log log.T, input artifact.DownloadInput) (output artifact.DownloadOutput, err error) {
		result := artifact.DownloadOutput{}
		result.LocalFilePath = "testdata/sampleManifest.json"
		return result, nil
	}

	manifest, err := manager.downloadManifest(logger, &util, pluginInformation, &output, context)

	assert.NoError(t, err)
	assert.NotNil(t, manifest)
}

func TestDownloadPackage(t *testing.T) {
	pluginInformation := createStubPluginInput()
	context := createStubInstanceContext()

	output := ConfigureComponentPluginOutput{}
	manager := configureManager{}
	util := mockUtility{}

	fileDownload = func(log log.T, input artifact.DownloadInput) (output artifact.DownloadOutput, err error) {
		result := artifact.DownloadOutput{}
		result.LocalFilePath = "components/PVDriver/9000.0.0"
		return result, nil
	}

	err := manager.downloadPackage(logger, &util, pluginInformation, &output, context)

	assert.NoError(t, err)
}

func TestDownloadPackage_Failed(t *testing.T) {
	pluginInformation := createStubPluginInput()
	context := createStubInstanceContext()

	output := ConfigureComponentPluginOutput{}
	manager := configureManager{}
	util := mockUtility{}

	// file download failed
	fileDownload = func(log log.T, input artifact.DownloadInput) (output artifact.DownloadOutput, err error) {
		result := artifact.DownloadOutput{}
		result.LocalFilePath = ""
		return result, fmt.Errorf("404")
	}

	err := manager.downloadPackage(logger, &util, pluginInformation, &output, context)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to download component installation package reliably")
	assert.Contains(t, err.Error(), "404")
}
