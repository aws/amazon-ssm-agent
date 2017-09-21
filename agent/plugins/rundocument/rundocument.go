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
// permissions and limitations under the License..

// Package rundocument implements the aws:runDocument plugin
package rundocument

import (
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/filemanager"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/basicexecuter"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	ssmsvc "github.com/aws/amazon-ssm-agent/agent/ssm"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/aws/aws-sdk-go/service/ssm"

	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"strings"

	"github.com/go-yaml/yaml"
)

const (
	executeCommandMaxDepth = 3 //Maximum depth of document execution
	jsonExtension          = ".json"
	yamlExtension          = ".yaml"

	SSMDocumentType = "SSMDocument"
	LocalPathType   = "LocalPath"

	downloadsDir = "downloads" //Directory under the orchestration directory where the downloaded resource resides

	FailExitCode = 1
	PassExitCode = 0
)

// NewPlugin returns a new instance of the plugin.
func NewPlugin(pluginConfig pluginutil.PluginConfig) (*Plugin, error) {
	var plugin Plugin
	plugin.MaxStdoutLength = pluginConfig.MaxStdoutLength
	plugin.MaxStderrLength = pluginConfig.MaxStderrLength
	plugin.StdoutFileName = pluginConfig.StdoutFileName
	plugin.StderrFileName = pluginConfig.StderrFileName
	plugin.OutputTruncatedSuffix = pluginConfig.OutputTruncatedSuffix
	plugin.ExecuteUploadOutputToS3Bucket = pluginutil.UploadOutputToS3BucketExecuter(plugin.UploadOutputToS3Bucket)

	return &plugin, nil
}

// Plugin is the type for the aws:copyContent plugin.
type Plugin struct {
	pluginutil.DefaultPlugin
	filesys filemanager.FileSystem
	ssmSvc  ssmsvc.Service
	execDoc ExecDocument
}

// RunDocumentPluginInput is a struct that holds the parameters sent through send command
type RunDocumentPluginInput struct {
	contracts.PluginInput
	DocumentType       string `json:"docType"`
	DocumentPath       string `json:"docPath"`
	DocumentParameters string `json:"docParameters"`
}

// ExecutePluginDepth is the struct that is sent through to the sub-documents to maintain the depth of execution
type ExecutePluginDepth struct {
	executeCommandDepth int
}

// Execute runs multiple sets of commands and returns their outputs.
// res.Output will contain a slice of RunCommandPluginOutput.
func (p *Plugin) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag) (res contracts.PluginResult) {
	p.filesys = filemanager.FileSystemImpl{}
	p.ssmSvc = ssmsvc.NewService()
	exec := basicexecuter.NewBasicExecuter(context)
	p.execDoc = ExecDocumentImpl{
		DocExecutor: exec,
	}
	return p.execute(context, config, cancelFlag)
}

func (p *Plugin) execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag) (res contracts.PluginResult) {
	log := context.Log()
	log.Info("Plugin aws:runDocument started with configuration", config)

	res.StartDateTime = time.Now()
	defer func() { res.EndDateTime = time.Now() }()

	var output contracts.PluginOutput

	if cancelFlag.ShutDown() {
		res.Code = FailExitCode
		res.Status = contracts.ResultStatusFailed
		output.MarkAsShutdown()
	} else if cancelFlag.Canceled() {
		res.Code = FailExitCode
		res.Status = contracts.ResultStatusCancelled
		output.MarkAsCancelled()
	} else if input, err := parseAndValidateInput(config.Properties); err != nil {
		output.MarkAsFailed(log, err)
	} else {
		p.runDocument(context, input, config, &output)
	}

	if config.OrchestrationDirectory != "" {
		useTemp := false
		outFile := filepath.Join(config.OrchestrationDirectory, p.StdoutFileName)

		if err := p.filesys.MakeDirs(config.OrchestrationDirectory); err != nil {
			output.AppendError(log, "Failed to create orchestrationDir directory for log files")
		} else {
			if err := p.filesys.WriteFile(outFile, output.Stdout); err != nil {
				log.Debugf("Error writing to %v", outFile)
				output.AppendErrorf(log, "Error saving stdout: %v", err.Error())
			}
			errFile := filepath.Join(config.OrchestrationDirectory, p.StderrFileName)
			if err := p.filesys.WriteFile(errFile, output.Stderr); err != nil {
				log.Debugf("Error writing to %v", errFile)
				output.AppendErrorf(log, "Error saving stderr: %v", err.Error())
			}
		}

		uploadErrs := p.ExecuteUploadOutputToS3Bucket(log,
			config.PluginID,
			config.OrchestrationDirectory,
			config.OutputS3BucketName,
			config.OutputS3KeyPrefix,
			useTemp,
			config.OrchestrationDirectory,
			output.Stdout,
			output.Stderr)
		for _, uploadErr := range uploadErrs {
			output.AppendError(log, uploadErr)
		}
	}

	res.Code = output.ExitCode
	res.Status = output.Status
	res.Output = output.String()
	res.StandardOutput = pluginutil.StringPrefix(output.Stdout, p.MaxStdoutLength, p.OutputTruncatedSuffix)
	res.StandardError = pluginutil.StringPrefix(output.Stderr, p.MaxStderrLength, p.OutputTruncatedSuffix)

	return res
}

