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

package dockercontainer

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/executers"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

const (
	//Action values
	CREATE  = "Create"
	START   = "Start"
	RUN     = "Run"
	STOP    = "Stop"
	RM      = "Rm"
	EXEC    = "Exec"
	INSPECT = "Inspect"
	LOGS    = "Logs"
	PS      = "Ps"
	STATS   = "Stats"
	PULL    = "Pull"
	IMAGES  = "Images"
	RMI     = "Rmi"
)
const (
	ACTION_REQUIRES_PARAMETER = "Action %s requires parameter %s"
)

var dockerExecCommand = "docker.exe"
var duration_Seconds time.Duration = 30 * time.Second

// Plugin is the type for the plugin.
type Plugin struct {
	// ExecuteCommand is an object that can execute commands.
	CommandExecuter executers.T
}

// DockerContainerPluginInput represents one set of commands executed by the RunCommand plugin.
type DockerContainerPluginInput struct {
	contracts.PluginInput
	Action           string
	ID               string
	WorkingDirectory string
	TimeoutSeconds   interface{}
	Container        string
	Cmd              string
	Image            string
	Memory           string
	CpuShares        string
	Volume           []string
	Env              string
	User             string
	Publish          string
}

// NewPlugin returns a new instance of the plugin.
func NewPlugin() (*Plugin, error) {
	var plugin Plugin
	plugin.CommandExecuter = executers.ShellCommandExecuter{}

	return &plugin, nil
}

// Name returns the name of the plugin
func Name() string {
	return appconfig.PluginNameDockerContainer
}

func (p *Plugin) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag, output iohandler.IOHandler) {
	log := context.Log()
	log.Infof("%v started with configuration %v", Name(), config)
	if cancelFlag.ShutDown() {
		output.MarkAsShutdown()
	} else if cancelFlag.Canceled() {
		output.MarkAsCancelled()
	} else {
		p.runCommandsRawInput(log, config.PluginID, config.Properties, config.OrchestrationDirectory, cancelFlag, output)
	}
	return
}

// runCommandsRawInput executes one set of commands and returns their output.
// The input is in the default json unmarshal format (e.g. map[string]interface{}).
func (p *Plugin) runCommandsRawInput(log log.T, pluginID string, rawPluginInput interface{}, orchestrationDirectory string, cancelFlag task.CancelFlag, output iohandler.IOHandler) {
	var pluginInput DockerContainerPluginInput
	err := jsonutil.Remarshal(rawPluginInput, &pluginInput)
	log.Debugf("Plugin input %v", pluginInput)
	if err != nil {
		errorString := fmt.Errorf("Invalid format in plugin properties %v;\nerror %v", rawPluginInput, err)
		output.MarkAsFailed(errorString)
		return
	}

	p.runCommands(log, pluginID, pluginInput, orchestrationDirectory, cancelFlag, output)
}

