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

// Package cloudwatch implements cloudwatch plugin

package cloudwatch

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"strings"

	"errors"
	"strconv"

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

	//CloudWatch Exe Absolute Path
	CloudWatchExePath = "C:\\Users\\Administrator\\Desktop\\Cloudwatch\\Debug\\EC2Config.CloudWatch.Standalone.exe"

	// Location of CloudWatch
	CloudWatchWorkingDir = "C:\\Users\\Administrator\\Desktop\\Cloudwatch\\Debug"

	// CloudWatch Exe Absolute Path
	CloudWatchProcessName = "EC2Config.CloudWatch.Standalone"
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
	plugin.WorkingDir = CloudWatchWorkingDir
	plugin.ExeLocation = CloudWatchExePath

	//Process details of cloudwatch.exe will be stored here accordingly
	plugin.Process = os.Process{}
	plugin.Name = "aws:cloudWatch"

	//health check specific stuff will be done here
	instanceId, _ := platform.InstanceID()
	plugin.DefaultHealthCheckOrchestrationDir = filepath.Join(appconfig.DefaultDataStorePath,
		instanceId,
		appconfig.LongRunningPluginsLocation,
		appconfig.LongRunningPluginsHealthCheck,
		plugin.Name)
	_ = fileutil.MakeDirsWithExecuteAccess(plugin.DefaultHealthCheckOrchestrationDir)

	exec := executers.ShellCommandExecuter{}
	plugin.ExecuteCommand = pluginutil.CommandExecuter(exec.StartExe)

	return &plugin, nil
}

// IsRunning returns if the said plugin is running or not
func (p *Plugin) IsRunning(context context.T) bool {
	log := context.Log()
	//working directory here doesn't really matter much since we run a powershell script to determine if exe is running
	return p.IsCloudWatchExeRunning(log, p.DefaultHealthCheckOrchestrationDir, p.DefaultHealthCheckOrchestrationDir, task.NewChanneledCancelFlag())
}

