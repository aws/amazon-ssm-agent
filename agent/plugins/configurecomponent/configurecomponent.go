// Package configurecomponent implements the ConfigureComponent plugin.
package configurecomponent

import (
	"errors"
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/executers"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
)

// Plugin is the type for the configurecomponent plugin
type Plugin struct {
	pluginutil.DefaultPlugin
}

// ConfigureComponentPluginInput represents one set of commands executed by the ConfigureComponent plugin
type ConfigureComponentPluginInput struct {
	contracts.PluginInput
	Name         string
	Version      string
	Platform     string
	Architecture string
	Action       string
}

// ConfigureComponentsPluginOutput represents the output of the plugin
type ConfigureComponentPluginOutput struct {
	contracts.PluginOutput
}

// SMarkAsSucceeded marks plugin as Successful
func (result *ConfigureComponentPluginOutput) MarkAsSucceeded() {
	result.ExitCode = 0
	result.Status = contracts.ResultStatusSuccess
}

// MarkAsFailed marks plugin as Failed
func (result *ConfigureComponentPluginOutput) MarkAsFailed(log log.T, err error) {
	result.ExitCode = 1
	result.Status = contracts.ResultStatusFailed
	if len(result.Stderr) != 0 {
		result.Stderr = fmt.Sprintf("\n%v\n%v", result.Stderr, err.Error())
	} else {
		result.Stderr = fmt.Sprintf("\n%v", err.Error())
	}
	log.Error(err.Error())
	result.Errors = append(result.Errors, err.Error())
}

// AppendInfo adds info to ConfigureComponentPluginOutput StandardOut
func (result *ConfigureComponentPluginOutput) AppendInfo(log log.T, format string, params ...interface{}) {
	message := fmt.Sprintf(format, params...)
	log.Info(message)
	result.Stdout = fmt.Sprintf("%v\n%v", result.Stdout, message)
}

// NewPlugin returns a new instance of the plugin.
func NewPlugin(pluginConfig pluginutil.PluginConfig) (*Plugin, error) {
	var plugin Plugin
	plugin.MaxStdoutLength = pluginConfig.MaxStdoutLength
	plugin.MaxStderrLength = pluginConfig.MaxStderrLength
	plugin.StdoutFileName = pluginConfig.StdoutFileName
	plugin.StderrFileName = pluginConfig.StderrFileName
	plugin.OutputTruncatedSuffix = pluginConfig.OutputTruncatedSuffix
	plugin.Uploader = pluginutil.GetS3Config()
	plugin.ExecuteUploadOutputToS3Bucket = pluginutil.UploadOutputToS3BucketExecuter(plugin.UploadOutputToS3Bucket)

	exec := executers.ShellCommandExecuter{}
	plugin.ExecuteCommand = pluginutil.CommandExecuter(exec.Execute)

	return &plugin, nil
}

type configureManager struct{}

type pluginHelper interface {
	DownloadPackage(log log.T, input *ConfigureComponentPluginInput, output *ConfigureComponentPluginOutput, context *updateutil.InstanceContext) (err error)
}

var fileDownload = artifact.Download

// downloadPackage downloads the installation package from s3 bucket
func (m *configureManager) DownloadPackage(log log.T, util Util, input *ConfigureComponentPluginInput, output *ConfigureComponentPluginOutput, context *updateutil.InstanceContext) (err error) {
	// package to download
	packageName := CreatePackageName(input)

	// path to package
	packageLocation := CreatePackageLocation(input, context, packageName)

	// path to download destination
	packageDestination, err := util.CreateComponentFolder(input)
	if err != nil {
		return err
	}

	downloadInput := artifact.DownloadInput{
		SourceURL:            packageLocation,
		DestinationDirectory: packageDestination}

	// download package
	downloadOutput, downloadErr := fileDownload(log, downloadInput)
	if downloadErr != nil || downloadOutput.LocalFilePath == "" {
		errMessage := fmt.Sprintf("failed to download file reliably, %v", downloadInput.SourceURL)
		if downloadErr != nil {
			errMessage = fmt.Sprintf("%v, %v", errMessage, downloadErr.Error())
		}
		return errors.New(errMessage)
	}

	output.AppendInfo(log, "Successfully downloaded %v", downloadInput.SourceURL)

	return nil
}

// Name returns the name of the plugin
func Name() string {
	return appconfig.PluginNameAwsConfigureComponent
}
