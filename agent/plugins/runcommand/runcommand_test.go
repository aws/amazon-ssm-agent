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

package runcommand

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/executers"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/s3util"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/twinj/uuid"
)

type TestCase struct {
	Input               RunCommandPluginInput
	Output              contracts.PluginOutput
	ExecuterErrors      []error
	S3UploadStdoutError error
	S3UploadStderrError error
	MessageID           string
}

type CommandTester func(p *Plugin, mockCancelFlag *task.MockCancelFlag, mockExecuter *executers.MockCommandExecuter, mockS3Uploader *s3util.MockS3Uploader)

const (
	orchestrationDirectory = "OrchesDir"
	s3BucketName           = "bucket"
	s3KeyPrefix            = "key"
	testInstanceID         = "i-12345678"
	bucketRegionErrorMsg   = "AuthorizationHeaderMalformed: The authorization header is malformed; the region 'us-east-1' is wrong; expecting 'us-west-2' status code: 400, request id: []"
)

var TestCases = []TestCase{
	generateTestCaseOk("0"),
	generateTestCaseOk("1"),
	generateTestCaseFail("2"),
	generateTestCaseFail("3"),
}

var logger = log.NewMockLog()

func generateTestCaseOk(id string) TestCase {
	return TestCase{
		Input: RunCommandPluginInput{
			RunCommand:       []string{"echo " + id},
			ID:               id + ".aws:runScript",
			WorkingDirectory: "Dir" + id,
			TimeoutSeconds:   "1",
		},
		Output: contracts.PluginOutput{
			Stdout:   "standard output of test case " + id,
			Stderr:   "standard error of test case " + id,
			ExitCode: 0,
			Status:   "Success",
		},
	}
}

func generateTestCaseFail(id string) TestCase {

	t := generateTestCaseOk(id)
	t.ExecuterErrors = []error{fmt.Errorf("Error happened for cmd %v", id)}
	t.S3UploadStdoutError = fmt.Errorf("Error uploading stdout to s3 for cmd %v", id)
	t.S3UploadStderrError = fmt.Errorf("Error uploading stderr to s3 for cmd %v", id)

	for _, err := range t.ExecuterErrors {
		t.Output.Errors = append(t.Output.Errors, err.Error())
	}
	t.Output.Errors = append(t.Output.Errors, t.S3UploadStdoutError.Error())
	t.Output.Errors = append(t.Output.Errors, t.S3UploadStderrError.Error())
	t.Output.Status = "Failed"
	return t
}

//func generateTestCaseReboot(id string) TestCase {
//	t := generateTestCaseOk(id)
//	t.Output.ExitCode = appconfig.RebootExitCode
//	t.Output.Status = "PassedAndReboot"
//	return t
//}

// TestRunCommands tests the runCommands and runCommandsRawInput methods, which run one set of commands.
func TestRunCommands(t *testing.T) {
	for _, testCase := range TestCases {
		testRunCommands(t, testCase, true)
		testRunCommands(t, testCase, false)
	}
}

//var RebootTestCases = []TestCase{
//	generateTestCaseReboot("1"),
//}

// TestRunCommands tests the runCommands and runCommandsRawInput methods, which run one set of commands.
func TestRunCommandsReboot(t *testing.T) {
	for _, testCase := range TestCases {
		testRunCommands(t, testCase, true)
		testRunCommands(t, testCase, false)
	}
}

// testRunCommands tests the runCommands or the runCommandsRawInput method for one testcase.
func testRunCommands(t *testing.T, testCase TestCase, rawInput bool) {
	logger.On("Error", mock.Anything).Return(nil)
	logger.Infof("test run commands %v", testCase)
	runCommandTester := func(p *Plugin, mockCancelFlag *task.MockCancelFlag, mockExecuter *executers.MockCommandExecuter, mockS3Uploader *s3util.MockS3Uploader) {
		// set expectations
		setExecuterExpectations(mockExecuter, testCase, mockCancelFlag)
		setS3UploaderExpectations(mockS3Uploader, testCase, p)
		setS3TestUploadExpectations(logger, mockS3Uploader, testCase, p, false)

		// call method under test
		var res contracts.PluginOutput
		if rawInput {
			// prepare plugin input
			var rawPluginInput interface{}
			err := jsonutil.Remarshal(testCase.Input, &rawPluginInput)
			assert.Nil(t, err)

			res = p.runCommandsRawInput(logger, rawPluginInput, orchestrationDirectory, mockCancelFlag, s3BucketName, s3KeyPrefix)
		} else {
			res = p.runCommands(logger, testCase.Input, orchestrationDirectory, mockCancelFlag, s3BucketName, s3KeyPrefix)
		}

		// assert output is correct (mocked object expectations are tested automatically by testExecution)
		assert.Equal(t, testCase.Output, res)
	}

	testExecution(t, runCommandTester)
}

