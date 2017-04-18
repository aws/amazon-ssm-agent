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
	"fmt"
	"os"
	"path/filepath"
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
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/aws/aws-sdk-go/aws"
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

// NewPlugin returns a new instance of the plugin.
func NewPlugin(pluginConfig pluginutil.PluginConfig) (*Plugin, error) {
	var plugin Plugin
	plugin.MaxStdoutLength = pluginConfig.MaxStdoutLength
	plugin.MaxStderrLength = pluginConfig.MaxStderrLength
	plugin.StdoutFileName = pluginConfig.StdoutFileName
	plugin.StderrFileName = pluginConfig.StderrFileName
	plugin.OutputTruncatedSuffix = pluginConfig.OutputTruncatedSuffix
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
	if properties = pluginutil.LoadParametersAsList(log, config.Properties, &res); res.Code != 0 {
		return res
	}

	out := make([]contracts.PluginOutput, len(properties))
	for i, prop := range properties {

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
		out[i] = p.runCommandsRawInput(log, config.PluginID, prop, config.OrchestrationDirectory, cancelFlag, config.OutputS3BucketName, config.OutputS3KeyPrefix)
	}

	if len(properties) > 0 {
		res.Code = out[0].ExitCode
		res.Status = out[0].Status
		res.Output = out[0].String()
		res.StandardOutput = pluginutil.StringPrefix(out[0].Stdout, p.MaxStdoutLength, p.OutputTruncatedSuffix)
		res.StandardError = pluginutil.StringPrefix(out[0].Stderr, p.MaxStderrLength, p.OutputTruncatedSuffix)
	}

	pluginutil.PersistPluginInformationToCurrent(log, config.PluginID, config, res)
	return res
}

// runCommandsRawInput executes one set of commands and returns their output.
// The input is in the default json unmarshal format (e.g. map[string]interface{}).
func (p *Plugin) runCommandsRawInput(log log.T, pluginID string, rawPluginInput interface{}, orchestrationDirectory string, cancelFlag task.CancelFlag, outputS3BucketName string, outputS3KeyPrefix string) (out contracts.PluginOutput) {
	var pluginInput RefreshAssociationPluginInput
	err := jsonutil.Remarshal(rawPluginInput, &pluginInput)
	log.Debugf("Plugin input %v", pluginInput)
	if err != nil {
		errorString := fmt.Errorf("Invalid format in plugin properties %v;\nerror %v", rawPluginInput, err)
		out.MarkAsFailed(log, errorString)
		return
	}

	out = p.runCommands(log, pluginInput, orchestrationDirectory, cancelFlag, outputS3BucketName, outputS3KeyPrefix)

	// Set output status
	out.Status = pluginutil.GetStatus(out.ExitCode, cancelFlag)

	// Create output file paths
	stdoutFilePath := filepath.Join(orchestrationDirectory, p.StdoutFileName)
	stderrFilePath := filepath.Join(orchestrationDirectory, p.StderrFileName)
	log.Debugf("stdout file %v, stderr file %v", stdoutFilePath, stderrFilePath)

	// create orchestration dir if needed
	if err := fileutil.MakeDirs(orchestrationDirectory); err != nil {
		out.AppendError(log, "Failed to create orchestrationDir directory for log files")
	}
	if _, err = fileutil.WriteIntoFileWithPermissions(stdoutFilePath, out.Stdout, os.FileMode(int(appconfig.ReadWriteAccess))); err != nil {
		log.Error(err)
	}

	if _, err = fileutil.WriteIntoFileWithPermissions(stderrFilePath, out.Stderr, os.FileMode(int(appconfig.ReadWriteAccess))); err != nil {
		log.Error(err)
	}

	// Upload output to S3
	s3PluginID := pluginInput.ID
	if s3PluginID == "" {
		s3PluginID = pluginID
	}
	uploadOutputToS3BucketErrors := p.ExecuteUploadOutputToS3Bucket(log, s3PluginID, orchestrationDirectory, outputS3BucketName, outputS3KeyPrefix, false, "", out.Stdout, out.Stderr)
	if len(uploadOutputToS3BucketErrors) > 0 {
		log.Errorf("Unable to upload the logs: %s", uploadOutputToS3BucketErrors)
	}

	// Return Json indented response
	responseContent, _ := jsonutil.Marshal(out)
	log.Debug("Returning response:\n", jsonutil.Indent(responseContent))

	return out
}

