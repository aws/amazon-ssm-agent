// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package ssminstaller implements the installer for ssm packages that use documents or scripts to install and uninstall.
package ssminstaller

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/executers"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/envdetect"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/trace"
	"github.com/aws/amazon-ssm-agent/agent/times"
)

type Installer struct {
	filesysdep         fileSysDep
	execdep            execDep
	packageName        string
	version            string
	packagePath        string
	config             contracts.Configuration // TODO:MF: See if we can use a smaller struct that has just the things we need
	envdetectCollector envdetect.Collector
}

type ActionType uint8

const (
	ACTION_TYPE_SH  ActionType = iota
	ACTION_TYPE_PS1 ActionType = iota
)

type Action struct {
	actionName string
	filepath   string
	actionType ActionType
}

func New(packageName string,
	version string,
	packagePath string,
	configuration contracts.Configuration,
	envdetectCollector envdetect.Collector) *Installer {
	return &Installer{
		filesysdep:         &fileSysDepImp{},
		execdep:            &execDepImp{},
		packageName:        packageName,
		version:            version,
		packagePath:        packagePath,
		config:             configuration,
		envdetectCollector: envdetectCollector,
	}
}

func (inst *Installer) Install(tracer trace.Tracer, context context.T) contracts.PluginOutputter {
	return inst.executeAction(tracer, context, "install")
}

func (inst *Installer) Uninstall(tracer trace.Tracer, context context.T) contracts.PluginOutputter {
	return inst.executeAction(tracer, context, "uninstall")
}

func (inst *Installer) Validate(tracer trace.Tracer, context context.T) contracts.PluginOutputter {
	return inst.executeAction(tracer, context, "validate")
}

func (inst *Installer) Version() string {
	return inst.version
}

func (inst *Installer) PackageName() string {
	return inst.packageName
}

// executeAction will execute the installer scripts if they exist.
func (inst *Installer) executeAction(tracer trace.Tracer, context context.T, actionName string) contracts.PluginOutputter {
	exectrace := tracer.BeginSection(fmt.Sprintf("execute action: %s", actionName))

	output := &trace.PluginOutputTrace{Tracer: tracer}
	output.SetStatus(contracts.ResultStatusSuccess)

	exists, pluginsInfo, _, orchestrationDir, err := inst.readAction(tracer, context, actionName)
	if exists {
		if err != nil {
			exectrace.WithError(err)
			output.MarkAsFailed(nil, nil)
		}
		exectrace.AppendInfof("Initiating %v %v %v", inst.packageName, inst.version, actionName)
		inst.executeDocument(tracer, context, actionName, orchestrationDir, pluginsInfo, output)
	}

	exectrace.End()
	return output
}

// getActionPath is a helper function that builds the path to an action document file
func (inst *Installer) getActionPath(actionName string, extension string) string {
	return filepath.Join(inst.packagePath, fmt.Sprintf("%v.%v", actionName, extension))
}

func (inst *Installer) readScriptAction(action *Action, workingDir string, orchestrationDir string, pluginName string, runCommand []interface{}) (pluginsInfo []contracts.PluginState, err error) {
	pluginsInfo = []contracts.PluginState{}

	pluginFullName := fmt.Sprintf("aws:%v", pluginName)
	var s3Prefix string
	if inst.config.OutputS3BucketName != "" {
		s3Prefix = fileutil.BuildS3Path(inst.config.OutputS3KeyPrefix, inst.config.PluginID, action.actionName, pluginFullName)
	}

	inputs := make(map[string]interface{})
	inputs["workingDirectory"] = workingDir
	inputs["runCommand"] = runCommand

	config := contracts.Configuration{
		Settings:                nil,
		Properties:              inputs,
		OutputS3BucketName:      inst.config.OutputS3BucketName,
		OutputS3KeyPrefix:       s3Prefix,
		OrchestrationDirectory:  orchestrationDir,
		MessageId:               inst.config.MessageId,
		BookKeepingFileName:     inst.config.BookKeepingFileName,
		PluginName:              pluginFullName,
		PluginID:                inst.version,
		Preconditions:           make(map[string][]string),
		IsPreconditionEnabled:   false,
		DefaultWorkingDirectory: workingDir,
	}

	var plugin contracts.PluginState
	plugin.Configuration = config
	plugin.Id = config.PluginID
	plugin.Name = config.PluginName
	pluginsInfo = append(pluginsInfo, plugin)

	return pluginsInfo, nil
}

