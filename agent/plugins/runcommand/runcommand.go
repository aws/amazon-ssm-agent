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

// Package runcommand implements the RunCommand plugin.
package runcommand

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/executers"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	command_state_helper "github.com/aws/amazon-ssm-agent/agent/message/statemanager"
	"github.com/aws/amazon-ssm-agent/agent/rebooter"
	"github.com/aws/amazon-ssm-agent/agent/s3util"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

var s3RegionUSStandard = "us-east-1"

// CommandExecuter is a function that can execute a set of commands.
type CommandExecuter func(log log.T, workingDir string, scriptPath string, orchestrationDir string, cancelFlag task.CancelFlag, executionTimeout int) (stdout io.Reader, stderr io.Reader, exitCode int, errs []error)

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

const (
	defaultExecutionTimeoutInSeconds   = 3600
	maxExecutionTimeoutInSeconds       = 28800
	minExecutionTimeoutInSeconds       = 5
	commandStoppedPreemptivelyExitCode = 137 // Fatal error (128) + signal for SIGKILL (9) = 137
)

// Plugin is the type for the RunCommand plugin.
type Plugin struct {
	// ExecuteCommand is an object that can execute commands.
	ExecuteCommand CommandExecuter

	// Uploader is an object that can upload data to s3.
	Uploader S3Uploader

	// UploadToS3Sync is true if uploading to S3 should be done synchronously, false for async.
	UploadToS3Sync bool

	// StdoutFileName is the name of the file that stores standard output.
	StdoutFileName string

	// StderrFileName is the name of the file that stores standard error.
	StderrFileName string

	// MaxStdoutLength is the maximum length of the standard output returned in the plugin result.
	// If the output is longer, it will be truncated. The full output will be uploaded to s3.
	MaxStdoutLength int

	// MaxStderrLength is the maximum length of the standard error returned in the plugin result.
	MaxStderrLength int

	// OutputTruncatedSuffix is an optional suffix that is inserted at the end of the truncated stdout/stderr.
	OutputTruncatedSuffix string
}

// NewPlugin returns a new instance of the plugin.
func NewPlugin() (*Plugin, error) {
	config, err := appconfig.GetConfig(false)
	if err != nil {
		return nil, err
	}

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
	// TODO: Move this s3 upload task outside of runcommand plugin

	awsConfig := sdkutil.GetAwsConfig()
	awsConfig.Region = &s3RegionUSStandard

	s3 := s3.New(session.New(awsConfig))

	pluginConf, ok := config.Plugins[appconfig.PluginNameAwsRunScript]
	if !ok {
		return nil, fmt.Errorf("Missing configuration for plugin %v", appconfig.PluginNameAwsRunScript)
	}

	var plugin Plugin
	err = jsonutil.Remarshal(pluginConf, &plugin)
	if err != nil {
		return nil, err
	}
	plugin.Uploader = s3util.NewManager(s3)

	exec := executers.NewShellCommandExecuter()
	exec.StdoutFileName = plugin.StdoutFileName
	exec.StderrFileName = plugin.StderrFileName
	plugin.ExecuteCommand = CommandExecuter(exec.Execute)
	return &plugin, nil
}

// Execute runs multiple sets of commands and returns their outputs.
// res.Output will contain a slice of RunCommandPluginOutput.
func (p *Plugin) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag) (res contracts.PluginResult) {
	log := context.Log()
	log.Info("RunCommand started with configuration ", config)
	res.StartDateTime = time.Now()
	defer func() { res.EndDateTime = time.Now() }()

	out := make([]contracts.PluginOutput, len(config.Properties))
	for i, prop := range config.Properties {
		// check if a reboot has been requested
		if rebooter.RebootRequested() {
			log.Info("A plugin has requested a reboot.")
			break
		}

		if cancelFlag.ShutDown() {
			out[i] = contracts.PluginOutput{Errors: []string{"Task canceled due to ShutDown"}}
			out[i].ExitCode = 1
			out[i].Status = contracts.ResultStatusFailed
			break
		} else if cancelFlag.Canceled() {
			out[i] = contracts.PluginOutput{Errors: []string{"Task canceled"}}
			out[i].ExitCode = 1
			out[i].Status = contracts.ResultStatusCancelled
			break
		}

		out[i] = p.runCommandsRawInput(log, prop, config.OrchestrationDirectory, cancelFlag, config.OutputS3BucketName, config.OutputS3KeyPrefix)

		// TODO: instance here we have to do more result processing, where individual sub properties results are merged smartly into plugin response.
		// Currently assuming we have only one work.
		res.Code = out[i].ExitCode
		res.Status = out[i].Status
		res.Output = fmt.Sprintf("%v", out[i].String())
	}

	//Every plugin should persist information inside the execute method.
	//At this point a plugin knows that an interim state is already stored in Current folder.
	//Plugin will continue to add data to the same file in Current folder

	messageIDSplit := strings.Split(config.MessageId, ".")
	instanceID := messageIDSplit[len(messageIDSplit)-1]

	pluginState := command_state_helper.GetPluginState(log,
		appconfig.PluginNameAwsRunScript,
		config.BookKeepingFileName,
		instanceID,
		appconfig.DefaultLocationOfCurrent)

	//set plugin state's execution details
	pluginState.Configuration = config
	pluginState.Result = res
	pluginState.HasExecuted = true

	command_state_helper.PersistPluginState(log,
		pluginState,
		appconfig.PluginNameAwsRunScript,
		config.BookKeepingFileName,
		instanceID,
		appconfig.DefaultLocationOfCurrent)

	return res
}

