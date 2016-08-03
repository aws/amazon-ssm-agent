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

	err := manager.DownloadPackage(logger, &util, pluginInformation, &output, context)

	assert.NoError(t, err)
}