// readShAction turns an sh action into a set of SSM Document Plugins to execute
func (inst *Installer) readShAction(context context.T, action *Action, workingDir string, orchestrationDir string, envVars map[string]string) (pluginsInfo []contracts.PluginState, err error) {
	if action.actionType != ACTION_TYPE_SH {
		return nil, fmt.Errorf("Internal error")
	}

	runCommand := []interface{}{}
	runCommand = append(runCommand, fmt.Sprintf("echo Running sh %v.sh", action.actionName))

	for k, v := range envVars {
		v = executers.QuoteShString(v)
		runCommand = append(runCommand, fmt.Sprintf("export %v=%v", k, v))
	}

	runCommand = append(runCommand, fmt.Sprintf("sh %v.sh", action.actionName))

	return inst.readScriptAction(action, workingDir, orchestrationDir, "runShellScript", runCommand)
}

// readPs1Action turns an ps1 action into a set of SSM Document Plugins to execute
func (inst *Installer) readPs1Action(context context.T, action *Action, workingDir string, orchestrationDir string, envVars map[string]string) (pluginsInfo []contracts.PluginState, err error) {
	if action.actionType != ACTION_TYPE_PS1 {
		return nil, fmt.Errorf("Internal error")
	}

	runCommand := []interface{}{}
	runCommand = append(runCommand, fmt.Sprintf("echo 'Running %v.ps1'", action.actionName))

	for k, v := range envVars {
		v = executers.QuotePsString(v)
		runCommand = append(runCommand, fmt.Sprintf("$env:%v = %v", k, v))
	}

	runCommand = append(runCommand, fmt.Sprintf(".\\%v.ps1; exit $LASTEXITCODE", action.actionName))

	return inst.readScriptAction(action, workingDir, orchestrationDir, "runPowerShellScript", runCommand)
}

// resolveAction checks if there are multiple installer files for the same action type
// and returns false and nil error if the file does not exist.
func (inst *Installer) resolveAction(tracer trace.Tracer, actionName string) (exists bool, action *Action, err error) {
	actionPathSh := inst.getActionPath(actionName, "sh")
	actionPathPs1 := inst.getActionPath(actionName, "ps1")

	actionPathExistsSh := inst.filesysdep.Exists(actionPathSh)
	actionPathExistsPs1 := inst.filesysdep.Exists(actionPathPs1)
	countExists := 0

	actionTemp := &Action{}

	if actionPathExistsSh {
		countExists += 1
		actionTemp.actionName = actionName
		actionTemp.actionType = ACTION_TYPE_SH
		actionTemp.filepath = actionPathSh
	}
	if actionPathExistsPs1 {
		countExists += 1
		actionTemp.actionName = actionName
		actionTemp.actionType = ACTION_TYPE_PS1
		actionTemp.filepath = actionPathPs1
	}

	if countExists > 1 {
		err = fmt.Errorf("%v has more than one implementation (sh, ps1, json)", actionName)
		tracer.CurrentTrace().WithError(err)
		return true, nil, err
	} else if countExists == 1 {
		return true, actionTemp, nil
	}

	return false, nil, nil
}

