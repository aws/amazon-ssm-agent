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
//
// +build windows

// Package cloudwatch implements cloudwatch plugin and its configuration
package cloudwatch

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/executers"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

// Plugin is the type for the Cloudwatch plugin.
type Plugin struct {
	CommandExecuter                    executers.T
	Process                            os.Process
	WorkingDir                         string
	ExeLocation                        string
	Name                               string
	DefaultHealthCheckOrchestrationDir string
}

const (
	//TODO: Change the way the output is being returned to return exit codes
	IsProcessRunning = "$ProcessActive = Get-Process -Name %v -ErrorAction SilentlyContinue ; $ProcessActive -ne $null"
	GetPidOfExe      = "Get-Process -Name %v -ErrorAction SilentlyContinue | Select ProcessName, Id | ConvertTo-Json"
	ProcessNotFound  = "Process not found"
	// CloudWatchProcessName represents CloudWatch Exe Absolute Path
	CloudWatchProcessName = "AWS.CloudWatch"
	// CloudWatchExeName represents the name of the executable file of cloud watch
	CloudWatchExeName = "AWS.CloudWatch.exe"
	// CloudWatchFolderName represents the default folder name for cloud watch plugin
	CloudWatchFolderName = "awsCloudWatch"
)

// CloudwatchProcessInfo is a structure for info returned by Cloudwatch process
type CloudwatchProcessInfo struct {
	ProcessName string `json:"ProcessName"`
	PId         int    `json:"Id"`
}

// Assign method to global variables to allow unittest to override
// TODO change these to deps.go later
var fileExist = fileutil.Exists
var getInstanceId = platform.InstanceID
var getRegion = platform.Region
var exec = executers.ShellCommandExecuter{}

// var createScript = pluginutil.CreateScriptFile

//todo: honor cancel flag for Start
//todo: honor cancel flag for Stop
//todo: Start,Stop -> should return plugin.result or error as well -> so that caller can report the results/errors accordingly.
// NewPlugin returns a new instance of Cloudwatch plugin
func NewPlugin(pluginConfig iohandler.PluginConfig) (*Plugin, error) {

	//Note: This is a wrapper on top of cloudwatch.exe - basically this executes the exe in a separate process.

	var plugin Plugin
	plugin.WorkingDir = fileutil.BuildPath(appconfig.DefaultPluginPath, CloudWatchFolderName)
	plugin.ExeLocation = filepath.Join(plugin.WorkingDir, CloudWatchExeName)

	//Process details of cloudwatch.exe will be stored here accordingly
	plugin.Process = os.Process{}
	plugin.Name = Name()

	//health check specific stuff will be done here
	instanceId, _ := platform.InstanceID()
	plugin.DefaultHealthCheckOrchestrationDir = fileutil.BuildPath(appconfig.DefaultDataStorePath,
		instanceId,
		appconfig.LongRunningPluginsLocation,
		appconfig.LongRunningPluginsHealthCheck,
		plugin.Name)
	_ = fileutil.MakeDirsWithExecuteAccess(plugin.DefaultHealthCheckOrchestrationDir)
	plugin.CommandExecuter = exec

	return &plugin, nil
}

// Name returns the plugin name
func Name() string {
	return appconfig.PluginNameCloudWatch
}

// IsRunning returns if the said plugin is running or not
func (p *Plugin) IsRunning(context context.T) bool {
	log := context.Log()
	//working directory here doesn't really matter much since we run a powershell script to determine if exe is running
	return p.IsCloudWatchExeRunning(log, p.DefaultHealthCheckOrchestrationDir, p.DefaultHealthCheckOrchestrationDir, task.NewChanneledCancelFlag())
}