// runCommands executes one set of commands and returns their output.
func (p *Plugin) runCommands(log log.T, pluginID string, pluginInput DockerContainerPluginInput, orchestrationDirectory string, cancelFlag task.CancelFlag, output iohandler.IOHandler) {
	var err error

	// TODO:MF: This subdirectory is only needed because we could be running multiple sets of properties for the same plugin - otherwise the orchestration directory would already be unique
	orchestrationDir := fileutil.BuildPath(orchestrationDirectory, pluginInput.ID)
	log.Debugf("OrchestrationDir %v ", orchestrationDir)

	if err = validateInputs(pluginInput); err != nil {
		output.MarkAsFailed(fmt.Errorf("Validation error, %v", err))
		return
	}

	// create orchestration dir if needed
	if err = fileutil.MakeDirs(orchestrationDir); err != nil {
		log.Debug("failed to create orchestrationDir directory", orchestrationDir, err)
		output.MarkAsFailed(err)
		return
	}
	var commandName string = "docker"
	var commandArguments []string
	switch pluginInput.Action {
	case CREATE, RUN:
		if len(pluginInput.Image) == 0 {
			log.Errorf(ACTION_REQUIRES_PARAMETER, pluginInput.Action, "image")
			output.MarkAsFailed(fmt.Errorf(ACTION_REQUIRES_PARAMETER, pluginInput.Action, "image"))
			return
		}
		commandArguments = make([]string, 0)
		if pluginInput.Action == RUN {
			commandArguments = append(commandArguments, "run", "-d")
		} else {
			commandArguments = append(commandArguments, "create")
		}
		if len(pluginInput.Volume) > 0 && len(pluginInput.Volume[0]) > 0 {
			output.AppendInfo("pluginInput.Volume:" + strconv.Itoa(len(pluginInput.Volume)))

			log.Info("pluginInput.Volume", len(pluginInput.Volume))
			commandArguments = append(commandArguments, "--volume")
			for _, vol := range pluginInput.Volume {
				log.Info("pluginInput.Volume item", vol)
				commandArguments = append(commandArguments, vol)
			}
		}
		if len(pluginInput.Container) > 0 {
			commandArguments = append(commandArguments, "--name")
			commandArguments = append(commandArguments, pluginInput.Container)
		}
		if len(pluginInput.Memory) > 0 {
			commandArguments = append(commandArguments, "--memory")
			commandArguments = append(commandArguments, pluginInput.Memory)
		}
		if len(pluginInput.CpuShares) > 0 {
			commandArguments = append(commandArguments, "--cpu-shares")
			commandArguments = append(commandArguments, pluginInput.CpuShares)
		}
		if len(pluginInput.Publish) > 0 {
			commandArguments = append(commandArguments, "--publish")
			commandArguments = append(commandArguments, pluginInput.Publish)
		}
		if len(pluginInput.Env) > 0 {
			commandArguments = append(commandArguments, "--env")
			commandArguments = append(commandArguments, pluginInput.Env)
		}
		if len(pluginInput.User) > 0 {
			commandArguments = append(commandArguments, "--user")
			commandArguments = append(commandArguments, pluginInput.User)
		}
		commandArguments = append(commandArguments, pluginInput.Image)
		commandArguments = append(commandArguments, pluginInput.Cmd)

	case START:
		commandArguments = append(commandArguments, "start")
		if len(pluginInput.Container) == 0 {
			log.Errorf(ACTION_REQUIRES_PARAMETER, pluginInput.Action, "container")
			output.MarkAsFailed(fmt.Errorf(ACTION_REQUIRES_PARAMETER, pluginInput.Action, "container"))
			return
		}
		commandArguments = append(commandArguments, pluginInput.Container)

	case RM:
		commandArguments = append(commandArguments, "rm")
		if len(pluginInput.Container) == 0 {
			log.Errorf(ACTION_REQUIRES_PARAMETER, pluginInput.Action, "container")
			output.MarkAsFailed(fmt.Errorf(ACTION_REQUIRES_PARAMETER, pluginInput.Action, "container"))
			return
		}
		commandArguments = append(commandArguments, pluginInput.Container)

	case STOP:
		commandArguments = append(commandArguments, "stop")
		if len(pluginInput.Container) == 0 {
			log.Errorf(ACTION_REQUIRES_PARAMETER, pluginInput.Action, "container")
			output.MarkAsFailed(fmt.Errorf(ACTION_REQUIRES_PARAMETER, pluginInput.Action, "container"))
			return
		}
		commandArguments = append(commandArguments, pluginInput.Container)

	case EXEC:
		commandArguments = append(commandArguments, "exec")
		if len(pluginInput.Container) == 0 {
			log.Errorf(ACTION_REQUIRES_PARAMETER, pluginInput.Action, "container")
			output.MarkAsFailed(fmt.Errorf(ACTION_REQUIRES_PARAMETER, pluginInput.Action, "container"))
			return
		}
		if len(pluginInput.Cmd) == 0 {
			log.Errorf(ACTION_REQUIRES_PARAMETER, pluginInput.Action, "cmd")
			output.MarkAsFailed(fmt.Errorf(ACTION_REQUIRES_PARAMETER, pluginInput.Action, "cmd"))
			return
		}
		if len(pluginInput.User) > 0 {
			commandArguments = append(commandArguments, "--user")
			commandArguments = append(commandArguments, pluginInput.User)
		}
		commandArguments = append(commandArguments, pluginInput.Container)
		commandArguments = append(commandArguments, pluginInput.Cmd)
	case INSPECT:
		commandArguments = append(commandArguments, "inspect")
		if len(pluginInput.Container) == 0 && len(pluginInput.Image) == 0 {
			log.Errorf(ACTION_REQUIRES_PARAMETER, pluginInput.Action, "container or image")
			output.MarkAsFailed(fmt.Errorf(ACTION_REQUIRES_PARAMETER, pluginInput.Action, "container or image"))
			return
		}
		commandArguments = append(commandArguments, pluginInput.Container)
		commandArguments = append(commandArguments, pluginInput.Image)
	case STATS:
		commandArguments = append(commandArguments, "stats")
		commandArguments = append(commandArguments, "--no-stream")
		if len(pluginInput.Container) > 0 {
			commandArguments = append(commandArguments, pluginInput.Container)
		}
	case LOGS:
		commandArguments = append(commandArguments, "logs")
		if len(pluginInput.Container) == 0 {
			log.Errorf(ACTION_REQUIRES_PARAMETER, pluginInput.Action, "container")
			output.MarkAsFailed(fmt.Errorf(ACTION_REQUIRES_PARAMETER, pluginInput.Action, "container"))
			return
		}
		commandArguments = append(commandArguments, pluginInput.Container)
	case PULL:
		commandArguments = append(commandArguments, "pull")
		if len(pluginInput.Image) == 0 {
			log.Errorf(ACTION_REQUIRES_PARAMETER, pluginInput.Action, "image")
			output.MarkAsFailed(fmt.Errorf(ACTION_REQUIRES_PARAMETER, pluginInput.Action, "image"))
			return
		}
		commandArguments = append(commandArguments, pluginInput.Image)
	case IMAGES:
		commandArguments = append(commandArguments, "images")
	case RMI:
		commandArguments = append(commandArguments, "rmi")
		if len(pluginInput.Image) == 0 {
			log.Errorf(ACTION_REQUIRES_PARAMETER, pluginInput.Action, "image")
			output.MarkAsFailed(fmt.Errorf(ACTION_REQUIRES_PARAMETER, pluginInput.Action, "image"))
			return
		}
		commandArguments = append(commandArguments, pluginInput.Image)

	case PS:
		commandArguments = append(commandArguments, "ps", "--all")
	default:
		output.MarkAsFailed(fmt.Errorf("Docker Action is set to unsupported value: %v", pluginInput.Action))
		return
	}

	executionTimeout := pluginutil.ValidateExecutionTimeout(log, pluginInput.TimeoutSeconds)

	// Execute Command
	exitCode, err := p.CommandExecuter.NewExecute(log, pluginInput.WorkingDirectory, output.GetStdoutWriter(), output.GetStderrWriter(), cancelFlag, executionTimeout, commandName, commandArguments)

	// Set output status
	output.SetExitCode(exitCode)
	output.SetStatus(pluginutil.GetStatus(exitCode, cancelFlag))

	if err != nil {
		status := output.GetStatus()
		if status != contracts.ResultStatusCancelled &&
			status != contracts.ResultStatusTimedOut &&
			status != contracts.ResultStatusSuccessAndReboot {
			output.MarkAsFailed(fmt.Errorf("failed to run commands: %v", err))
		}
	}
	return
}

