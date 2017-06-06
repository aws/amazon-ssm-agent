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
	"path/filepath"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/times"
)

type Installer struct {
	filesysdep  fileSysDep
	execdep     execDep
	packageName string
	version     string
	packagePath string
	config      contracts.Configuration
	runner      runpluginutil.PluginRunner
}

func New(packageName string,
	version string,
	packagePath string,
	configuration contracts.Configuration,
	runner runpluginutil.PluginRunner) *Installer {
	return &Installer{
		filesysdep:  &fileSysDepImp{},
		execdep:     &execDepImp{},
		packageName: packageName,
		version:     version,
		packagePath: packagePath,
		config:      configuration,
		runner:      runner,
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

func (inst *Installer) executeAction(context context.T, actionName string) *contracts.PluginOutput {
	log := context.Log()
	output := &contracts.PluginOutput{Status: contracts.ResultStatusSuccess}
	exists, actionContent, workingDir, err := inst.readAction(context, actionName)
	if exists {
		if err != nil {
			output.MarkAsFailed(log, err)
		}
		if pluginsInfo, err := inst.parseAction(context, actionName, actionContent, workingDir); err != nil {
			output.MarkAsFailed(log, err)
		} else {
			output.AppendInfof(log, "Initiating %v %v %v", inst.packageName, inst.version, actionName)
			inst.executeDocument(context, actionName, pluginsInfo, output)
		}
	}
	return output
}

// getActionPath is a helper function that builds the path to an action document file
func (inst *Installer) getActionPath(actionName string) string {
	return filepath.Join(inst.packagePath, fmt.Sprintf("%v.json", actionName))
}

// readAction returns a JSON document describing a management action and its working directory, or an empty string
// if there is nothing to do for a given action
func (inst *Installer) readAction(context context.T, actionName string) (exists bool, actionDocument []byte, workingDir string, err error) {
	actionPath := inst.getActionPath(actionName)
	if !inst.filesysdep.Exists(actionPath) {
		return false, []byte{}, "", nil
	}
	if actionContent, err := inst.filesysdep.ReadFile(actionPath); err != nil {
		return true, []byte{}, "", err
	} else {
		actionJson := string(actionContent[:])
		var jsonTest interface{}
		if err = jsonutil.Unmarshal(actionJson, &jsonTest); err != nil {
			return true, []byte{}, "", err
		}
		return true, actionContent, inst.packagePath, nil
	}
}

// parseAction turns an action into a set of SSM Document Plugins to execute
func (inst *Installer) parseAction(context context.T,
	actionName string,
	actionContent []byte,
	executeDirectory string) (pluginsInfo []model.PluginState, err error) {
	if err != nil {
		return nil, err
	}
	var s3Prefix string
	if inst.config.OutputS3BucketName != "" {
		s3Prefix = fileutil.BuildS3Path(inst.config.OutputS3KeyPrefix, actionName)
	}
	pluginsInfo, err = inst.execdep.ParseDocument(
		context,
		actionContent,
		inst.config.OrchestrationDirectory,
		inst.config.OutputS3BucketName,
		s3Prefix,
		inst.config.MessageId,
		inst.config.BookKeepingFileName,
		executeDirectory)
	if err != nil {
		return nil, err
	}
	if len(pluginsInfo) == 0 {
		return nil, fmt.Errorf("%v document contained no work and may be malformed", actionName)
	}
	return pluginsInfo, nil
}

// executeDocument executes a command document as a sub-document of the current command and returns the result
func (inst *Installer) executeDocument(context context.T,
	actionName string,
	pluginsInfo []model.PluginState,
	output *contracts.PluginOutput) {
	log := context.Log()

	pluginOutputs := inst.execdep.ExecuteDocument(inst.runner, context, pluginsInfo, inst.config.BookKeepingFileName, times.ToIso8601UTC(time.Now()))
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
