package dockercontainer

import (
	"errors"
	"fmt"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	iohandlermocks "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler/mock"
	multiwritermock "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler/multiwriter/mock"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/mocks/context"
	"github.com/aws/amazon-ssm-agent/agent/mocks/executers"
	taskmocks "github.com/aws/amazon-ssm-agent/agent/mocks/task"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/twinj/uuid"
	"testing"
)

type CommandTester func(p *Plugin, mockCancelFlag *taskmocks.MockCancelFlag, mockExecuter *executers.MockCommandExecuter, mockIOHandler *iohandlermocks.MockIOHandler)

type TestCase struct {
	Input          DockerContainerPluginInput
	Output         iohandler.DefaultIOHandler
	ExecuterErrors error
	MessageID      string
}

func makeConfig() *contracts.IOConfiguration {
	return &contracts.IOConfiguration{
		OrchestrationDirectory: "OrchestrationDirectory",
		OutputS3BucketName:     "test",
		OutputS3KeyPrefix:      "test",
		CloudWatchConfig: contracts.CloudWatchConfiguration{
			LogGroupName:              "test",
			LogStreamPrefix:           "test",
			LogGroupEncryptionEnabled: true,
		},
	}
}

func getDefaultInput() *DockerContainerPluginInput {
	return &DockerContainerPluginInput{
		Action:           "Create",
		ID:               uuid.NewV4().String(),
		WorkingDirectory: "",
		TimeoutSeconds:   "",
		Container:        "testContainer",
		Cmd:              "test",
		Image:            "testImage",
		Memory:           "100m",
		CpuShares:        "test",
		Volume:           []string{"test.file", "testdir/test.file"},
		Env:              "test",
		User:             "testalias",
		Publish:          "8080:80",
	}
}

const (
	orchDir = "OrchestrationDirectory"
)

var TestCases = []TestCase{
	generateTestCaseOk("0"),
	generateTestCaseOk("1"),
	generateTestCaseFail("2"),
	generateTestCaseFail("3"),
}

var Actions = []string{CREATE, RUN, START, STOP, RM, EXEC, INSPECT, LOGS, PS, STATS, PULL, IMAGES, RMI}

func generateTestCaseOk(id string) TestCase {
	testCase := TestCase{
		Input:  *getDefaultInput(),
		Output: iohandler.DefaultIOHandler{},
	}

	testCase.Output.StdoutWriter = new(multiwritermock.MockDocumentIOMultiWriter)
	testCase.Output.StderrWriter = new(multiwritermock.MockDocumentIOMultiWriter)
	testCase.Output.SetStdout("standard output of test case " + id)
	testCase.Output.SetStderr("standard error of test case " + id)
	testCase.Output.ExitCode = 0
	testCase.Output.Status = "Success"

	return testCase
}

func generateTestCaseFail(id string) TestCase {
	t := generateTestCaseOk(id)
	t.ExecuterErrors = fmt.Errorf("Error happened for cmd %v", id)
	t.Output.SetStderr(t.ExecuterErrors.Error())
	t.Output.ExitCode = 1
	t.Output.Status = "Failed"
	return t
}

// TestRunCommands tests the runCommands and runCommandsRawInput methods, which run one set of commands.
func TestRunCommands(t *testing.T) {
	for _, testCase := range TestCases {
		testRunCommands(t, testCase, true)
		testRunCommands(t, testCase, false)
	}
}

// testRunCommands tests the runCommands or the runCommandsRawInput method for one testcase.
func testRunCommands(t *testing.T, testCase TestCase, rawInput bool) {
	runCommandTester := func(p *Plugin, mockCancelFlag *taskmocks.MockCancelFlag, mockExecuter *executers.MockCommandExecuter, mockIOHandler *iohandlermocks.MockIOHandler) {
		// set expectations
		setExecuterExpectations(mockExecuter, testCase, mockCancelFlag, p)
		setIOHandlerExpectations(mockIOHandler, testCase)

		// call method under test for each action type
		for _, action := range Actions {
			testCase.Input.Action = action
			if rawInput {
				// prepare plugin input
				var rawPluginInput interface{}
				err := jsonutil.Remarshal(testCase.Input, &rawPluginInput)
				assert.Nil(t, err)
				p.runCommandsRawInput("", rawPluginInput, orchDir, mockCancelFlag, mockIOHandler)
			} else {
				p.runCommands("", testCase.Input, orchDir, mockCancelFlag, mockIOHandler)
			}
		}
	}

	testExecution(t, runCommandTester)
}

// testExecution sets up boiler plate mocked objects then delegates to a more
// specific tester, then asserts general expectations on the mocked objects.
// It is the responsibility of the inner tester to set up expectations
// and assert specific result conditions.
func testExecution(t *testing.T, commandtester CommandTester) {
	// create mocked objects
	mockCancelFlag := new(taskmocks.MockCancelFlag)
	mockExecuter := new(executers.MockCommandExecuter)
	mockIOHandler := new(iohandlermocks.MockIOHandler)

	// create plugin
	p := &Plugin{
		context: context.NewMockDefault(),
	}
	p.CommandExecuter = mockExecuter

	// run inner command tester
	commandtester(p, mockCancelFlag, mockExecuter, mockIOHandler)

	// assert that the expectations were met
	mockExecuter.AssertExpectations(t)
	mockCancelFlag.AssertExpectations(t)
	mockIOHandler.AssertExpectations(t)
}

func setExecuterExpectations(mockExecuter *executers.MockCommandExecuter, t TestCase, cancelFlag task.CancelFlag, p *Plugin) {
	mockExecuter.On("NewExecute", mock.Anything, t.Input.WorkingDirectory, t.Output.StdoutWriter, t.Output.StderrWriter, cancelFlag, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		t.Output.ExitCode, t.ExecuterErrors)
}

