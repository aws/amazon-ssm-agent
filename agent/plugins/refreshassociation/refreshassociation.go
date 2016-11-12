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

// Package refreshassociation implements the refreshassociation plugin.
package refreshassociation

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/association/cache"
	"github.com/aws/amazon-ssm-agent/agent/association/model"
	"github.com/aws/amazon-ssm-agent/agent/association/schedulemanager"
	"github.com/aws/amazon-ssm-agent/agent/association/schedulemanager/signal"
	"github.com/aws/amazon-ssm-agent/agent/association/service"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/rebooter"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/agent/times"
)

// Plugin is the type for the refreshassociation plugin.
type Plugin struct {
	pluginutil.DefaultPlugin
	assocSvc *service.AssociationService
}

// RefreshAssociationPluginInput represents one set of commands executed by the refreshassociation plugin.
type RefreshAssociationPluginInput struct {
	contracts.PluginInput
	ID             string
	AssociationIds []string
}

// RefreshAssociationPluginOutput represents the output of the plugin
type RefreshAssociationPluginOutput struct {
	contracts.PluginOutput
}

// MarkAsFailed marks plugin as Failed
func (out *RefreshAssociationPluginOutput) MarkAsFailed(log log.T, err error) {
	out.ExitCode = 1
	out.Status = contracts.ResultStatusFailed
	if len(out.Stderr) != 0 {
		out.Stderr = fmt.Sprintf("\n%v\n%v", out.Stderr, err.Error())
	} else {
		out.Stderr = fmt.Sprintf("\n%v", err.Error())
	}
	log.Error(err.Error())
	out.Errors = append(out.Errors, err.Error())
}

// NewPlugin returns a new instance of the plugin.
func NewPlugin(pluginConfig pluginutil.PluginConfig) (*Plugin, error) {
	var plugin Plugin
	plugin.MaxStdoutLength = pluginConfig.MaxStdoutLength
	plugin.MaxStderrLength = pluginConfig.MaxStderrLength
	plugin.StdoutFileName = pluginConfig.StdoutFileName
	plugin.StderrFileName = pluginConfig.StderrFileName
	plugin.OutputTruncatedSuffix = pluginConfig.OutputTruncatedSuffix
	plugin.Uploader = pluginutil.GetS3Config()
	plugin.ExecuteUploadOutputToS3Bucket = pluginutil.UploadOutputToS3BucketExecuter(plugin.UploadOutputToS3Bucket)

	plugin.assocSvc = service.NewAssociationService(Name())

	return &plugin, nil
}

// Name returns the name of the plugin
func Name() string {
	return appconfig.PluginNameRefreshAssociation
}

// Execute runs multiple sets of commands and returns their outputs.
// res.Output will contain a slice of PluginOutput.
func (p *Plugin) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag, subDocumentRunner runpluginutil.PluginRunner) (res contracts.PluginResult) {
	log := context.Log()
	log.Infof("%v started with configuration %v", Name(), config)
	res.StartDateTime = time.Now()
	defer func() { res.EndDateTime = time.Now() }()

	p.assocSvc.CreateNewServiceIfUnHealthy(log)
	//loading Properties as list since aws:updateSsmAgent uses properties as list
	var properties []interface{}
	if properties, res = pluginutil.LoadParametersAsList(log, config.Properties); res.Code != 0 {
		return res
	}

	out := make([]RefreshAssociationPluginOutput, len(properties))
	for i, prop := range properties {

		// check if a reboot has been requested
		if rebooter.RebootRequested() {
			log.Infof("Stopping execution of %v plugin due to an external reboot request.", Name())
			return
		}

		if cancelFlag.ShutDown() {
			res.Code = 1
			res.Status = contracts.ResultStatusFailed
			pluginutil.PersistPluginInformationToCurrent(log, config.PluginID, config, res)
			return
		}

		if cancelFlag.Canceled() {
			res.Code = 1
			res.Status = contracts.ResultStatusCancelled
			pluginutil.PersistPluginInformationToCurrent(log, config.PluginID, config, res)
			return
		}
		out[i] = p.runCommandsRawInput(log, prop, config.OrchestrationDirectory, cancelFlag, config.OutputS3BucketName, config.OutputS3KeyPrefix)
	}

	if len(properties) > 0 {
		res.Code = out[0].ExitCode
		res.Status = out[0].Status
		res.Output = out[0].String()
		res.StandardOutput = contracts.TruncateOutput(out[0].Stdout, "", 24000)
		res.StandardError = contracts.TruncateOutput(out[0].Stderr, "", 8000)
	}

	pluginutil.PersistPluginInformationToCurrent(log, config.PluginID, config, res)
	return res
}

