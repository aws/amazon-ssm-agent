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
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	command_state_helper "github.com/aws/amazon-ssm-agent/agent/message/statemanager"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/s3util"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

const (
	defaultExecutionTimeoutInSeconds = 3600
	maxExecutionTimeoutInSeconds     = 28800
	minExecutionTimeoutInSeconds     = 5
)

// S3RegionUSStandard is a standard S3 Region used to upload output related documents.
var S3RegionUSStandard = "us-east-1"

var s3Bjs = "cn-north-1"

var s3BjsEndpoint = "s3.cn-north-1.amazonaws.com.cn"

var s3StandardEndpoint = "s3.amazonaws.com"

// CommandExecuter is a function that can execute a set of commands.
type CommandExecuter func(log log.T, workingDir string, stdoutFilePath string, stderrFilePath string, cancelFlag task.CancelFlag, executionTimeout int, commandName string, commandArguments []string) (stdout io.Reader, stderr io.Reader, exitCode int, errs []error)

// UploadOutputToS3BucketExecuter is a function that can upload outputs to S3 bucket.
type UploadOutputToS3BucketExecuter func(log log.T, pluginID string, orchestrationDir string, outputS3BucketName string, outputS3KeyPrefix string, useTempDirectory bool, tempDir string, Stdout string, Stderr string) []string