func (inst *Installer) getEnvVars(actionName string, context context.T) (envVars map[string]string, err error) {
	log := context.Log()

	envVars = make(map[string]string)

	envVars["BWS_ACTION_NAME"] = actionName

	// Copy proxy settings from the environment
	envVars["https_proxy"] = os.Getenv("https_proxy")
	envVars["http_proxy"] = os.Getenv("http_proxy")
	envVars["no_proxy"] = os.Getenv("no_proxy")

	env, err := inst.envdetectCollector.CollectData(log)
	if err != nil {
		return envVars, fmt.Errorf("failed to collect data: %v", err)
	}

	// (Some of these are already available to script as AWS_SSM_INSTANCE_ID and AWS_SSM_REGION_NAME)
	envVars["BWS_PLATFORM_NAME"] = env.OperatingSystem.Platform
	envVars["BWS_PLATFORM_VERSION"] = env.OperatingSystem.PlatformVersion
	envVars["BWS_PLATFORM_FAMILY"] = env.OperatingSystem.PlatformFamily
	envVars["BWS_ARCHITECTURE"] = env.OperatingSystem.Architecture
	envVars["BWS_INIT_SYSTEM"] = env.OperatingSystem.InitSystem
	envVars["BWS_PACKAGE_MANAGER"] = env.OperatingSystem.PackageManager
	envVars["BWS_INSTANCE_ID"] = env.Ec2Infrastructure.InstanceID
	envVars["BWS_INSTANCE_TYPE"] = env.Ec2Infrastructure.InstanceType
	envVars["BWS_REGION"] = env.Ec2Infrastructure.Region
	envVars["BWS_ACCOUNT_ID"] = env.Ec2Infrastructure.AccountID
	envVars["BWS_AVAILABILITY_ZONE"] = env.Ec2Infrastructure.AvailabilityZone

	return envVars, err
}

// readAction returns a JSON document describing a management action and its working directory, or an empty string
// if there is nothing to do for a given action
func (inst *Installer) readAction(tracer trace.Tracer, context context.T, actionName string) (exists bool, pluginsInfo []contracts.PluginState, workingDir string, orchestrationDir string, err error) {
	// TODO: Split into linux and windows

	var action *Action

	if exists, action, err = inst.resolveAction(tracer, actionName); !exists || action == nil || err != nil {
		// If the action file does not exist (for eg validate) then this method will return here, with no error.
		// It could also return if there is an error
		return exists, nil, "", "", err
	}

	workingDir = inst.packagePath
	orchestrationDir = filepath.Join(inst.config.OrchestrationDirectory, actionName)

	if action.actionType == ACTION_TYPE_SH {
		var envVars map[string]string
		if envVars, err = inst.getEnvVars(actionName, context); err != nil {
			return exists, nil, "", "", err
		}

		if pluginsInfo, err = inst.readShAction(context, action, workingDir, orchestrationDir, envVars); err != nil {
			return exists, nil, "", "", err
		}

		return exists, pluginsInfo, workingDir, orchestrationDir, nil
	} else if action.actionType == ACTION_TYPE_PS1 {
		var envVars map[string]string
		if envVars, err = inst.getEnvVars(actionName, context); err != nil {
			return exists, nil, "", "", err
		}

		if pluginsInfo, err = inst.readPs1Action(context, action, workingDir, orchestrationDir, envVars); err != nil {
			return exists, nil, "", "", err
		}

		return exists, pluginsInfo, workingDir, orchestrationDir, nil
	} else {
		return exists, nil, "", "", fmt.Errorf("Internal error. Unknown actionType %v", action.actionType)
	}
}

// executeDocument executes a command document as a sub-document of the current command and returns the result
func (inst *Installer) executeDocument(
	tracer trace.Tracer,
	context context.T,
	actionName string,
	orchestrationDir string,
	pluginsInfo []contracts.PluginState,
	output contracts.PluginOutputter) {

	exectrace := tracer.CurrentTrace()

	pluginOutputs := inst.execdep.ExecuteDocument(context, pluginsInfo, inst.config.BookKeepingFileName, times.ToIso8601UTC(time.Now()), orchestrationDir)
	if pluginOutputs == nil {
		exectrace.WithError(fmt.Errorf("No output from executing %s document", actionName))
		output.MarkAsFailed(nil, nil)
		return
	}

	for _, pluginOut := range pluginOutputs {
		exectrace.WithExitcode(int64(pluginOut.Code))
		exectrace.AppendInfof("Plugin %v ResultStatus %v", pluginOut.PluginName, pluginOut.Status)
		if pluginOut.StandardOutput != "" {
			exectrace.AppendInfof("%v output: %v", actionName, pluginOut.StandardOutput)
		}
		if pluginOut.StandardError != "" {
			exectrace.AppendErrorf("%v errors: %v", actionName, pluginOut.StandardError)
		}
		if pluginOut.Error != "" {
			exectrace.WithError(errors.New(pluginOut.Error))
			output.MarkAsFailed(nil, nil)
		}
		output.SetStatus(contracts.MergeResultStatus(output.GetStatus(), pluginOut.Status))
	}
}