// TestBucketsInDifferentRegions tests runCommands when S3Buckets are present in IAD and PDX region.
func TestBucketsInDifferentRegions(t *testing.T) {
	for _, testCase := range TestCases {
		testBucketsInDifferentRegions(t, testCase, true)
		testBucketsInDifferentRegions(t, testCase, false)
	}
}

// testBucketsInDifferentRegions tests the runCommands with S3 buckets present in IAD and PDX region.
func testBucketsInDifferentRegions(t *testing.T, testCase TestCase, testingBucketsInDifferentRegions bool) {
	logger.On("Error", mock.Anything).Return(nil)
	logger.Infof("test run commands %v", testCase)
	runCommandTester := func(p *Plugin, mockCancelFlag *task.MockCancelFlag, mockExecuter *executers.MockCommandExecuter, mockS3Uploader *s3util.MockS3Uploader) {
		// set expectations
		setExecuterExpectations(mockExecuter, testCase, mockCancelFlag)
		setS3UploaderExpectations(mockS3Uploader, testCase, p)
		setS3TestUploadExpectations(logger, mockS3Uploader, testCase, p, testingBucketsInDifferentRegions)

		// call method under test
		var res contracts.PluginOutput
		res = p.runCommands(logger, testCase.Input, orchestrationDirectory, mockCancelFlag, s3BucketName, s3KeyPrefix)

		// assert output is correct (mocked object expectations are tested automatically by testExecution)
		assert.Equal(t, testCase.Output, res)
	}

	testExecution(t, runCommandTester)
}

// TestExecute tests the Execute method, which runs multiple sets of commands.
func TestExecute(t *testing.T) {
	// test each plugin input as a separate execution
	for i := range TestCases {
		testExecute(t, TestCases[i:i+1])
	}

	// test all plugin inputs as one execution
	testExecute(t, TestCases)
}

// testExecute tests the run command plugin's Execute method.
func testExecute(t *testing.T, testCases []TestCase) {
	executeTester := func(p *Plugin, mockCancelFlag *task.MockCancelFlag, mockExecuter *executers.MockCommandExecuter, mockS3Uploader *s3util.MockS3Uploader) {
		// setup expectations and correct outputs
		var pluginProperties []interface{}
		var correctOutputs string
		mockContext := context.NewMockDefault()
		for _, testCase := range testCases {
			// set expectations
			mockCancelFlag.On("Canceled").Return(false)
			mockCancelFlag.On("ShutDown").Return(false)
			setExecuterExpectations(mockExecuter, testCase, mockCancelFlag)
			setS3UploaderExpectations(mockS3Uploader, testCase, p)
			setS3TestUploadExpectations(mockContext.Log(), mockS3Uploader, testCase, p, false)

			// prepare plugin input
			var rawPluginInput interface{}
			err := jsonutil.Remarshal(testCase.Input, &rawPluginInput)
			assert.Nil(t, err)

			pluginProperties = append(pluginProperties, rawPluginInput)
			correctOutputs = testCase.Output.String()
		}

		//Create messageId which is in the format of aws.ssm.<commandID>.<InstanceID>
		commandID := uuid.NewV4().String()

		// call plugin
		res := p.Execute(
			mockContext,
			contracts.Configuration{
				Properties:             pluginProperties,
				OutputS3BucketName:     s3BucketName,
				OutputS3KeyPrefix:      s3KeyPrefix,
				OrchestrationDirectory: orchestrationDirectory,
				BookKeepingFileName:    commandID,
			}, mockCancelFlag)

		// assert output is correct (mocked object expectations are tested automatically by testExecution)
		assert.NotNil(t, res.StartDateTime)
		assert.NotNil(t, res.EndDateTime)
		assert.Equal(t, correctOutputs, res.Output)

		// assert that the flag is checked after every set of commands
		mockCancelFlag.AssertNumberOfCalls(t, "Canceled", len(testCases))
	}

	testExecution(t, executeTester)
}

// testExecution sets up boiler plate mocked objects then delegates to a more
// specific tester, then asserts general expectations on the mocked objects.
// It is the responsibility of the inner tester to set up expectations
// and assert specific result conditions.
func testExecution(t *testing.T, commandtester CommandTester) {
	// create mocked objects
	mockCancelFlag := new(task.MockCancelFlag)
	mockExecuter := new(executers.MockCommandExecuter)
	mockS3Uploader := new(s3util.MockS3Uploader)

	// create plugin
	p := &Plugin{
		ExecuteCommand:        CommandExecuter(mockExecuter.Execute),
		Uploader:              mockS3Uploader,
		StdoutFileName:        "stdout",
		StderrFileName:        "stderr",
		MaxStdoutLength:       1000,
		MaxStderrLength:       1000,
		OutputTruncatedSuffix: "-more-",
		UploadToS3Sync:        true,
	}

	// run inner command tester
	commandtester(p, mockCancelFlag, mockExecuter, mockS3Uploader)

	// assert that the expectations were met
	mockExecuter.AssertExpectations(t)
	mockCancelFlag.AssertExpectations(t)
	mockS3Uploader.AssertExpectations(t)
}