// runCommandsRawInput executes one set of commands and returns their output.
// The input is in the default json unmarshal format (e.g. map[string]interface{}).
func (p *Plugin) runCommandsRawInput(log log.T, rawPluginInput interface{}, orchestrationDirectory string, cancelFlag task.CancelFlag, outputS3BucketName string, outputS3KeyPrefix string) (out RefreshAssociationPluginOutput) {
	var pluginInput RefreshAssociationPluginInput
	err := jsonutil.Remarshal(rawPluginInput, &pluginInput)
	log.Debugf("Plugin input %v", pluginInput)
	if err != nil {
		errorString := fmt.Errorf("Invalid format in plugin properties %v;\nerror %v", rawPluginInput, err)
		out.MarkAsFailed(log, errorString)
		return
	}
	return p.runCommands(log, pluginInput, orchestrationDirectory, cancelFlag, outputS3BucketName, outputS3KeyPrefix)
}

// runCommands executes one the command and returns their output.
func (p *Plugin) runCommands(log log.T, pluginInput RefreshAssociationPluginInput, orchestrationDirectory string, cancelFlag task.CancelFlag, outputS3BucketName string, outputS3KeyPrefix string) (out RefreshAssociationPluginOutput) {
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

	orchestrationDir := fileutil.BuildPath(orchestrationDirectory, pluginInput.ID)
	log.Debugf("OrchestrationDir %v ", orchestrationDir)

	// create orchestration dir if needed
	if err = fileutil.MakeDirs(orchestrationDir); err != nil {
		log.Debug("failed to create orchestrationDir directory", orchestrationDir, err)
		out.Errors = append(out.Errors, err.Error())
		return
	}

	var instanceID string
	var associations []*model.InstanceAssociation

	if instanceID, err = platform.InstanceID(); err != nil {
		log.Debug("failed to load instance ID ", err)
		out.Errors = append(out.Errors, err.Error())
		return
	}

	// Get associations
	if associations, err = p.assocSvc.ListInstanceAssociations(log, instanceID); err != nil {
		log.Debug("failed to list instance associations ", err)
		out.Errors = append(out.Errors, err.Error())
	}

	// update the cache first
	for _, assoc := range associations {
		cache.ValidateCache(assoc)
	}

	// read from cache or load association details from service
	for _, assoc := range associations {
		if err = p.assocSvc.LoadAssociationDetail(log, assoc); err != nil {
			message := fmt.Sprintf("unable to load association details, %v", err)
			log.Error(message)
			p.assocSvc.UpdateInstanceAssociationStatus(
				log,
				*assoc.Association.AssociationId,
				*assoc.Association.Name,
				*assoc.Association.InstanceId,
				contracts.AssociationStatusFailed,
				contracts.AssociationErrorCodeListAssociationError,
				times.ToIso8601UTC(time.Now()),
				message)
			out.Errors = append(out.Errors, err.Error())
			return
		}

		// if user provided empty associationIds in the document, we will run all the associations now
		if len(pluginInput.AssociationIds) == 0 {
			assoc.RunNow = true
		} else {
			for _, id := range pluginInput.AssociationIds {
				if *assoc.Association.AssociationId == id {
					assoc.RunNow = true
					break
				}
			}
		}
	}

	schedulemanager.Refresh(log, associations, p.assocSvc)
	stdout := fmt.Sprintf("Associations %v have been requested to execute immediately", pluginInput.AssociationIds)
	log.Info(stdout)
	signal.ExecuteAssociation(log)

	// Set output status
	out.ExitCode = 0
	out.Status = pluginutil.GetStatus(out.ExitCode, cancelFlag)

	// read (a prefix of) the standard output/error
	out.Stdout, err = pluginutil.ReadPrefix(bytes.NewBufferString(stdout), p.MaxStdoutLength, p.OutputTruncatedSuffix)
	if err != nil {
		out.Errors = append(out.Errors, err.Error())
		log.Error(err)
	}

	// Upload output to S3
	uploadOutputToS3BucketErrors := p.ExecuteUploadOutputToS3Bucket(log, pluginInput.ID, orchestrationDir, outputS3BucketName, outputS3KeyPrefix, useTempDirectory, tempDir, out.Stdout, out.Stderr)
	out.Errors = append(out.Errors, uploadOutputToS3BucketErrors...)

	// Return Json indented response
	responseContent, _ := jsonutil.Marshal(out)
	log.Debug("Returning response:\n", jsonutil.Indent(responseContent))
	return
}
