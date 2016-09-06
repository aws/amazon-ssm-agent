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
//// +build windows
//
// Package cloudwatch implements cloudwatch plugin
package cloudwatch

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/executers"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

// Plugin is the type for the Cloudwatch plugin.
type Plugin struct {
	pluginutil.DefaultPlugin
	Process                            os.Process
	WorkingDir                         string
	ExeLocation                        string
	Name                               string
	DefaultHealthCheckOrchestrationDir string
}

const (
	IsProcessRunning = "$ProcessActive = Get-Process -Name %v -ErrorAction SilentlyContinue ; $ProcessActive -ne $null"
	GetPidOfExe      = "$ProcessActive = Get-Process -Name %v -ErrorAction SilentlyContinue ; if ($ProcessActive -ne $null) {$ProcessActive.Id} else {'Process not found'}"
	ProcessNotFound  = "Process not found"

	// CloudWatch Exe Absolute Path
	CloudWatchProcessName = "EC2Config.CloudWatch"

	// CloudWatch Exe Name
	CloudWatchExeName = "EC2Config.CloudWatch.exe"
	// TODO use constants and filepath.join after add build windows @zimo
	// SSM plugins folder path under local app data.
	SSMPluginFolder = "Amazon\\SSM\\Plugins\\"
)

//todo: honor cancel flag for Start
//todo: honor cancel flag for Stop
//todo: this should only run for windows platform -> make sure +build windows is added just after the package description
//todo: Start,Stop -> should return plugin.result or error as well -> so that caller can report the results/errors accordingly.

// NewPlugin returns a new instance of Cloudwatch plugin
func NewPlugin(pluginConfig pluginutil.PluginConfig) (*Plugin, error) {

	//Note: This is a wrapper on top of cloudwatch.exe - basically this executes the exe in a separate process.

	var plugin Plugin
	plugin.MaxStdoutLength = pluginConfig.MaxStdoutLength
	plugin.MaxStderrLength = pluginConfig.MaxStderrLength
	plugin.StdoutFileName = pluginConfig.StdoutFileName
	plugin.StderrFileName = pluginConfig.StderrFileName
	plugin.OutputTruncatedSuffix = pluginConfig.OutputTruncatedSuffix
	plugin.Uploader = pluginutil.GetS3Config()
	plugin.ExecuteUploadOutputToS3Bucket = pluginutil.UploadOutputToS3BucketExecuter(plugin.UploadOutputToS3Bucket)
	plugin.WorkingDir = filepath.Join(os.Getenv("ProgramFiles"), SSMPluginFolder, fileutil.RemoveInvalidChars(Name()))
	plugin.ExeLocation = filepath.Join(plugin.WorkingDir, CloudWatchExeName)

	//Process details of cloudwatch.exe will be stored here accordingly
	plugin.Process = os.Process{}
	plugin.Name = Name()

	//health check specific stuff will be done here
	instanceId, _ := platform.InstanceID()
	plugin.DefaultHealthCheckOrchestrationDir = filepath.Join(appconfig.DefaultDataStorePath,
		instanceId,
		appconfig.LongRunningPluginsLocation,
		appconfig.LongRunningPluginsHealthCheck,
		fileutil.RemoveInvalidChars(plugin.Name))
	_ = fileutil.MakeDirsWithExecuteAccess(plugin.DefaultHealthCheckOrchestrationDir)

	exec := executers.ShellCommandExecuter{}
	plugin.ExecuteCommand = pluginutil.CommandExecuter(exec.StartExe)

	return &plugin, nil
}

// Name returns the plugin name
func Name() string {
	return appconfig.PluginNameCloudWatch
}

// TODO refactor @zimo
// IsRunning returns if the said plugin is running or not
func (p *Plugin) IsRunning(context context.T) bool {
	log := context.Log()
	//working directory here doesn't really matter much since we run a powershell script to determine if exe is running
	return p.IsCloudWatchExeRunning(log, p.DefaultHealthCheckOrchestrationDir, p.DefaultHealthCheckOrchestrationDir, task.NewChanneledCancelFlag()) == nil
}

