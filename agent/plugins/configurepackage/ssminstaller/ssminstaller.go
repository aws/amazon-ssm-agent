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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docparser"
	"github.com/aws/amazon-ssm-agent/agent/executers"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/envdetect"
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
	ACTION_TYPE_JSON ActionType = iota
	ACTION_TYPE_SH   ActionType = iota
	ACTION_TYPE_PS1  ActionType = iota
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

func (inst *Installer) Install(context context.T) *contracts.PluginOutput {
	return inst.executeAction(context, "install")
}

func (inst *Installer) Uninstall(context context.T) *contracts.PluginOutput {
	return inst.executeAction(context, "uninstall")
}

func (inst *Installer) Validate(context context.T) *contracts.PluginOutput {
	return inst.executeAction(context, "validate")
}

func (inst *Installer) Version() string {
	return inst.version
}

func (inst *Installer) PackageName() string {
	return inst.packageName
}

func (inst *Installer) executeAction(context context.T, actionName string) *contracts.PluginOutput {
	log := context.Log()
	output := &contracts.PluginOutput{Status: contracts.ResultStatusSuccess}
	exists, pluginsInfo, _, err := inst.readAction(context, actionName)
	if exists {
		if err != nil {
			output.MarkAsFailed(log, err)
		}
		output.AppendInfof(log, "Initiating %v %v %v", inst.packageName, inst.version, actionName)
		inst.executeDocument(context, actionName, pluginsInfo, output)
	}
	return output
}

// getActionPath is a helper function that builds the path to an action document file
func (inst *Installer) getActionPath(actionName string, extension string) string {
	return filepath.Join(inst.packagePath, fmt.Sprintf("%v.%v", actionName, extension))
}