// Start starts the executable file and returns encountered errors
func (p *Plugin) Start(context context.T, configuration string, orchestrationDir string, cancelFlag task.CancelFlag, out iohandler.IOHandler) (err error) {
	log := context.Log()
	logFormatConfig := logger.PrintCWConfig(configuration, log)
	log.Infof("CloudWatch Configuration to be applied - %s ", logFormatConfig)

	//check if the exe is located
	if !fileExist(p.ExeLocation) {
		errorMessage := "Unable to locate cloudwatch.exe"
		log.Errorf(errorMessage)
		return errors.New(errorMessage)
	}

	//if no orchestration directory specified, create temp directory
	var useTempDirectory = (orchestrationDir == "")
	var tempDir string

	//var err error
	if useTempDirectory {
		if tempDir, err = ioutil.TempDir("", "Ec2RunCommand"); err != nil {
			log.Error(err)
			return
		}
		orchestrationDir = tempDir
	}

	//workingDirectory -> is the location where the exe runs from -> for cloudwatch this is where all configurations are present
	orchestrationDir = fileutil.BuildPath(orchestrationDir, p.Name)
	log.Debugf("Cloudwatch specific commands will be run in workingDirectory %v; orchestrationDir %v ", p.WorkingDir, orchestrationDir)
	// create orchestration dir if needed
	if !fileExist(orchestrationDir) {
		if err = fileutil.MakeDirsWithExecuteAccess(orchestrationDir); err != nil {
			log.Errorf("Encountered error while creating orchestrationDir directory %s:%s", orchestrationDir, err.Error())
			return
		}
	}

	//check if cloudwatch.exe is already running or not
	if p.IsCloudWatchExeRunning(log, p.DefaultHealthCheckOrchestrationDir, p.DefaultHealthCheckOrchestrationDir, cancelFlag) {
		log.Debug("Cloudwatch executable is already running. Starting to terminate the process")
		p.Stop(context, cancelFlag)
	}

	/*
		In general exec.Execute -> waits for the command to finish with added attribute to timeout and cancel the command
		We don't want that for Cloudwatch.exe -> because we simply launch the exe and forget about it, hence we are using
		exec.StartExe that just launches an exe.

		Also, for aws:runPowerShellScript, aws:psModule & aws:applications plugins -> we create a powershellscript which
		has all commands expressed as []string and then we execute that script. For cloudwatch we directly invoke the exe,
		 and that's why we don't have to create any powershellscript.
	*/

	//construct command name and arguments that will be run by executer
	commandName := p.ExeLocation
	var commandArguments []string
	var instanceId, instanceRegion string
	if instanceId, err = getInstanceId(); err != nil {
		log.Error("Cannot get the current instance ID")
		return
	}

	if instanceRegion, err = getRegion(); err != nil {
		log.Error("Cannot get the current instance region information")
		return
	}

	commandArguments = append(commandArguments, instanceId, instanceRegion, getFileName())

	value, _, err := pluginutil.LocalRegistryKeyGetStringsValue(appconfig.ItemPropertyPath, appconfig.ItemPropertyName)
	if err != nil {
		log.Debug("Cannot find customized proxy setting.")
	}
	// if user has customized proxy setting
	if (err == nil) && (len(value) != 0) {
		url, noProxy := pluginutil.GetProxySetting(value)
		if (len(url) != 0) && (len(noProxy) != 0) {
			commandArguments = append(commandArguments, url, noProxy)
		} else if len(url) != 0 {
			commandArguments = append(commandArguments, url)
		}
	}

	log.Debugf("commandName: %s", commandName)
	log.Debugf("arguments passed: %s", commandArguments)

	//start the new process
	stdoutFilePath := filepath.Join(orchestrationDir, "stdout")
	stderrFilePath := filepath.Join(orchestrationDir, "stderr")

	//remove previous output log files if they are present
	fileutil.DeleteFile(stdoutFilePath)
	fileutil.DeleteFile(stderrFilePath)

	process, exitCode, err := p.CommandExecuter.StartExe(log, p.WorkingDir, out.GetStdoutWriter(), out.GetStderrWriter(), cancelFlag, commandName, commandArguments)
	if err != nil || exitCode != 0 {
		return fmt.Errorf("Errors occurred while starting Cloudwatch exit code %v, error %v", exitCode, err)
	}

	// Cloudwatch process details
	p.Process = *process
	log.Infof("Process id of cloudwatch.exe -> %v", p.Process.Pid)

	return nil
}

// Stop returns true if it successfully killed the cloudwatch exe or else it returns false
func (p *Plugin) Stop(context context.T, cancelFlag task.CancelFlag) (err error) {
	log := context.Log()

	var cwProcInfo []CloudwatchProcessInfo
	if cwProcInfo, err = p.GetProcInfoOfCloudWatchExe(log,
		p.DefaultHealthCheckOrchestrationDir,
		p.DefaultHealthCheckOrchestrationDir,
		task.NewChanneledCancelFlag()); err != nil {
		log.Errorf("Can't stop cloudwatch because unable to find Pid of cloudwatch.exe : %s", err)
		return err
	}
	log.Info("The number of cloudwatch processes running are ", len(cwProcInfo))
	var processKillError error
	var currentProcess os.Process
	processKillError = nil
	//Iterating through the cwProcess info to in case multiple Cloudwatch processes are running.
	//All existing processes must be killed
	for _, cloudwatchInfo := range cwProcInfo {
		//Assigning existing cloudwatch process Id to currentProcess in order to kill that process.
		currentProcess.Pid = cloudwatchInfo.PId
		log.Debug("PID of Cloudwatch is ", p.Process.Pid)
		if err = currentProcess.Kill(); err != nil {
			// Continuing here without returning to kill whatever processes can be killed even if something
			// goes wrong. Return on error later
			log.Errorf("Encountered error while trying to kill the process %v : %s", p.Process.Pid, err)
			processKillError = err
		} else {
			log.Infof("Successfully killed the process %v", p.Process.Pid)
		}
	}
	if p.IsRunning(context) || processKillError != nil {
		log.Errorf("There was an error while killing Cloudwatch: %s", processKillError)
		return processKillError
	} else {
		log.Infof("All existing Cloudwatch processes killed successfully.")
	}
	return nil
}