func validateInputs(pluginInput DockerContainerPluginInput) (err error) {
	validContainerName := regexp.MustCompile(`^[a-zA-Z0-9_\-\\\/]*$`)
	if !validContainerName.MatchString(pluginInput.Container) {
		return errors.New("Invalid container name, only [a-zA-Z0-9_-] are allowed")
	}
	validImageValue := regexp.MustCompile(`^[a-zA-Z0-9_\-\\\/]*$`)
	if !validImageValue.MatchString(pluginInput.Image) {
		return errors.New("Invalid image value, only [a-zA-Z0-9_-] are allowed")
	}
	validUserValue := regexp.MustCompile(`^[a-zA-Z0-9_-]*$`)
	if !validUserValue.MatchString(pluginInput.User) {
		return errors.New("Invalid user value")
	}
	validPathName := regexp.MustCompile(`^[\w\\\/_\:\-\.\"\(\)\^ ]*$`)
	for _, vol := range pluginInput.Volume {
		if !validPathName.MatchString(vol) {
			return errors.New("Invalid volume")
		}
	}
	validCpuSharesValue := regexp.MustCompile(`^/?[a-zA-Z0-9_-]*$`)
	if !validCpuSharesValue.MatchString(pluginInput.CpuShares) {
		return errors.New("Invalid CpuShares value, only integars are allowed")
	}
	validMemoryValue := regexp.MustCompile(`^[0-9]*[bkmg]?$`)
	if !validMemoryValue.MatchString(pluginInput.Memory) {
		return errors.New("Invalid CpuShares value")
	}
	validPublishValue := regexp.MustCompile(`^[0-9a-zA-Z:\-\/.]*$`)
	if !validPublishValue.MatchString(pluginInput.Publish) {
		return errors.New("Invalid Publish value")
	}
	blacklist := regexp.MustCompile(`[;,&|]+`)
	if blacklist.MatchString(pluginInput.Env) {
		return errors.New("Invalid environment variable value")
	}
	if blacklist.MatchString(pluginInput.Cmd) {
		return errors.New("Invalid command value")
	}

	return err
}