func (inst *Installer) readScriptAction(context context.T, action *Action, workingDir string, pluginName string, runCommand []interface{}) (pluginsInfo []contracts.PluginState, err error) {
	pluginsInfo = []contracts.PluginState{}

	pluginFullName := fmt.Sprintf("aws:%v", pluginName)
	var s3Prefix string
	if inst.config.OutputS3BucketName != "" {
		s3Prefix = fileutil.BuildS3Path(inst.config.OutputS3KeyPrefix, inst.config.PluginID, action.actionName, pluginFullName)
	}
	orchestrationDir := filepath.Join(inst.config.OrchestrationDirectory, action.actionName)

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
func (inst *Installer) readShAction(context context.T, action *Action, workingDir string, envVars map[string]string) (pluginsInfo []contracts.PluginState, err error) {
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

	return inst.readScriptAction(context, action, workingDir, "runShellScript", runCommand)
}

// readPs1Action turns an ps1 action into a set of SSM Document Plugins to execute
func (inst *Installer) readPs1Action(context context.T, action *Action, workingDir string, envVars map[string]string) (pluginsInfo []contracts.PluginState, err error) {
	if action.actionType != ACTION_TYPE_PS1 {
		return nil, fmt.Errorf("Internal error")
	}

	runCommand := []interface{}{}
	runCommand = append(runCommand, fmt.Sprintf("echo Running %v.ps1", action.actionName))

	for k, v := range envVars {
		v = executers.QuotePsString(v)
		runCommand = append(runCommand, fmt.Sprintf("$env:%v = %v", k, v))
	}

	runCommand = append(runCommand, fmt.Sprintf(".\\%v.ps1; exit $LASTEXITCODE", action.actionName))

	return inst.readScriptAction(context, action, workingDir, "runPowerShellScript", runCommand)
}

// readJsonAction turns an json action into a set of SSM Document Plugins to execute
func (inst *Installer) readJsonAction(context context.T, action *Action, workingDir string) (pluginsInfo []contracts.PluginState, err error) {
	if action.actionType != ACTION_TYPE_JSON {
		return nil, fmt.Errorf("Internal error")
	}

	log := context.Log()

	var actionContent []byte
	if actionContent, err = inst.filesysdep.ReadFile(action.filepath); err != nil {
		return nil, err
	}

	actionJson := string(actionContent[:])
	var jsonTest interface{}
	if err = jsonutil.Unmarshal(actionJson, &jsonTest); err != nil {
		return nil, err
	}

	var s3Prefix string
	if inst.config.OutputS3BucketName != "" {
		s3Prefix = fileutil.BuildS3Path(inst.config.OutputS3KeyPrefix, inst.config.PluginID, action.actionName)
	}
	orchestrationDir := filepath.Join(inst.config.OrchestrationDirectory, action.actionName)

	parserInfo := docparser.DocumentParserInfo{
		OrchestrationDir:  orchestrationDir,
		S3Bucket:          inst.config.OutputS3BucketName,
		S3Prefix:          s3Prefix,
		MessageId:         inst.config.MessageId,
		DocumentId:        inst.config.BookKeepingFileName,
		DefaultWorkingDir: workingDir,
	}

	var docContent contracts.DocumentContent
	err = json.Unmarshal(actionContent, &docContent)
	if err != nil {
		return nil, err
	}

	pluginsInfo, err = docparser.ParseDocument(log, &docContent, parserInfo, nil)

	if err != nil {
		return nil, err
	}
	if len(pluginsInfo) == 0 {
		return nil, fmt.Errorf("%v document contained no work and may be malformed", action.actionName)
	}
	return pluginsInfo, nil
}

func (inst *Installer) resolveAction(context context.T, actionName string) (exists bool, action *Action, err error) {
	log := context.Log()

	actionPathSh := inst.getActionPath(actionName, "sh")
	actionPathPs1 := inst.getActionPath(actionName, "ps1")
	actionPathJson := inst.getActionPath(actionName, "json")

	actionPathExistsSh := inst.filesysdep.Exists(actionPathSh)
	actionPathExistsPs1 := inst.filesysdep.Exists(actionPathPs1)
	actionPathExistsJson := inst.filesysdep.Exists(actionPathJson)
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
	if actionPathExistsJson {
		countExists += 1
		actionTemp.actionName = actionName
		actionTemp.actionType = ACTION_TYPE_JSON
		actionTemp.filepath = actionPathJson
	}

	if countExists > 1 {
		log.Debugf("%v has more than one implementation (sh, ps1, json)")
		return true, nil, fmt.Errorf("%v has more than one implementation (sh, ps1, json)", actionName)
	} else if countExists == 1 {
		return true, actionTemp, nil
	}

	return false, nil, nil
}

func (inst *Installer) getEnvVars(context context.T) (envVars map[string]string, err error) {
	log := context.Log()

	envVars = make(map[string]string)

	// Set proxy settings from the environment
	envVars["BWS_HTTPS_PROXY"] = os.Getenv("https_proxy")
	envVars["BWS_HTTP_PROXY"] = os.Getenv("http_proxy")
	envVars["BWS_NO_PROXY"] = os.Getenv("no_proxy")

	env, err := inst.envdetectCollector.CollectData(log)
	if err != nil {
		return envVars, fmt.Errorf("failed to collect data: %v", err)
	}

	// (Some of these are already available to script as AWS_SSM_INSTANCE_ID and AWS_SSM_REGION_NAME)
	envVars["BWS_PLATFORM_NAME"] = env.OperatingSystem.Platform
	envVars["BWS_PLATFORM_VERSION"] = env.OperatingSystem.PlatformVersion
	envVars["BWS_ARCHITECTURE"] = env.OperatingSystem.Architecture
	envVars["BWS_INSTANCE_ID"] = env.Ec2Infrastructure.InstanceID
	envVars["BWS_INSTANCE_TYPE"] = env.Ec2Infrastructure.InstanceType
	envVars["BWS_REGION"] = env.Ec2Infrastructure.Region
	envVars["BWS_AVAILABILITY_ZONE"] = env.Ec2Infrastructure.AvailabilityZone

	return envVars, err
}

// readAction returns a JSON document describing a management action and its working directory, or an empty string
// if there is nothing to do for a given action
func (inst *Installer) readAction(context context.T, actionName string) (exists bool, pluginsInfo []contracts.PluginState, workingDir string, err error) {
	// TODO: Split into linux and windows

	var action *Action

	if exists, action, err = inst.resolveAction(context, actionName); !exists || action == nil || err != nil {
		return exists, nil, "", err
	}

	workingDir = inst.packagePath

	if action.actionType == ACTION_TYPE_SH {
		var envVars map[string]string
		if envVars, err = inst.getEnvVars(context); err != nil {
			return exists, nil, "", err
		}

		if pluginsInfo, err = inst.readShAction(context, action, workingDir, envVars); err != nil {
			return exists, nil, "", err
		}

		return exists, pluginsInfo, workingDir, nil
	} else if action.actionType == ACTION_TYPE_PS1 {
		var envVars map[string]string
		if envVars, err = inst.getEnvVars(context); err != nil {
			return exists, nil, "", err
		}

		if pluginsInfo, err = inst.readPs1Action(context, action, workingDir, envVars); err != nil {
			return exists, nil, "", err
		}

		return exists, pluginsInfo, workingDir, nil
	} else if action.actionType == ACTION_TYPE_JSON {
		if pluginsInfo, err = inst.readJsonAction(context, action, workingDir); err != nil {
			return exists, nil, "", err
		}

		return exists, pluginsInfo, workingDir, nil
	} else {
		return exists, nil, "", fmt.Errorf("Internal error. Unknown actionType %v", action.actionType)
	}
}

// executeDocument executes a command document as a sub-document of the current command and returns the result
func (inst *Installer) executeDocument(context context.T,
	actionName string,
	pluginsInfo []contracts.PluginState,
	output *contracts.PluginOutput) {
	log := context.Log()

	pluginOutputs := inst.execdep.ExecuteDocument(context, pluginsInfo, inst.config.BookKeepingFileName, times.ToIso8601UTC(time.Now()))
	if pluginOutputs == nil {
		output.MarkAsFailed(log, errors.New("No output from executing install document (install.json)"))
	}
	for _, pluginOut := range pluginOutputs {
		log.Debugf("Plugin %v ResultStatus %v", pluginOut.PluginName, pluginOut.Status)
		if pluginOut.StandardOutput != "" {
			output.AppendInfof(log, "%v output: %v", actionName, pluginOut.StandardOutput)
		}
		if pluginOut.StandardError != "" {
			output.AppendErrorf(log, "%v errors: %v", actionName, pluginOut.StandardError)
		}
		if pluginOut.Error != nil {
			output.MarkAsFailed(log, pluginOut.Error)
		}
		output.Status = contracts.MergeResultStatus(output.Status, pluginOut.Status)
	}

	return
}