// IsCloudWatchExeRunning runs a powershell script to determine if the given process is running
func (p *Plugin) IsCloudWatchExeRunning(log logger.T, workingDirectory, orchestrationDir string, cancelFlag task.CancelFlag) bool {
	/*
		Since most functions in "os" package in GoLang isn't implemented for Windows platform, we run a powershell
		script (using Get-Process) to get process details in Windows.
	*/
	//constructing the powershell command to execute
	var commandArguments []string
	var err error
	cloudwatchProcessName := CloudWatchProcessName
	cmdIsExeRunning := fmt.Sprintf(IsProcessRunning, cloudwatchProcessName)
	log.Debugf("Final cmd to check if process is still running is", cmdIsExeRunning)
	commandArguments = append(commandArguments, cmdIsExeRunning)

	// execute the command
	var commandOutput string
	if commandOutput, err = p.runPowerShell(log, workingDirectory, cancelFlag, commandArguments); err != nil {
		//TODO Returning false here because we are unsure if Cloudwatch is running. Trying to kill PID will lead to error. Handle this situation
		return false
	}

	log.Debugf("The output of IsCloudwatchExeRunning is %s", commandOutput)
	//Get-Process returned the Pid -> means it was not null
	if strings.Contains(commandOutput, "True") {
		log.Infof("Process %s is running", cloudwatchProcessName)
		return true
	} else if !strings.Contains(commandOutput, "False") {
		log.Infof("Multiple processes of %s running. Command output is ", cloudwatchProcessName, commandOutput)
		return true
	}

	log.Infof("Process %s is not running", cloudwatchProcessName)
	return false
}

// GetProcInfoOfCloudWatchExe runs a powershell script to determine the process ID of the Cloudwatch process. It should be called only after confirming that cloudwatch is running
func (p *Plugin) GetProcInfoOfCloudWatchExe(log logger.T, orchestrationDir, workingDirectory string, cancelFlag task.CancelFlag) (cwProcInfo []CloudwatchProcessInfo, err error) {
	//constructing the powershell command to execute
	var commandArguments []string
	cmdGetPidOfCW := fmt.Sprintf(GetPidOfExe, CloudWatchProcessName)
	log.Debugf("Command to get the PID info is ", cmdGetPidOfCW)
	commandArguments = append(commandArguments, cmdGetPidOfCW)

	// execute the command
	var commandOutput string
	if commandOutput, err = p.runPowerShell(log, workingDirectory, cancelFlag, commandArguments); err != nil {
		return cwProcInfo, err
	}

	//Since output is returned as a Json, checking to see if output is not in the form of an array
	//Output will be in the form of an array only in case of multiple Cloudwatch instances running
	if !strings.HasPrefix(commandOutput, "[") && !strings.HasSuffix(commandOutput, "]") {
		commandOutput = "[" + commandOutput + "]"
	}

	//Unmarshal the result into json obj.
	if err = jsonutil.Unmarshal(commandOutput, &cwProcInfo); err != nil {
		log.Errorf("Error unmarshalling Cloudwatch process information is %s", err)
		return cwProcInfo, err
	}

	return cwProcInfo, err
}

// runPowerShell is a wrapper around Execute command to run powershell script
func (p *Plugin) runPowerShell(log logger.T, workingDirectory string, cancelFlag task.CancelFlag, commandArguments []string) (commandOutput string, err error) {
	commandName := pluginutil.GetShellCommand()
	log.Infof("commandName: %s", commandName)
	log.Infof("arguments passed: %s", commandArguments)

	//If the stdoutFile and stderrFile path is empty, p.CommandExecuter.Execute return the output as a buffer
	stdoutFilePath := ""
	stderrFilePath := ""
	//executionTimeout -> determining if a process is running or not shouldn't take more than 60 seconds
	executionTimeout := pluginutil.ValidateExecutionTimeout(log, 60)

	//execute the command
	stdout, stderr, exitCode, errs := p.CommandExecuter.Execute(log, workingDirectory, stdoutFilePath,
		stderrFilePath, cancelFlag, executionTimeout, commandName, commandArguments)

	stdOutBuf := new(bytes.Buffer)
	stdOutBuf.ReadFrom(stdout)
	commandOutput = stdOutBuf.String()
	stdErrBuf := new(bytes.Buffer)
	stdErrBuf.ReadFrom(stderr)
	commandOutputError := stdErrBuf.String()

	//We don't expect any errors because the powershell script that we run has error action set as SilentlyContinue
	if commandOutputError != "" {
		log.Errorf("Powershell script to get process ID of the Cloudwatch executable currently running failed with error - %v", commandOutputError)
	}

	log.Debugf("exitCode - %v", exitCode)
	log.Debugf("errs - %v", errs)

	return commandOutput, nil
}