// runCommands executes one the command and returns their output.
func (p *Plugin) runCommands(log log.T, pluginInput RefreshAssociationPluginInput, orchestrationDirectory string, cancelFlag task.CancelFlag, outputS3BucketName string, outputS3KeyPrefix string) (out contracts.PluginOutput) {
	var err error
	var instanceID string
	associations := []*model.InstanceAssociation{}

	if instanceID, err = platform.InstanceID(); err != nil {
		out.MarkAsFailed(log, fmt.Errorf("failed to load instance ID, %v", err))
		return
	}

	// Get associations
	if associations, err = p.assocSvc.ListInstanceAssociations(log, instanceID); err != nil {
		out.MarkAsFailed(log, fmt.Errorf("failed to list instance associations, %v", err))
		return
	}

	// evict the invalid cache first
	for _, assoc := range associations {
		cache.ValidateCache(assoc)
	}

	// if user provided empty list or "" in the document, we will run all the associations now
	applyAll := len(pluginInput.AssociationIds) == 0 || (len(pluginInput.AssociationIds) == 1 && pluginInput.AssociationIds[0] == "")

	// Default is success
	out.MarkAsSucceeded()

	// read from cache or load association details from service
	for _, assoc := range associations {
		if err = p.assocSvc.LoadAssociationDetail(log, assoc); err != nil {
			err = fmt.Errorf("Encountered error while loading association %v contents, %v",
				*assoc.Association.AssociationId,
				err)
			assoc.Errors = append(assoc.Errors, err)
			p.assocSvc.UpdateInstanceAssociationStatus(
				log,
				*assoc.Association.AssociationId,
				*assoc.Association.Name,
				*assoc.Association.InstanceId,
				contracts.AssociationStatusFailed,
				contracts.AssociationErrorCodeListAssociationError,
				times.ToIso8601UTC(time.Now()),
				err.Error(),
				service.NoOutputUrl)
			out.MarkAsFailed(log, err)
			continue
		}

		// validate association expression, fail association if expression cannot be passed
		// Note: we do not want to fail runcommand with out.MarkAsFailed
		if !assoc.IsRunOnceAssociation() {
			if err := assoc.ParseExpression(log); err != nil {
				message := fmt.Sprintf("Encountered error while parsing expression for association %v", *assoc.Association.AssociationId)
				log.Errorf("%v, %v", message, err)
				assoc.Errors = append(assoc.Errors, err)
				p.assocSvc.UpdateInstanceAssociationStatus(
					log,
					*assoc.Association.AssociationId,
					*assoc.Association.Name,
					*assoc.Association.InstanceId,
					contracts.AssociationStatusFailed,
					contracts.AssociationErrorCodeInvalidExpression,
					times.ToIso8601UTC(time.Now()),
					message,
					service.NoOutputUrl)
				out.MarkAsFailed(log, err)
				continue
			}
		}

		if applyAll || isAssociationQualifiedToRunNow(pluginInput.AssociationIds, assoc) {
			// If association is already InProgress, we don't want to run it again
			if assoc.Association.DetailedStatus == nil ||
				(*assoc.Association.DetailedStatus != contracts.AssociationStatusInProgress && *assoc.Association.DetailedStatus != contracts.AssociationStatusPending) {
				// Updates status to pending, which is the indicator for schedulemanager for immediate execution
				p.assocSvc.UpdateInstanceAssociationStatus(
					log,
					*assoc.Association.AssociationId,
					*assoc.Association.Name,
					*assoc.Association.InstanceId,
					contracts.AssociationStatusPending,
					contracts.AssociationErrorCodeNoError,
					times.ToIso8601UTC(time.Now()),
					contracts.AssociationPendingMessage,
					service.NoOutputUrl)
				assoc.Association.DetailedStatus = aws.String(contracts.AssociationStatusPending)
			}
		}
	}

	schedulemanager.Refresh(log, associations)

	if applyAll {
		out.AppendInfo(log, "All associations have been requested to execute immediately")
	} else {
		out.AppendInfof(log, "Associations %v have been requested to execute immediately", pluginInput.AssociationIds)
	}

	signal.ExecuteAssociation(log)

	return
}

// doesAssociationQualifiedToRunNow finds out if association is qualified to run now
func isAssociationQualifiedToRunNow(AssociationIds []string, assoc *model.InstanceAssociation) bool {
	for _, id := range AssociationIds {
		if *assoc.Association.AssociationId == id {
			return true
		}
	}

	return false
}