func (p *Plugin) Start(context context.T, configuration string, orchestrationDir string, cancelFlag task.CancelFlag) {
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

	var err error
	if useTempDirectory {
		if tempDir, err = ioutil.TempDir("", "Ec2RunCommand"); err != nil {
			log.Error(err)
			//return err
		}
		orchestrationDir = tempDir
	}

	//workingDirectory -> is the location where the exe runs from -> for cloudwatch this is where all configurations are present
	orchestrationDir = fileutil.RemoveInvalidChars(filepath.Join(orchestrationDir, p.Name))
	log.Debugf("Cloudwatch specific commands will be run in workingDirectory %v; orchestrationDir %v ", p.WorkingDir, orchestrationDir)

	// create orchestration dir if needed
	if !fileutil.Exists(orchestrationDir) {
		if err = fileutil.MakeDirsWithExecuteAccess(orchestrationDir); err != nil {
			log.Errorf("Encountered error while creating orchestrationDir directory %s:%s", orchestrationDir, err.Error())
			//return err
		}
	}

	//check if cloudwatch.exe is already running or not
	if p.IsCloudWatchExeRunning(log, p.WorkingDir, orchestrationDir, cancelFlag) {
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
	//todo: For testing purpose we have hardcoded the configuration here -> remove this later.
	config := `{"EngineConfiguration":{"PollInterval":"00:00:15","Components":[{"Id":"OsCpuUtilization","FullName":"AWS.EC2.Windows.CloudWatch.PerformanceCounterComponent.PerformanceCounterInputComponent,AWS.EC2.Windows.CloudWatch","Parameters":{"CategoryName":"Process","CounterName":"% Processor Time","InstanceName":"_Total","MetricName":"OsCpuUtilization","Unit":"Percent","DimensionName":"","DimensionValue":""}},{"Id":"OsMemoryUsage","FullName":"AWS.EC2.Windows.CloudWatch.PerformanceCounterComponent.PerformanceCounterInputComponent,AWS.EC2.Windows.CloudWatch","Parameters":{"CategoryName":"Memory","CounterName":"Available MBytes","InstanceName":"","MetricName":"Memory","Unit":"Megabytes","DimensionName":"","DimensionValue":""}},{"Id":"Ec2ConfigCpuUtilization","FullName":"AWS.EC2.Windows.CloudWatch.PerformanceCounterComponent.PerformanceCounterInputComponent,AWS.EC2.Windows.CloudWatch","Parameters":{"CategoryName":"Process","CounterName":"% Processor Time","InstanceName":"Ec2Config","MetricName":"Ec2Config-CpuUtilization","Unit":"Percent","DimensionName":"","DimensionValue":""}},{"Id":"Ec2ConfigMemoryUsage","FullName":"AWS.EC2.Windows.CloudWatch.PerformanceCounterComponent.PerformanceCounterInputComponent,AWS.EC2.Windows.CloudWatch","Parameters":{"CategoryName":"Process","CounterName":"Working Set - Private","InstanceName":"Ec2Config","MetricName":"Ec2Config-MemoryUsage","Unit":"Bytes","DimensionName":"","DimensionValue":""}},{"Id":"CloudWatch","FullName":"AWS.EC2.Windows.CloudWatch.CloudWatch.CloudWatchOutputComponent,AWS.EC2.Windows.CloudWatch","Parameters":{"AccessKey":"","SecretKey":"","Region":"us-west-2b","NameSpace":"SSMTest1"}},{"Id":"CloudWatchLogsForEC2ConfigService","FullName":"AWS.EC2.Windows.CloudWatch.CloudWatchLogsOutput,AWS.EC2.Windows.CloudWatch","Parameters":{"Region":"us-east-1","LogGroup":"EC2ConfigLogGroup","LogStream":"{instance_id}"}},{"Id":"CloudWatchTestTextLogs","FullName":"AWS.EC2.Windows.CloudWatch.CloudWatchLogsOutput,AWS.EC2.Windows.CloudWatch","Parameters":{"Region":"us-east-1","LogGroup":"CloudWatch-Test-LogGroup","LogStream":"{instance_id}-text"}},{"Id":"CloudWatchTestEventLogs","FullName":"AWS.EC2.Windows.CloudWatch.CloudWatchLogsOutput,AWS.EC2.Windows.CloudWatch","Parameters":{"Region":"us-east-1","LogGroup":"CloudWatch-Test-LogGroup","LogStream":"{instance_id}-event"}},{"Id":"TextLogs","FullName":"AWS.EC2.Windows.CloudWatch.CustomLog.CustomLogInputComponent,AWS.EC2.Windows.CloudWatch","Parameters":{"LogDirectoryPath":"C:\\CustomLogs\\","TimestampFormat":"MM/dd/yyyy HH:mm:ss,fff","Encoding":"UTF-8","Filter":"","CultureName":"en-US","TimeZoneKind":"Local","LineCount":"1"}},{"Id":"EventLogs","FullName":"AWS.EC2.Windows.CloudWatch.EventLog.EventLogInputComponent,AWS.EC2.Windows.CloudWatch","Parameters":{"LogName":"CloudWatch Test Log","Levels":"7"}},{"Id":"Ec2ConfigETW","FullName":"AWS.EC2.Windows.CloudWatch.EventLog.EventLogInputComponent,AWS.EC2.Windows.CloudWatch","Parameters":{"LogName":"EC2ConfigService","Levels":"7"}}],"Flows":{"Flows":["TextLogs,CloudWatchTestTextLogs"]}}}`
	commandArguments = append(commandArguments, "i-0191b213af64399f0", "us-east-1", config)

	log.Infof("commandName: %s", commandName)
	log.Infof("arguments passed: %s", commandArguments)

	//start the new process
	stdoutFilePath := filepath.Join(orchestrationDir, p.StdoutFileName)
	stderrFilePath := filepath.Join(orchestrationDir, p.StderrFileName)

	//executionTimeout -> starting cloudwatch.exe shouldn't take more than 600 seconds
	//NOTE: exec.StartExe doesn't honor executionTimeout
	executionTimeout := pluginutil.ValidateExecutionTimeout(log, 600)

	stdout, stderr, exitCode, errs := p.ExecuteCommand(log, p.WorkingDir, stdoutFilePath, stderrFilePath, cancelFlag, executionTimeout, commandName, commandArguments)

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

	log.Infof("stdout - %s", sout)
	log.Infof("stderr - %s", serr)
	log.Infof("exitCode - %v", exitCode)
	log.Infof("errs - %v", errs)

	//Storing Cloudwatch process details
	p.Process = *executers.Process
	log.Infof("Process id of cloudwatch.exe -> %v", p.Process.Pid)
}

// StopCloudWatchExe returns true if it successfully killed the cloudwatch exe or else it returns false
func (p *Plugin) Stop(context context.T, cancelFlag task.CancelFlag) {
	log := context.Log()

	//By default p.Process.Pid = 0 (when struct is not assigned any value)
	//For windows, pid = 0 means Idle process -> serious things can go wrong we even try to kill that process

	//If p.Process.Pid == 0, get process id of cloudwatch.exe
	if p.Process.Pid == 0 {
		log.Infof("Pid = 0 in windows is system idle process and is definitely Cloudwatch.exe. Getting Pid of Cloudwatch.exe")

		pid, err := p.GetPidOfCloudWatchExe(log,
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

// IsCloudWatchExeRunning runs a powershell script to determine if the given process is running
func (p *Plugin) IsCloudWatchExeRunning(log log.T, orchestrationDir, workingDirectory string, cancelFlag task.CancelFlag) bool {
	//todo: this should return error instead
	/*
		Since most functions in "os" package in GoLang isn't implemented for Windows platform, we run a powershell
		script (using Get-Process) to get process details in Windows.

		Also, since this is a powershell script just like the one that aws:runPowerShellScript plugin executes, we will
		use exec.StartExe as pluginutil.CommandExecuter. After we are done, we will revert back to using exec.Execute for
		future commands to enable/disable cloudwatch.
	*/
	var err error
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
			return false
		}
	}

	//create script path
	scriptPath := filepath.Join(orchestrationDir, pluginutil.RunCommandScriptName)

	// Create script file
	if err = pluginutil.CreateScriptFile(log, scriptPath, cmds); err != nil {
		log.Errorf("Failed to create script file : %v. Returning False because of this.", err)
		return false
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

	//Get-Process returned the Pid -> means it was not null
	if strings.Contains(sout, "True") {
		log.Info("Process %s is running", cloudwatchProcessName)
		return true
	}

	log.Info("Process % is not running", cloudwatchProcessName)
	return false
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
