// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the AWS Customer Agreement (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/agreement/
//
// Package pluginutil implements some common functions shared by multiple plugins.
package pluginutil

import (
	"io"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/executers"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/s3util"
	command_state_helper "github.com/aws/amazon-ssm-agent/agent/statemanager"
)

const (
	defaultExecutionTimeoutInSeconds = 3600
	maxExecutionTimeoutInSeconds     = 28800
	minExecutionTimeoutInSeconds     = 5
)

// UploadOutputToS3BucketExecuter is a function that can upload outputs to S3 bucket.
type UploadOutputToS3BucketExecuter func(log log.T, pluginID string, orchestrationDir string, outputS3BucketName string, outputS3KeyPrefix string, useTempDirectory bool, tempDir string, Stdout string, Stderr string) []string

// S3Uploader is an interface for objects that can upload data to s3.
type S3Uploader interface {
	S3Upload(log log.T, bucketName string, bucketKey string, filePath string) error
	UploadS3TestFile(log log.T, bucketName, key string) error
	IsS3ErrorRelatedToAccessDenied(errMsg string) bool
	IsS3ErrorRelatedToWrongBucketRegion(errMsg string) bool
	GetS3BucketRegionFromErrorMsg(log log.T, errMsg string) string
	GetS3ClientRegion() string
	SetS3ClientRegion(region string)
}

// DefaultPlugin is the type for the default plugin.
type DefaultPlugin struct {
	// ExecuteCommand is an object that can execute commands.
	CommandExecuter executers.T

	// ExecuteUploadOutputToS3Bucket is an object that can upload command outputs to S3 bucket.
	ExecuteUploadOutputToS3Bucket UploadOutputToS3BucketExecuter

	// Uploader is an object that can upload data to s3.
	Uploader S3Uploader

	// UploadToS3Sync is true if uploading to S3 should be done synchronously, false for async.
	UploadToS3Sync bool

	// StdoutFileName is the name of the file that stores standard output.
	StdoutFileName string

	// StderrFileName is the name of the file that stores standard error.
	StderrFileName string

	// MaxStdoutLength is the maximum lenght of the standard output returned in the plugin result.
	// If the output is longer, it will be truncated. The full output will be uploaded to s3.
	MaxStdoutLength int

	// MaxStderrLength is the maximum lenght of the standard error returned in the plugin result.
	MaxStderrLength int

	// OutputTruncatedSuffix is an optional suffix that is inserted at the end of the truncated stdout/stderr.
	OutputTruncatedSuffix string
}

// PluginConfig is used for initializing plugins with default values
type PluginConfig struct {
	StdoutFileName        string
	StderrFileName        string
	MaxStdoutLength       int
	MaxStderrLength       int
	OutputTruncatedSuffix string
}

// StringPrefix returns the beginning part of a string, truncated to the given limit.
func StringPrefix(input string, maxLength int, truncatedSuffix string) string {
	// no need to truncate
	if len(input) < maxLength {
		return input
	}

	// truncate and add suffix
	if maxLength > len(truncatedSuffix) {
		pos := maxLength - len(truncatedSuffix)
		return string(input[:pos]) + truncatedSuffix
	}

	// suffix longer than maxLength - return beginning of suffix
	return truncatedSuffix[:maxLength]
}

// ReadPrefix returns the beginning data from a given Reader, truncated to the given limit.
func ReadPrefix(input io.Reader, maxLength int, truncatedSuffix string) (out string, err error) {
	// read up to maxLength bytes from input
	data, err := ioutil.ReadAll(io.LimitReader(input, int64(maxLength)))
	if err != nil {
		return
	}

	out = StringPrefix(string(data), maxLength, truncatedSuffix)
	return
}