// S3Uploader is an interface for objects that can upload data to s3.
type S3Uploader interface {
	S3Upload(bucketName string, bucketKey string, filePath string) error
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
	ExecuteCommand CommandExecuter

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

// DefaultConfig is used for initializing plugins with default values
type PluginConfig struct {
	StdoutFileName        string
	StderrFileName        string
	MaxStdoutLength       int
	MaxStderrLength       int
	OutputTruncatedSuffix string
}

// ReadPrefix returns the beginning data from a given Reader, truncated to the given limit.
func ReadPrefix(input io.Reader, maxLength int, truncatedSuffix string) (out string, err error) {
	// read up to maxLength bytes from input
	data, err := ioutil.ReadAll(io.LimitReader(input, int64(maxLength)))
	if err != nil {
		return
	}

	// no need to truncate
	if len(data) < maxLength {
		out = string(data)
		return
	}

	// truncate and add suffix
	if maxLength > len(truncatedSuffix) {
		pos := maxLength - len(truncatedSuffix)
		out = string(data[:pos]) + truncatedSuffix
		return
	}

	// suffix longer than maxLength - return beginning of suffix
	out = truncatedSuffix[:maxLength]
	return
}

// GetS3Config returns the S3 config used for uploading output files to S3
func GetS3Config() *s3util.Manager {
	//There are multiple ways of supporting the cross-region upload to S3 bucket:
	//1) We can specify the url https://s3.amazonaws.com and not specify region in our s3 client. This approach only works in java & .net but not in golang
	//since it enforces to use region in our client.
	//2) We can make use of GetBucketLocation API to find the location of S3 bucket. This is a better way to handle this, however it has its own disadvantages:
	//-> We will have to update the managed policy of AmazonEC2RoleforSSM so that agent will have permissions to make that call.
	//-> We will still have to notify our customers regarding the change in our IAM policy - such that customers using inline policy will also make the change accordingly.
	//3) Special behavior for S3 PutObject API for IAD region which is described in detail below.
	//We have taken the 3rd option - until the changes for the 2nd option is in place.

	//In our current implementation, we upload a test S3 file and use the error message to determine the bucket's region,
	//but we do this with region set as "us-east-1". This is because of special behavior of S3 PutObject API:
	//Only for the endpoint "us-east-1", if the bucket is present in any other region (i.e non IAD bucket) PutObject API will throw an
	//error of type - AuthorizationHeaderMalformed with a message stating which region is the bucket present. A sample error message looks like:
	//AuthorizationHeaderMalformed: The authorization header is malformed; the region 'us-east-1' is wrong; expecting 'us-west-2' status code: 400, request id: []

	//We leverage the above error message to determine the bucket's region, and if there is no error - that means the bucket is indeed in IAD.

	//Note: The above behavior only exists for IAD endpoint (special endpoint for S3) - not just any other region.
	//For other region endpoints, you get a BucketRegionError which is not useful for us in determining where the bucket is present.
	//Revisit this if S3 ensures the PutObject API behavior consistent over all endpoints - in which case - instead of using IAD endpoint,
	//we can then pick the endpoint from meta-data instead.

	awsConfig := sdkutil.AwsConfig()

	if region, err := platform.Region(); err == nil && region == s3Bjs {
		awsConfig.Endpoint = &s3BjsEndpoint
		awsConfig.Region = &s3Bjs
	} else {
		awsConfig.Endpoint = &s3StandardEndpoint
		awsConfig.Region = &S3RegionUSStandard
	}
	s3 := s3.New(session.New(awsConfig))
	return s3util.NewManager(s3)
}

// UploadOutputToS3Bucket uploads outputs (if any) to s3
func (p *DefaultPlugin) UploadOutputToS3Bucket(log log.T, pluginID string, orchestrationDir string, outputS3BucketName string, outputS3KeyPrefix string, useTempDirectory bool, tempDir string, Stdout string, Stderr string) []string {
	var uploadOutputToS3BucketErrors []string
	if outputS3BucketName != "" {
		uploadOutputsToS3 := func() {
			uploadToS3 := true
			var testUploadError error

			if region, err := platform.Region(); err == nil && region != s3Bjs {
				p.Uploader.SetS3ClientRegion(S3RegionUSStandard)
			}

			log.Infof("uploading a test file to s3 bucket - %v , s3 key - %v with S3Client using region endpoint - %v",
				outputS3BucketName,
				outputS3KeyPrefix,
				p.Uploader.GetS3ClientRegion())

			testUploadError = p.Uploader.UploadS3TestFile(log, outputS3BucketName, outputS3KeyPrefix)

			if testUploadError != nil {
				//Check if the error is related to Access Denied - i.e missing permissions
				if p.Uploader.IsS3ErrorRelatedToAccessDenied(testUploadError.Error()) {
					log.Debugf("encountered access denied related error - can't upload to S3 due to missing permissions -%v", testUploadError.Error())
					uploadToS3 = false
					//since we don't have permissions - no S3 calls will go through no matter what
				} else if p.Uploader.IsS3ErrorRelatedToWrongBucketRegion(testUploadError.Error()) { //check if error is related to different bucket region

					log.Debugf("encountered error related to wrong bucket region while uploading test file to S3 - %v. parsing the message to get expected region",
						testUploadError.Error())

					expectedBucketRegion := p.Uploader.GetS3BucketRegionFromErrorMsg(log, testUploadError.Error())

					//set the region to expectedBucketRegion
					p.Uploader.SetS3ClientRegion(expectedBucketRegion)
				} else {
					log.Debugf("encountered unexpected error while uploading test file to S3 - %v, no need to modify s3client", testUploadError.Error())
				}
			} else { //there were no errors while uploading a test file to S3 - our s3client should continue to use "us-east-1"

				log.Debugf("there were no errors while uploading a test file to S3 in region - %v. S3 client will continue to use region - %v",
					S3RegionUSStandard,
					p.Uploader.GetS3ClientRegion())
			}

			if uploadToS3 {
				log.Infof("uploading logs to S3 with client configured to use region - %v", p.Uploader.GetS3ClientRegion())

				if useTempDirectory {
					// delete temp directory once we're done
					defer DeleteDirectory(log, tempDir)
				}

				if Stdout != "" {
					localPath := filepath.Join(orchestrationDir, p.StdoutFileName)
					s3Key := path.Join(outputS3KeyPrefix, pluginID, p.StdoutFileName)
					log.Debugf("Uploading %v to s3://%v/%v", localPath, outputS3BucketName, s3Key)
					err := p.Uploader.S3Upload(outputS3BucketName, s3Key, localPath)
					if err != nil {

						log.Errorf("failed uploading %v to s3://%v/%v err:%v", localPath, outputS3BucketName, s3Key, err)
						if p.UploadToS3Sync {
							// if we are in synchronous mode, we can also return the error
							uploadOutputToS3BucketErrors = append(uploadOutputToS3BucketErrors, err.Error())
						}
					}
				}

				if Stderr != "" {
					localPath := filepath.Join(orchestrationDir, p.StderrFileName)
					s3Key := path.Join(outputS3KeyPrefix, pluginID, p.StderrFileName)
					log.Debugf("Uploading %v to s3://%v/%v", localPath, outputS3BucketName, s3Key)
					err := p.Uploader.S3Upload(outputS3BucketName, s3Key, localPath)
					if err != nil {
						log.Errorf("failed uploading %v to s3://%v/%v err:%v", localPath, outputS3BucketName, s3Key, err)
						if p.UploadToS3Sync {
							// if we are in synchronous mode, we can also return the error
							uploadOutputToS3BucketErrors = append(uploadOutputToS3BucketErrors, err.Error())
						}
					}
				}
			} else {
				//TODO:Bubble this up to engine - so that document level status reply can be sent stating no permissions to perform S3 upload - similar to ec2config
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

// DeleteDirectory deletes a directory and all its content.
func DeleteDirectory(log log.T, dirName string) {
	if err := os.RemoveAll(dirName); err != nil {
		log.Error("error deleting directory", err)
	}
}

// CreateScriptFile creates a script containing the given commands.
func CreateScriptFile(log log.T, scriptPath string, runCommand []string) (err error) {
	var sourceFile *os.File
	var scriptFile *os.File

	// create script file
	mode := int(appconfig.ReadWriteExecuteAccess)
	scriptFile, err = os.OpenFile(scriptPath, os.O_CREATE|os.O_WRONLY, os.FileMode(mode))
	if err != nil {
		log.Errorf("failed to create local script file %v, err %v", scriptPath, err)
		return
	}
	defer func() {
		cerr := scriptFile.Close()
		if err == nil {
			err = cerr
		}
	}()

	// write source commands to file
	if sourceFile != nil {
		if _, err = io.Copy(scriptFile, sourceFile); err != nil {
			log.Errorf("failed to write source scripts to file %v", scriptPath)
			return
		}
		_, err = scriptFile.WriteString("\n")
		if err != nil {
			log.Errorf("failed to write source scripts scripts to file %v", scriptPath)
			return
		}
	}

	// write source commands to file
	_, err = scriptFile.WriteString(strings.Join(runCommand, "\n") + "\n")
	if err != nil {
		log.Errorf("failed to write runcommand scripts to file %v", scriptPath)
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
	log.Debugf("Downloading file %v", downloadInput)
	return artifact.Download(log, downloadInput)
}

// GetDefaultPluginConfig returns the default values for the plugin
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
func PersistPluginInformationToCurrent(log log.T, pluginName string, config contracts.Configuration, res contracts.PluginResult) {
	//Every plugin should persist information inside the execute method.
	//At this point a plugin knows that an interim state is already stored in Current folder.
	//Plugin will continue to add data to the same file in Current folder
	messageIDSplit := strings.Split(config.MessageId, ".")
	instanceID := messageIDSplit[len(messageIDSplit)-1]

	pluginState := command_state_helper.GetPluginState(log,
		pluginName,
		config.BookKeepingFileName,
		instanceID,
		appconfig.DefaultLocationOfCurrent)

	//set plugin state's execution details
	pluginState.Configuration = config
	pluginState.Result = res
	pluginState.HasExecuted = true

	command_state_helper.PersistPluginState(log,
		pluginState,
		pluginName,
		config.BookKeepingFileName,
		instanceID,
		appconfig.DefaultLocationOfCurrent)
}

// LoadParameterAsList returns properties as a list and appropriate PluginResult if error is encountered
func LoadParametersAsList(log log.T, prop interface{}) ([]interface{}, contracts.PluginResult) {

	var properties []interface{}
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