// Start starts the executable file and returns encountered errors
func (p *Plugin) Start(context context.T, configuration string, orchestrationDir string, cancelFlag task.CancelFlag) (err error) {
	log := context.Log()
	log.Infof("CloudWatch Configuration to be applied - %s", configuration)

	//check if the exe is located
	if !fileutil.Exists(p.ExeLocation) {
		log.Errorf("Unable to locate cloudwatch.exe")
		return
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
	// TODO: Separate RemoveInvalidChars method to windows and unix in fileutil
	orchestrationDir = filepath.Join(orchestrationDir, fileutil.RemoveInvalidChars(p.Name))
	log.Debugf("Cloudwatch specific commands will be run in workingDirectory %v; orchestrationDir %v ", p.WorkingDir, orchestrationDir)
	// create orchestration dir if needed
	if !fileutil.Exists(orchestrationDir) {
		if err = fileutil.MakeDirsWithExecuteAccess(orchestrationDir); err != nil {
			log.Errorf("Encountered error while creating orchestrationDir directory %s:%s", orchestrationDir, err.Error())
			return
		}
	}

	//check if cloudwatch.exe is already running or not
	// err != nil -> false, not running
	if err = p.IsCloudWatchExeRunning(log, p.WorkingDir, orchestrationDir, cancelFlag); err == nil {
		if err = p.Stop(context, cancelFlag); err != nil {
			// not stop successfully
			log.Errorf("Unable to disable current running cloudwatch. error: %s", err.Error())
			return
		}
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
	if instanceId, err = platform.InstanceID(); err != nil {
		log.Error("Cannot get the current instance ID")
		return
	}

	if instanceRegion, err = platform.Region(); err != nil {
		log.Error("Cannot get the current instance region information")
		return
	}

	commandArguments = append(commandArguments, instanceId, instanceRegion, configuration)

	log.Debugf("commandName: %s", commandName)
	log.Debugf("arguments passed: %s", commandArguments)

	//start the new process
	stdoutFilePath := filepath.Join(orchestrationDir, p.StdoutFileName)
	stderrFilePath := filepath.Join(orchestrationDir, p.StderrFileName)

	//executionTimeout -> starting cloudwatch.exe shouldn't take more than 600 seconds
	//NOTE: exec.StartExe doesn't honor executionTimeout
	executionTimeout := pluginutil.ValidateExecutionTimeout(log, 600)

	stdout, stderr, exitCode, errs := p.ExecuteCommand(log, p.WorkingDir, stdoutFilePath, stderrFilePath, cancelFlag, executionTimeout, commandName, commandArguments)

	// Have to wait sometime until finish writing logs
	duration := time.Duration(5) * time.Second
	time.Sleep(duration)

	// read standard output/error
	var sout, serr string
	sout, err = pluginutil.ReadPrefix(stdout, p.MaxStdoutLength, p.OutputTruncatedSuffix)
	if err != nil {
		log.Error(err)
	}

	serr, err = pluginutil.ReadPrefix(stderr, p.MaxStderrLength, p.OutputTruncatedSuffix)
	if err != nil {
		log.Error(err)
	}

	log.Debugf("stdout - %s", sout)
	log.Debugf("stderr - %s", serr)
	log.Debugf("exitCode - %v", exitCode)
	log.Debugf("errs - %v", errs)

	//Storing Cloudwatch process details
	p.Process = *executers.Process
	log.Infof("Process id of cloudwatch.exe -> %v", p.Process.Pid)

	return nil
}

// StopCloudWatchExe returns true if it successfully killed the cloudwatch exe or else it returns false
func (p *Plugin) Stop(context context.T, cancelFlag task.CancelFlag) (err error) {
	log := context.Log()

	//By default p.Process.Pid = 0 (when struct is not assigned any value)
	//For windows, pid = 0 means Idle process -> serious things can go wrong we even try to kill that process
	var pid int
	//If p.Process.Pid == 0, get process id of cloudwatch.exe
	if p.Process.Pid == 0 {
		log.Infof("Pid = 0 in windows is system idle process and is definitely Cloudwatch.exe. Getting Pid of Cloudwatch.exe")

		pid, err = p.GetPidOfCloudWatchExe(log,
			p.DefaultHealthCheckOrchestrationDir,
			p.DefaultHealthCheckOrchestrationDir,
			task.NewChanneledCancelFlag())
		if err != nil {
			log.Infof("Can't stop cloudwatch because unable to find Pid of cloudwatch.exe. It might not be even running.")
			return
		}

		p.Process.Pid = pid
	}

	if err := p.Process.Kill(); err != nil {
		log.Errorf("Encountered error while trying to kill the process %v : %s", p.Process.Pid, err.Error())
	} else {
		log.Infof("Successfully killed the process %v", p.Process.Pid)
	}

	return
}

// TODO refactor @zimo
// IsCloudWatchExeRunning runs a powershell script to determine if the given process is running
func (p *Plugin) IsCloudWatchExeRunning(log log.T, workingDirectory, orchestrationDir string, cancelFlag task.CancelFlag) (err error) {
	//todo: this should return error instead
	/*
		Since most functions in "os" package in GoLang isn't implemented for Windows platform, we run a powershell
		script (using Get-Process) to get process details in Windows.

		Also, since this is a powershell script just like the one that aws:runPowerShellScript plugin executes, we will
		use exec.StartExe as pluginutil.CommandExecuter. After we are done, we will revert back to using exec.Execute for
		future commands to enable/disable cloudwatch.
	*/
	//we need to wait for the command to finish so change this to Execute
	exec := executers.ShellCommandExecuter{}
	p.ExecuteCommand = pluginutil.CommandExecuter(exec.Execute)

	//constructing the powershell command to execute
	var cmds []string
	cloudwatchProcessName := CloudWatchProcessName
	cmdIsExeRunning := fmt.Sprintf(IsProcessRunning, cloudwatchProcessName)
	log.Debugf("Final cmd to check if process is still running : %s", cmdIsExeRunning)
	cmds = append(cmds, cmdIsExeRunning)

	//running commands to see if process exists or not
	orchestrationDir = filepath.Join(orchestrationDir, "IsExeRunning")

	// create orchestration dir if needed
	if !fileutil.Exists(orchestrationDir) {
		if err = fileutil.MakeDirsWithExecuteAccess(orchestrationDir); err != nil {
			log.Errorf("Encountered error while creating orchestrationDir directory %s:%s", orchestrationDir, err.Error())
			return
		}
	}

	//create script path
	scriptPath := filepath.Join(orchestrationDir, pluginutil.RunCommandScriptName)

	// Create script file
	if err = pluginutil.CreateScriptFile(log, scriptPath, cmds); err != nil {
		log.Errorf("Failed to create script file : %v. Returning False because of this.", err)
		return
	}

	//construct command name and arguments
	commandName := pluginutil.GetShellCommand()
	commandArguments := append(pluginutil.GetShellArguments(), scriptPath, pluginutil.ExitCodeTrap)
	log.Infof("check running commandName: %s", commandName)
	log.Infof("arguments passed: %s", commandArguments)

	//start the new process

	stdoutFilePath := filepath.Join(orchestrationDir, p.StdoutFileName)
	stderrFilePath := filepath.Join(orchestrationDir, p.StderrFileName)
	//executionTimeout -> determining if a process is running or not shouldn't take more than 600 seconds
	executionTimeout := pluginutil.ValidateExecutionTimeout(log, 600)

	stdout, stderr, exitCode, errs := p.ExecuteCommand(log, workingDirectory, stdoutFilePath, stderrFilePath, cancelFlag, executionTimeout, commandName, commandArguments)

	//Reverting back to exec.StartExe (that doesn't wait for the command to finish)
	p.ExecuteCommand = pluginutil.CommandExecuter(exec.StartExe)

	// read (a prefix of) the standard output/error
	var sout, serr string
	sout, err = pluginutil.ReadPrefix(stdout, p.MaxStdoutLength, p.OutputTruncatedSuffix)
	if err != nil {
		log.Error(err)
	}

	serr, err = pluginutil.ReadPrefix(stderr, p.MaxStderrLength, p.OutputTruncatedSuffix)
	if err != nil {
		log.Error(err)
	}

	log.Infof("stdout - %s", sout)
	log.Infof("stderr - %s", serr)
	log.Infof("exitCode - %v", exitCode)
	log.Infof("errs - %v", errs)
	//TODO not sure if this contains is correct, maybe need to read the last bool
	//Get-Process returned the Pid -> means it was not null
	if strings.Contains(sout, "True") {
		log.Info("Process %s is running", cloudwatchProcessName)
		return nil
	} else {
		log.Info("Process %s is not running", cloudwatchProcessName)
		return errors.New(fmt.Sprintf("Process is not running"))
	}
}

// IsCloudWatchExeRunning runs a powershell script to determine if the given process is running
func (p *Plugin) GetPidOfCloudWatchExe(log log.T, orchestrationDir, workingDirectory string, cancelFlag task.CancelFlag) (int, error) {
	/*
		We will execute a powershell script by using exec.StartExe as pluginutil.CommandExecuter. After we are
		done, we will revert back to using exec.Execute for future commands to enable/disable cloudwatch.
	*/

	var err error
	//we need to wait for the command to finish so change this to Execute
	exec := executers.ShellCommandExecuter{}
	p.ExecuteCommand = pluginutil.CommandExecuter(exec.Execute)
	//constructing the powershell command to execute
	var cmds []string
	cloudwatchProcessName := CloudWatchProcessName
	cmdIsExeRunning := fmt.Sprintf(GetPidOfExe, cloudwatchProcessName)
	log.Debugf("Final cmd to check if process is still running : %s", cmdIsExeRunning)
	cmds = append(cmds, cmdIsExeRunning)

	//running commands to see if process exists or not
	orchestrationDir = filepath.Join(orchestrationDir, "GetPidOfExe")

	// create orchestration dir if needed
	if !fileutil.Exists(orchestrationDir) {
		if err = fileutil.MakeDirsWithExecuteAccess(orchestrationDir); err != nil {
			log.Errorf("Encountered error while creating orchestrationDir directory %s:%s", orchestrationDir, err.Error())
			return 0, errors.New(fmt.Sprintf("Couldn't create orchestrationDirectory to find Pid of cloudwatch"))
		}
	}

	//create script path
	scriptPath := filepath.Join(orchestrationDir, pluginutil.RunCommandScriptName)

	// Create script file
	if err = pluginutil.CreateScriptFile(log, scriptPath, cmds); err != nil {
		log.Errorf("Failed to create script file : %v. Returning False because of this.", err)
		return 0, errors.New(fmt.Sprintf("Couldn't create script file to find Pid of cloudwatch"))
	}

	//construct command name and arguments
	commandName := pluginutil.GetShellCommand()
	commandArguments := append(pluginutil.GetShellArguments(), scriptPath, pluginutil.ExitCodeTrap)
	log.Infof("commandName: %s", commandName)
	log.Infof("arguments passed: %s", commandArguments)

	//start the new process

	stdoutFilePath := filepath.Join(orchestrationDir, p.StdoutFileName)
	stderrFilePath := filepath.Join(orchestrationDir, p.StderrFileName)
	//executionTimeout -> determining if a process is running or not shouldn't take more than 600 seconds
	executionTimeout := pluginutil.ValidateExecutionTimeout(log, 600)

	stdout, stderr, exitCode, errs := p.ExecuteCommand(log, workingDirectory, stdoutFilePath, stderrFilePath, cancelFlag, executionTimeout, commandName, commandArguments)

	//Reverting back to exec.StartExe (that doesn't wait for the command to finish)
	p.ExecuteCommand = pluginutil.CommandExecuter(exec.StartExe)

	// read (a prefix of) the standard output/error
	var sout, serr string
	sout, err = pluginutil.ReadPrefix(stdout, p.MaxStdoutLength, p.OutputTruncatedSuffix)
	if err != nil {
		log.Error(err)
	}

	serr, err = pluginutil.ReadPrefix(stderr, p.MaxStderrLength, p.OutputTruncatedSuffix)
	if err != nil {
		log.Error(err)
	}

	log.Infof("stdout - %s", sout)
	log.Infof("stderr - %s", serr)
	log.Infof("exitCode - %v", exitCode)
	log.Infof("errs - %v", errs)

	//We don't expect any errors because the powershell script that we run has error action set as SilentlyContinue
	//Parse pid from the output
	if strings.Contains(sout, ProcessNotFound) {
		log.Infof("Process %s is not running", cloudwatchProcessName)
		return 0, errors.New(fmt.Sprintf("%s is not running", cloudwatchProcessName))
	} else {
		if pid, err := strconv.Atoi(strings.TrimSpace(sout)); err != nil {
			log.Infof("Unable to get parse pid from command output: %s", sout)
			return 0, err
		} else {
			log.Infof("Pid %v after parsing from %s", pid, sout)
			return pid, nil
		}
	}
}