// runCopyContent figures out the type of location, downloads the resource, saves it on disk and returns information required for it
func (p *Plugin) runDocument(context context.T, input *RunDocumentPluginInput, config contracts.Configuration, output *contracts.PluginOutput) {

	log := context.Log()
	//Run aws:copyContent plugin
	log.Debug("Inside run copyContent function")
	var destinationDir string
	var pluginsInfo []model.PluginState
	var err error
	//Set the depth of execution to be 1 for the first level execution
	execDepth := 1
	// Getting the current depth of execution and checking against maximum depth
	if config.Settings != nil {
		if settings, ok := config.Settings.(*ExecutePluginDepth); !ok {
			log.Error("Plugin setting is not of the right type")
			output.MarkAsFailed(log, errors.New("There was an error obtaining the depth of execution"))
			return
		} else {
			execDepth = settings.executeCommandDepth + 1
			if execDepth > executeCommandMaxDepth {
				output.MarkAsFailed(log, fmt.Errorf("Maximum depth for document execution exceeded. "+
					"Maximum depth permitted - %v and current depth - %v", executeCommandMaxDepth, execDepth))
				return
			}
		}
	}
	log.Info("Depth of execution - ", execDepth)

	if input.DocumentType == SSMDocumentType {
		if destinationDir, err = p.downloadDocumentFromSSM(log, config, input); err != nil {
			output.MarkAsFailed(log, err)
		}
	} else {
		if filepath.IsAbs(input.DocumentPath) {
			destinationDir = input.DocumentPath
		} else {

			destinationDir = filepath.Join(config.OrchestrationDirectory, input.DocumentPath)
		}
	}
	if pluginsInfo, err = p.prepareDocumentForExecution(log, destinationDir, config, input.DocumentParameters); err != nil {
		output.MarkAsFailed(log, fmt.Errorf("There was an error while preparing documents - %v", err.Error()))
		return
	}
	// Sending execution depth in Configuration.Settings to the sub-documents
	for i, plugins := range pluginsInfo {
		plugins.Configuration.Settings = &ExecutePluginDepth{executeCommandDepth: execDepth}
		pluginsInfo[i] = plugins
	}
	p.execDoc.ExecuteDocument(context, pluginsInfo, config.BookKeepingFileName, times.ToIso8601UTC(time.Now()), output)
}

func parseDocumentNameForVersion(name string) (docName, docVersion string) {
	if len(name) == 0 {
		return "", ""
	}
	docNameArray := strings.Split(name, ":")
	docName = docNameArray[0]
	if len(docNameArray) > 1 {
		docVersion = docNameArray[1]
	}
	return docName, docVersion
}

func (p *Plugin) downloadDocumentFromSSM(log log.T, config contracts.Configuration, input *RunDocumentPluginInput) (string, error) {
	instanceId, err := platform.InstanceID()
	// Downloads folder for download path
	destination := filepath.Join(appconfig.DefaultDataStorePath, instanceId, appconfig.DefaultDocumentRootDirName, downloadsDir)

	//This gets the document name if the fullARN is provided
	docName := filepath.Base(input.DocumentPath)
	docName, docVersion := parseDocumentNameForVersion(docName)
	var docResponse *ssm.GetDocumentOutput
	if docResponse, err = p.ssmSvc.GetDocument(log, docName, docVersion); err != nil {
		log.Errorf("Unable to get ssm document. %v", err)
		return "", err
	}

	log.Debugf("Destination is %v ", destination)
	// create directory to download github resources
	if err = p.filesys.MakeDirs(filepath.Dir(destination)); err != nil {
		log.Error("failed to create directory for github - ", err)
		return "", err
	}

	pathToFile := filepath.Join(destination, docName+jsonExtension)

	if err = p.filesys.WriteFile(pathToFile, *docResponse.Content); err != nil {
		log.Errorf("Error writing to file %v - %v", pathToFile, err)
		return "", err
	}
	return pathToFile, nil

}