// RunCommandPluginInput represents one set of commands executed by the RunCommand plugin.
type RunCommandPluginInput struct {
	contracts.PluginInput
	RunCommand       []string
	ID               string
	WorkingDirectory string
	TimeoutSeconds   string
	Source           string
	SourceHash       string
	SourceHashType   string
}

// runCommandsRawInput executes one set of commands and returns their output.
// The input is in the default json unmarshal format (e.g. map[string]interface{}).
func (p *Plugin) runCommandsRawInput(log log.T, rawPluginInput interface{}, orchestrationDirectory string, cancelFlag task.CancelFlag, outputS3BucketName string, outputS3KeyPrefix string) (out contracts.PluginOutput) {
	var pluginInput RunCommandPluginInput
	err := jsonutil.Remarshal(rawPluginInput, &pluginInput)
	if err != nil {
		errorString := fmt.Sprintf("Invalid format in plugin properties %v;\nerror %v", rawPluginInput, err)
		out.Errors = append(out.Errors, errorString)
		out.Status = contracts.ResultStatusFailed
		log.Error(errorString)
		return
	}
	return p.runCommands(log, pluginInput, orchestrationDirectory, cancelFlag, outputS3BucketName, outputS3KeyPrefix)
}

// runCommands executes one set of commands and returns their output.
func (p *Plugin) runCommands(log log.T, pluginInput RunCommandPluginInput, orchestrationDirectory string, cancelFlag task.CancelFlag, outputS3BucketName string, outputS3KeyPrefix string) (out contracts.PluginOutput) {
	var err error

	// if no orchestration directory specified, create temp directory
	var useTempDirectory = (orchestrationDirectory == "")
	var tempDir string
	if useTempDirectory {
		if tempDir, err = ioutil.TempDir("", "Ec2RunCommand"); err != nil {
			out.Errors = append(out.Errors, err.Error())
			log.Error(err)
			return
		}
		orchestrationDirectory = tempDir
	}

	orchestrationDir := fileutil.RemoveInvalidChars(filepath.Join(orchestrationDirectory, pluginInput.ID))
	log.Debugf("Running commands %v in workingDirectory %v; orchestrationDir %v ", pluginInput.RunCommand, pluginInput.WorkingDirectory, orchestrationDir)

	// create orchestration dir if needed
	err = os.MkdirAll(orchestrationDir, appconfig.ReadWriteExecuteAccess)
	if err != nil {
		log.Debug("failed to create orchestrationDir directory", orchestrationDir)
		out.Errors = append(out.Errors, err.Error())
		return
	}

	scriptPath := filepath.Join(orchestrationDir, executers.RunCommandScriptName)
	log.Debugf("Writing commands %v to file %v", pluginInput, scriptPath)

	err = pluginInput.createScriptFile(log, scriptPath)
	if err != nil {
		out.Errors = append(out.Errors, err.Error())
		log.Errorf("failed to create script file. %v", err)
		return
	}

	executionTimeout, _ := strconv.Atoi(pluginInput.TimeoutSeconds)
	if executionTimeout < minExecutionTimeoutInSeconds || executionTimeout > maxExecutionTimeoutInSeconds {
		executionTimeout = defaultExecutionTimeoutInSeconds
	}

	stdout, stderr, exitCode, errs := p.ExecuteCommand(log, pluginInput.WorkingDirectory, scriptPath, orchestrationDir, cancelFlag, executionTimeout)

	out.ExitCode = exitCode
	if out.ExitCode == 0 {
		out.Status = contracts.ResultStatusSuccess
		// disabling special handling for exitcodes
		//	} else if out.ExitCode == appconfig.RebootExitCode {
		//		out.Status = contracts.ResultStatusSuccessAndReboot
		//		out.Stdout += "\nReboot requested"
	} else if out.ExitCode == commandStoppedPreemptivelyExitCode {
		if cancelFlag.ShutDown() {
			out.Status = contracts.ResultStatusFailed
		} else if cancelFlag.Canceled() {
			out.Status = contracts.ResultStatusCancelled
		} else {
			out.Status = contracts.ResultStatusTimedOut
		}
	} else {
		out.Status = contracts.ResultStatusFailed
	}

	if len(errs) > 0 {
		for _, err := range errs {
			out.Errors = append(out.Errors, err.Error())
			if out.Status != contracts.ResultStatusCancelled &&
				out.Status != contracts.ResultStatusTimedOut &&
				out.Status != contracts.ResultStatusSuccessAndReboot {
				log.Error("failed to run commands: ", err)
				out.Status = contracts.ResultStatusFailed
			}
		}
	}

	// read (a prefix of) the standard output/error
	out.Stdout, err = readPrefix(stdout, p.MaxStdoutLength, p.OutputTruncatedSuffix)
	if err != nil {
		out.Errors = append(out.Errors, err.Error())
		log.Error(err)
	}
	out.Stderr, err = readPrefix(stderr, p.MaxStderrLength, p.OutputTruncatedSuffix)
	if err != nil {
		out.Errors = append(out.Errors, err.Error())
		log.Error(err)
	}

	// upload outputs (if any) to s3
	if outputS3BucketName != "" {
		uploadOutputsToS3 := func() {
			uploadToS3 := true
			var testUploadError error

			//set region to us-east-1
			p.Uploader.SetS3ClientRegion(s3RegionUSStandard)

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
					s3RegionUSStandard,
					p.Uploader.GetS3ClientRegion())
			}

			if uploadToS3 {
				log.Infof("uploading logs to S3 with client configured to use region - %v", p.Uploader.GetS3ClientRegion())

				if useTempDirectory {
					// delete temp directory once we're done
					defer executers.DeleteDirectory(log, tempDir)
				}

				if out.Stdout != "" {
					localPath := filepath.Join(orchestrationDir, p.StdoutFileName)
					s3Key := path.Join(outputS3KeyPrefix, pluginInput.ID, p.StdoutFileName)
					log.Debugf("Uploading %v to s3://%v/%v", localPath, outputS3BucketName, s3Key)
					err := p.Uploader.S3Upload(outputS3BucketName, s3Key, localPath)
					if err != nil {

						log.Errorf("failed uploading %v to s3://%v/%v err:%v", localPath, outputS3BucketName, s3Key, err)
						if p.UploadToS3Sync {
							// if we are in synchronous mode, we can also return the error
							out.Errors = append(out.Errors, err.Error())
						}
					}
				}

				if out.Stderr != "" {
					localPath := filepath.Join(orchestrationDir, p.StderrFileName)
					s3Key := path.Join(outputS3KeyPrefix, pluginInput.ID, p.StderrFileName)
					log.Debugf("Uploading %v to s3://%v/%v", localPath, outputS3BucketName, s3Key)
					err := p.Uploader.S3Upload(outputS3BucketName, s3Key, localPath)
					if err != nil {
						log.Errorf("failed uploading %v to s3://%v/%v err:%v", localPath, outputS3BucketName, s3Key, err)
						if p.UploadToS3Sync {
							// if we are in synchronous mode, we can also return the error
							out.Errors = append(out.Errors, err.Error())
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

	responseContent, _ := jsonutil.Marshal(out)
	log.Debug("Returning response:\n", jsonutil.Indent(responseContent))
	return
}

// CreateScriptFile creates a script containing the given commands.
func (p *RunCommandPluginInput) createScriptFile(log log.T, scriptPath string) (err error) {
	log.Debugf("Writing commands to file %v", scriptPath)
	var sourceFile *os.File
	var scriptFile *os.File

	// download source and verify its integrity
	if p.Source != "" {
		downloadInput := artifact.DownloadInput{
			SourceURL:       p.Source,
			SourceHashValue: p.SourceHash,
			SourceHashType:  p.SourceHashType,
		}
		var downloadOutput artifact.DownloadOutput
		log.Debugf("Downloading file %v", downloadInput)
		downloadOutput, err = artifact.Download(log, downloadInput)
		if err != nil || downloadOutput.IsHashMatched == false || downloadOutput.LocalFilePath == "" {
			log.Errorf("failed to download file reliably. , %v", err)
			return
		}
		sourceFile, err = os.Open(downloadOutput.LocalFilePath)
		if err != nil {
			log.Errorf("failed to open local downloaded source file. %v", err)
			return
		}
		defer sourceFile.Close()
	}

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
	_, err = scriptFile.WriteString(strings.Join(p.RunCommand, "\n") + "\n")
	if err != nil {
		log.Errorf("failed to write runcommand scripts to file %v", scriptPath)
		return
	}

	return
}

// readPrefix returns the beginning data from a given Reader, truncated to the given limit.
func readPrefix(input io.Reader, maxLength int, truncatedSuffix string) (out string, err error) {
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

// Name returns the plugin name
func Name() string {
	return appconfig.PluginNameAwsRunScript
}

// The init method registers an instance of the plugin to the plugin registry.
// TODO:  Move away from Init during refactoring
func init() {

}