func setExecuterExpectations(mockExecuter *executers.MockCommandExecuter, t TestCase, cancelFlag task.CancelFlag) {
	orchestrationDir := fileutil.RemoveInvalidChars(filepath.Join(orchestrationDirectory, t.Input.ID))
	mockExecuter.On("Execute", mock.Anything, t.Input.WorkingDirectory, filepath.Join(orchestrationDir, executers.RunCommandScriptName), orchestrationDir, cancelFlag, mock.Anything).Return(
		readerFromString(t.Output.Stdout), readerFromString(t.Output.Stderr), t.Output.ExitCode, t.ExecuterErrors)
}

func setS3UploaderExpectations(mockS3Uploader *s3util.MockS3Uploader, t TestCase, p *Plugin) {
	orchestrationDir := fileutil.RemoveInvalidChars(filepath.Join(orchestrationDirectory, t.Input.ID))

	mockS3Uploader.On("S3Upload", s3BucketName,
		path.Join(s3KeyPrefix, t.Input.ID, p.StdoutFileName),
		filepath.Join(orchestrationDir, p.StdoutFileName)).Return(t.S3UploadStdoutError)

	mockS3Uploader.On("S3Upload", s3BucketName,
		path.Join(s3KeyPrefix, t.Input.ID, p.StderrFileName),
		filepath.Join(orchestrationDir, p.StderrFileName)).Return(t.S3UploadStderrError)
}

func setS3TestUploadExpectations(log log.T, mockS3Uploader *s3util.MockS3Uploader, t TestCase, p *Plugin, testingBucketsInDifferentRegions bool) {
	if testingBucketsInDifferentRegions {
		mockS3Uploader.On("UploadS3TestFile", log, s3BucketName, s3KeyPrefix).Return(nil)
		mockS3Uploader.On("GetS3ClientRegion").Return("us-east-1")
	} else {
		mockS3Uploader.On("UploadS3TestFile", log, s3BucketName, s3KeyPrefix).Return(errors.New(bucketRegionErrorMsg))
		mockS3Uploader.On("GetS3ClientRegion").Return("us-west-2")
		mockS3Uploader.On("IsS3ErrorRelatedToAccessDenied", bucketRegionErrorMsg).Return(false)
		mockS3Uploader.On("IsS3ErrorRelatedToWrongBucketRegion", bucketRegionErrorMsg).Return(true)
		mockS3Uploader.On("GetS3BucketRegionFromErrorMsg", log, bucketRegionErrorMsg).Return("us-west-2")
	}
}

func setCancelFlagExpectations(mockCancelFlag *task.MockCancelFlag) {
	mockCancelFlag.On("Canceled").Return(false)
	mockCancelFlag.On("ShutDown").Return(false)
	mockCancelFlag.On("Wait").Return(false).After(100 * time.Millisecond)
}

func readerFromString(s string) io.Reader {
	return bytes.NewReader([]byte(s))
}

// TestReadPrefix tests that readPrefix works correctly.
func TestReadPrefix(t *testing.T) {
	inputs := []string{"a string to truncate", ""}
	suffix := "-z-"

	for _, input := range inputs {
		testReadPrefix(t, input, suffix, true)
		testReadPrefix(t, input, suffix, false)
	}
}

func testReadPrefix(t *testing.T, input string, suffix string, truncate bool) {
	// setup inputs
	var maxLength int
	if truncate {
		maxLength = len(input) / 2
	} else {
		maxLength = len(input) + 1
	}
	reader := bytes.NewReader([]byte(input))

	// call method under test
	output, err := readPrefix(reader, maxLength, suffix)

	// test results
	assert.Nil(t, err)
	if truncate {
		testTruncatedString(t, input, output, maxLength, suffix)
	} else {
		assert.Equal(t, input, output)
	}
}

// testTruncatedString tests that truncated is obtained from original, truncated has the expected length and ends with the expected suffix.
func testTruncatedString(t *testing.T, original string, truncated string, truncatedLength int, truncatedSuffix string) {
	assert.Equal(t, truncatedLength, len(truncated))
	if truncatedLength >= len(truncatedSuffix) {
		// enough room to fit the suffix
		assert.True(t, strings.HasSuffix(truncated, truncatedSuffix))
		assert.True(t, strings.HasPrefix(original, truncated[:truncatedLength-len(truncatedSuffix)]))
	} else {
		// suffix doesn't fir, expect a prefix of the suffix
		assert.Equal(t, truncated, truncatedSuffix[:truncatedLength])
	}
}

func TestOutputTruncation(t *testing.T) {
	out := contracts.PluginOutput{
		Stdout:   "standard output of test case",
		Stderr:   "standard error of test case",
		ExitCode: 0,
		Status:   "Success",
	}
	response := contracts.TruncateOutput(out.Stdout, out.Stderr, 200)
	fmt.Printf("response=\n%v\n", response)
	assert.Equal(t, out.String(), response)

}