// PrepareDocumentForExecution parses the raw content of the document, validates it and returns a PluginState that can be executed.
func (p *Plugin) prepareDocumentForExecution(log log.T, pathToFile string, config contracts.Configuration, params string) (pluginsInfo []model.PluginState, err error) {
	parameters := make(map[string]interface{})
	if params != "" {

		log.Info("Params to be unmarshaled - ", params)
		// TODO: meloniam@ 08/24/2017 - https://amazon.awsapps.com/workdocs/index.html#/document/7d56a42ea5b040a7c33548d77dc98040f0fb380bbbfb2fd580c861225e2ee1c7
		// TODO: Remove the Unmarshalling once RC supports StringMap
		// TODO: documentParameters will be of type map[string]interface{} from the beginning
		if filepath.Ext(pathToFile) == jsonExtension {
			if json.Unmarshal([]byte(params), &parameters); err != nil {
				log.Error("Unmarshalling JSON remote resource parameters failed. Please make sure the document is in the right format")
				return pluginsInfo, err
			}
		} else if filepath.Ext(pathToFile) == yamlExtension {
			if yaml.Unmarshal([]byte(params), &parameters); err != nil {
				log.Error("Unmarshalling YAML remote resource parameters failed. Please make sure the document is in the right format")
				return pluginsInfo, err
			}
		} else {
			return pluginsInfo, errors.New("Extension type for documents is not supported")
		}

		log.Info("Parameters passed in are ", parameters)
	}
	var rawDocument []byte
	if rawDocument, err = readFileContents(log, p.filesys, pathToFile); err != nil {
		log.Error("Could not read document from remote resource - ", err)
		return nil, err
	}
	log.Infof("Sending the document received for parsing - %v", string(rawDocument))

	return p.execDoc.ParseDocument(log, filepath.Ext(pathToFile), rawDocument, config.OrchestrationDirectory, config.OutputS3BucketName, config.OutputS3KeyPrefix, config.MessageId, config.PluginID, config.DefaultWorkingDirectory, parameters)
}

// Name returns the plugin name
func Name() string {
	return appconfig.PluginRunDocument
}

// parseAndValidateInput parses the input json file and also validates its inputs
func parseAndValidateInput(rawPluginInput interface{}) (*RunDocumentPluginInput, error) {
	var input RunDocumentPluginInput
	var err error
	if err = jsonutil.Remarshal(rawPluginInput, &input); err != nil {
		return nil, fmt.Errorf("invalid format in plugin properties %v; \nerror %v", rawPluginInput, err)
	}

	if valid, err := validateInput(&input); !valid {
		return nil, fmt.Errorf("invalid input: %v", err)
	}

	return &input, nil
}

// validateInput ensures the plugin input matches the defined schema
func validateInput(input *RunDocumentPluginInput) (valid bool, err error) {
	// ensure non-empty location type
	if input.DocumentType == "" {
		return false, errors.New("Source location must be specified to either by SSMDocument or LocalPath.")
	}
	if input.DocumentType != SSMDocumentType && input.DocumentType != LocalPathType {
		return false, errors.New("Source type specified in invalid")
	}
	if input.DocumentPath == "" {
		return false, errors.New("Souce Info must be provided")
	}
	return true, nil
}

// readFileContents is a method to read the contents of a give file path
func readFileContents(log log.T, filesysdep filemanager.FileSystem, destinationPath string) (fileContent []byte, err error) {

	log.Debug("Reading file contents from file - ", destinationPath)

	var rawFile string
	if rawFile, err = filesysdep.ReadFile(destinationPath); err != nil {
		log.Error("Error occured while reading file - ", err)
		return nil, err
	}
	if rawFile == "" {
		return []byte(rawFile), errors.New("File is empty!")
	}

	return []byte(rawFile), nil
}

var instance instanceInfo = &instanceInfoImp{}

// system represents the dependency for platform
type instanceInfo interface {
	InstanceID() (string, error)
}

type instanceInfoImp struct{}

// InstanceID wraps platform InstanceID
func (instanceInfoImp) InstanceID() (string, error) { return platform.InstanceID() }