func setIOHandlerExpectations(mockIOHandler *iohandlermocks.MockIOHandler, t TestCase) {
	mockIOHandler.On("GetStdoutWriter").Return(t.Output.StdoutWriter)
	mockIOHandler.On("GetStderrWriter").Return(t.Output.StderrWriter)
	mockIOHandler.On("SetExitCode", t.Output.ExitCode).Return()
	mockIOHandler.On("SetStatus", t.Output.Status).Return()
	mockIOHandler.On("AppendInfo", mock.Anything).Return()
	if t.ExecuterErrors != nil {
		mockIOHandler.On("GetStatus").Return(t.Output.Status)
		mockIOHandler.On("MarkAsFailed", fmt.Errorf("failed to run commands: %v", t.ExecuterErrors)).Return()
		mockIOHandler.On("SetStatus", contracts.ResultStatusFailed).Return()
	}
}

func TestRunCommandsInvalidParameters(t *testing.T) {
	output := iohandler.NewDefaultIOHandler(context.NewMockDefault(), *makeConfig())
	plugin, _ := NewPlugin(context.NewMockDefault())
	flag := taskmocks.NewMockDefault()

	dockerContainerPlugin := *getDefaultInput()
	dockerContainerPlugin.Action = "Invalid"
	plugin.runCommands("id", dockerContainerPlugin, orchDir, flag, output)
	assert.Contains(t, output.GetStderr(), "Docker Action is set to unsupported value: Invalid")

	dockerContainerPlugin.Action = "Create"
	dockerContainerPlugin.Image = ""
	plugin.runCommands("id", dockerContainerPlugin, orchDir, flag, output)
	assert.Contains(t, output.GetStderr(), "Action Create requires parameter image")

	dockerContainerPlugin.Action = "Start"
	dockerContainerPlugin.Container = ""
	plugin.runCommands("id", dockerContainerPlugin, orchDir, flag, output)
	assert.Contains(t, output.GetStderr(), "Action Start requires parameter container")

	dockerContainerPlugin.Action = "Rm"
	dockerContainerPlugin.Container = ""
	plugin.runCommands("id", dockerContainerPlugin, orchDir, flag, output)
	assert.Contains(t, output.GetStderr(), "Action Rm requires parameter container")

	dockerContainerPlugin.Action = "Stop"
	dockerContainerPlugin.Container = ""
	plugin.runCommands("id", dockerContainerPlugin, orchDir, flag, output)
	assert.Contains(t, output.GetStderr(), "Action Stop requires parameter container")

	dockerContainerPlugin.Action = "Exec"
	dockerContainerPlugin.Container = ""
	plugin.runCommands("id", dockerContainerPlugin, orchDir, flag, output)
	assert.Contains(t, output.GetStderr(), "Action Exec requires parameter container")
	dockerContainerPlugin.Container = "testContainer"
	dockerContainerPlugin.Cmd = ""
	plugin.runCommands("id", dockerContainerPlugin, orchDir, flag, output)
	assert.Contains(t, output.GetStderr(), "Action Exec requires parameter cmd")

	dockerContainerPlugin.Action = "Inspect"
	dockerContainerPlugin.Container = ""
	dockerContainerPlugin.Image = ""
	plugin.runCommands("id", dockerContainerPlugin, orchDir, flag, output)
	assert.Contains(t, output.GetStderr(), "Action Inspect requires parameter container or image")

	dockerContainerPlugin.Action = "Logs"
	dockerContainerPlugin.Container = ""
	plugin.runCommands("id", dockerContainerPlugin, orchDir, flag, output)
	assert.Contains(t, output.GetStderr(), "Action Logs requires parameter container")

	dockerContainerPlugin.Action = "Pull"
	dockerContainerPlugin.Image = ""
	plugin.runCommands("id", dockerContainerPlugin, orchDir, flag, output)
	assert.Contains(t, output.GetStderr(), "Action Pull requires parameter image")

	dockerContainerPlugin.Action = "Rmi"
	dockerContainerPlugin.Image = ""
	plugin.runCommands("id", dockerContainerPlugin, orchDir, flag, output)
	assert.Contains(t, output.GetStderr(), "Action Rmi requires parameter image")
}

func TestValidateInputsNoErrors(t *testing.T) {
	pluginInput := getDefaultInput()

	err := validateInputs(*pluginInput)
	assert.Equal(t, nil, err)
}

func TestValidateInputsMemoryValues(t *testing.T) {
	pluginInput := getDefaultInput()
	errorMessage := errors.New("Invalid Memory value")

	pluginInput.Memory = "100"
	err := validateInputs(*pluginInput)
	assert.Equal(t, nil, err)

	pluginInput.Memory = "100g"
	err2 := validateInputs(*pluginInput)
	assert.Equal(t, nil, err2)

	pluginInput.Memory = "100b"
	err3 := validateInputs(*pluginInput)
	assert.Equal(t, nil, err3)

	pluginInput.Memory = "100l"
	err4 := validateInputs(*pluginInput)
	assert.Equal(t, errorMessage, err4)
}

func TestValidateInputsEnvValues(t *testing.T) {
	pluginInput := getDefaultInput()
	errorMessage := errors.New("Invalid environment variable value")

	pluginInput.Env = ",test"
	err := validateInputs(*pluginInput)
	assert.Equal(t, errorMessage, err)

	pluginInput.Env = ";test"
	err2 := validateInputs(*pluginInput)
	assert.Equal(t, errorMessage, err2)

	pluginInput.Env = "&test"
	err3 := validateInputs(*pluginInput)
	assert.Equal(t, errorMessage, err3)

	pluginInput.Env = "-test"
	err4 := validateInputs(*pluginInput)
	assert.Equal(t, nil, err4)
}