// ReadAll returns all data from a given Reader.
func ReadAll(input io.Reader, maxLength int, truncatedSuffix string) (out string, err error) {
	// read up to maxLength bytes from input
	data, err := ioutil.ReadAll(io.LimitReader(input, int64(maxLength)))
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// UploadOutputToS3Bucket uploads outputs (if any) to s3
func (p *DefaultPlugin) UploadOutputToS3Bucket(log log.T, pluginID string, orchestrationDir string,
	outputS3BucketName string, outputS3KeyPrefix string, useTempDirectory bool, tempDir string,
	Stdout string, Stderr string) []string {
	var uploadOutputToS3BucketErrors []string
	if outputS3BucketName != "" {

		uploadOutputsToS3 := func() {
			if useTempDirectory {
				// delete temp directory once we're done
				defer func() {
					if err := fileutil.DeleteDirectory(tempDir); err != nil {
						log.Error("Error deleting directory", err)
					}
				}()
			}
			if Stdout != "" {
				localPath := filepath.Join(orchestrationDir, p.StdoutFileName)
				s3Key := fileutil.BuildS3Path(outputS3KeyPrefix, pluginID, p.StdoutFileName)
				if err := s3util.NewAmazonS3Util(log, outputS3BucketName).S3Upload(log, outputS3BucketName, s3Key, localPath); err != nil && p.UploadToS3Sync {
					// if we are in synchronous mode, we can also return the error
					uploadOutputToS3BucketErrors = append(uploadOutputToS3BucketErrors, err.Error())
				}
			}
			if Stderr != "" {
				localPath := filepath.Join(orchestrationDir, p.StderrFileName)
				s3Key := fileutil.BuildS3Path(outputS3KeyPrefix, pluginID, p.StderrFileName)
				if err := s3util.NewAmazonS3Util(log, outputS3BucketName).S3Upload(log, outputS3BucketName, s3Key, localPath); err != nil && p.UploadToS3Sync {
					// if we are in synchronous mode, we can also return the error
					uploadOutputToS3BucketErrors = append(uploadOutputToS3BucketErrors, err.Error())
				}
			}
		}

		if p.UploadToS3Sync {
			uploadOutputsToS3()
		} else {
			go uploadOutputsToS3()
		}
	}

	//return out.Errors
	return uploadOutputToS3BucketErrors
}

// CreateScriptFile creates a script containing the given commands.
func CreateScriptFile(log log.T, scriptPath string, runCommand []string) (err error) {
	// write source commands to file
	_, err = fileutil.WriteIntoFileWithPermissions(scriptPath, strings.Join(runCommand, "\n")+"\n", appconfig.ReadWriteExecuteAccess)
	if err != nil {
		log.Errorf("failed to write runcommand scripts to file %v, err %v", scriptPath, err)
		return
	}

	return
}

// CreateScriptFileUTF8 creates a script containing the given commands.
func CreateScriptFileUTF8(log log.T, scriptPath string, runCommand []string) (err error) {
	// write source commands to file
	_, err = fileutil.WriteIntoFileWithPermissionsUTF8(scriptPath, strings.Join(runCommand, "\n")+"\n", appconfig.ReadWriteExecuteAccess)
	if err != nil {
		log.Errorf("failed to write runcommand scripts to file %v, err %v", scriptPath, err)
		return
	}

	return
}

// DownloadFileFromSource downloads file from source
func DownloadFileFromSource(log log.T, source string, sourceHash string, sourceHashType string) (artifact.DownloadOutput, error) {
	// download source and verify its integrity
	downloadInput := artifact.DownloadInput{
		SourceURL:       source,
		SourceHashValue: sourceHash,
		SourceHashType:  sourceHashType,
	}
	log.Debug("Downloading file")
	return artifact.Download(log, downloadInput)
}

// DefaultPluginConfig returns the default values for the plugin
func DefaultPluginConfig() PluginConfig {
	return PluginConfig{
		StdoutFileName:        "stdout",
		StderrFileName:        "stderr",
		MaxStdoutLength:       2500,
		MaxStderrLength:       2500,
		OutputTruncatedSuffix: "--output truncated--",
	}
}

// PersistPluginInformationToCurrent persists the plugin execution results
func PersistPluginInformationToCurrent(log log.T, pluginID string, config contracts.Configuration, res contracts.PluginResult) {
	//Every plugin should persist information inside the execute method.
	//At this point a plugin knows that an interim state is already stored in Current folder.
	//Plugin will continue to add data to the same file in Current folder
	messageIDSplit := strings.Split(config.MessageId, ".")
	instanceID := messageIDSplit[len(messageIDSplit)-1]

	pluginState := command_state_helper.GetPluginState(log,
		pluginID,
		config.BookKeepingFileName,
		instanceID,
		appconfig.DefaultLocationOfCurrent)

	if pluginState == nil {
		log.Errorf("failed to find plugin state with id %v", pluginID)
		return
	}

	//set plugin state's execution details
	pluginState.Configuration = config
	pluginState.Result = res

	command_state_helper.PersistPluginState(log,
		*pluginState,
		pluginID,
		config.BookKeepingFileName,
		instanceID,
		appconfig.DefaultLocationOfCurrent)
}

// LoadParametersAsList returns properties as a list and appropriate PluginResult if error is encountered
func LoadParametersAsList(log log.T, prop interface{}) ([]interface{}, contracts.PluginResult) {

	var properties []interface{}
	var res contracts.PluginResult

	switch prop := prop.(type) {
	case []interface{}:
		if err := jsonutil.Remarshal(prop, &properties); err != nil {
			log.Errorf("unable to parse plugin configuration")
			res.Output = "Execution failed because agent is unable to parse plugin configuration"
			res.Code = 1
			res.Status = contracts.ResultStatusFailed
		}
	default:
		properties = append(properties, prop)
	}

	return properties, res
}

// LoadParametersAsMap returns properties as a map and appropriate PluginResult if error is encountered
func LoadParametersAsMap(log log.T, prop interface{}) (map[string]interface{}, contracts.PluginResult) {
	var properties map[string]interface{}
	var res contracts.PluginResult

	if err := jsonutil.Remarshal(prop, &properties); err != nil {
		log.Errorf("unable to parse plugin configuration")
		res.Output = "Execution failed because agent is unable to parse plugin configuration"
		res.Code = 1
		res.Status = contracts.ResultStatusFailed
	}

	return properties, res
}

// ValidateExecutionTimeout validates the supplied input interface and converts it into a valid int value.
func ValidateExecutionTimeout(log log.T, input interface{}) int {
	var num int

	switch input.(type) {
	case string:
		num = extractIntFromString(log, input.(string))
	case int:
		num = input.(int)
	case float64:
		f := input.(float64)
		num = int(f)
		log.Infof("Unexpected 'TimeoutSeconds' float value %v received. Applying 'TimeoutSeconds' as %v", f, num)
	default:
		log.Errorf("Unexpected 'TimeoutSeconds' value %v received. Setting 'TimeoutSeconds' to default value %v", input, defaultExecutionTimeoutInSeconds)
	}

	if num < minExecutionTimeoutInSeconds || num > maxExecutionTimeoutInSeconds {
		log.Infof("'TimeoutSeconds' value should be between %v and %v. Setting 'TimeoutSeconds' to default value %v", minExecutionTimeoutInSeconds, maxExecutionTimeoutInSeconds, defaultExecutionTimeoutInSeconds)
		num = defaultExecutionTimeoutInSeconds
	}
	return num
}

// ParseRunCommand checks the command type and convert it to the string array
func ParseRunCommand(input interface{}, output []string) []string {
	switch value := input.(type) {
	case string:
		output = append(output, value)
	case []interface{}:
		for _, element := range value {
			output = ParseRunCommand(element, output)
		}
	}
	return output
}

// extractIntFromString extracts a valid int value from a string.
func extractIntFromString(log log.T, input string) int {
	var iNum int
	var fNum float64
	var err error

	iNum, err = strconv.Atoi(input)
	if err == nil {
		return iNum
	}

	fNum, err = strconv.ParseFloat(input, 64)
	if err == nil {
		iNum = int(fNum)
		log.Infof("Unexpected 'TimeoutSeconds' float value %v received. Applying 'TimeoutSeconds' as %v", fNum, iNum)
	} else {
		log.Errorf("Unexpected 'TimeoutSeconds' string value %v received. Setting 'TimeoutSeconds' to default value %v", input, defaultExecutionTimeoutInSeconds)
		iNum = defaultExecutionTimeoutInSeconds
	}
	return iNum
}

// GetProxySetting returns proxy setting from registry entries
func GetProxySetting(proxyValue []string) (string, string) {
	var url string
	var noProxy string

	for _, proxy := range proxyValue {
		tmp := strings.TrimSpace(proxy)
		parts := strings.Split(tmp, "=")
		switch parts[0] {
		case "http_proxy":
			url = parts[1]
		case "no_proxy":
			noProxy = parts[1]
		}
	}

	return url, noProxy
}
