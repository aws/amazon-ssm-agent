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

// Package inventory contains implementation of aws:softwareInventory plugin
package inventory

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	associateModel "github.com/aws/amazon-ssm-agent/agent/association/model"
	"github.com/aws/amazon-ssm-agent/agent/association/schedulemanager"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	docmanagerModel "github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/datauploader"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/application"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/awscomponent"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/custom"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/instancedetailedinformation"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/network"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/windowsUpdate"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ssm"
)

//TODO: add more unit tests.

const (
	errorMsgForMultipleAssociations           = "%v detected multiple inventory configurations associated to one instance. You can’t associate multiple inventory configurations to an instance. The association IDs are: %v and %v."
	errorMsgForInvalidInventoryInput          = "Unrecongnized input for %v plugin"
	errorMsgForExecutingInventoryViaAssociate = "%v plugin can only be invoked via ssm-associate"
	errorMsgForUnableToDetectInvocationType   = "Unable to detect if %v plugin was invoked via ssm-associate because - %v"
	errorMsgForInabilityToSendDataToSSM       = "Unable to upload inventory data to SSM"
	msgWhenNoDataToReturnForInventoryPlugin   = "Inventory policy has been successfully applied but there is no inventory data to upload to SSM"
	successfulMsgForInventoryPlugin           = "Inventory policy has been successfully applied and collected inventory data has been uploaded to SSM"
)

var (
	//associationLock ensures that all read & write for lastExecutedAssociations & currentAssociations is thread-safe
	associationLock sync.RWMutex
)

// PluginInput represents configuration which is applied to inventory plugin during execution.
type PluginInput struct {
	contracts.PluginInput
	Applications                string
	AWSComponents               string
	NetworkConfig               string
	WindowsUpdates              string
	InstanceDetailedInformation string
	CustomInventory             string
	CustomInventoryDirectory    string
}

// decoupling schedulemanager.Schedules() for easy testability
var associationsProvider = getCurrentAssociations

func getCurrentAssociations() []*associateModel.InstanceAssociation {
	return schedulemanager.Schedules()
}

// decoupling platform.InstanceID for easy testability
var machineIDProvider = machineInfoProvider

func machineInfoProvider() (name string, err error) {
	return platform.InstanceID()
}

// Plugin encapsulates the logic of configuring, starting and stopping inventory plugin
type Plugin struct {
	pluginutil.DefaultPlugin

	context    context.T
	stopPolicy *sdkutil.StopPolicy

	//supportedGatherers is a map of all inventory gatherers supported by current OS
	// (e.g. WindowsUpdateGatherer is not included when running on Unix based systems)
	supportedGatherers gatherers.SupportedGatherer

	//installedGatherers is a map of gatherers that can run on all platforms
	installedGatherers gatherers.InstalledGatherer

	//uploader handles uploading inventory data to SSM.
	uploader datauploader.T

	//lastExecutedAssociations stores a map of associations & its execution time for inventory
	lastExecutedAssociations map[string]string

	// currentAssociations stores a copy of all current associations to instance. It's refreshed everytime inventory
	// is invoked via association
	currentAssociations map[string]string

	// machineID of the machine where agent is running - useful during command detection
	machineID string
}

// Name returns the plugin name
func Name() string {
	return appconfig.PluginNameAwsSoftwareInventory
}

// NewPlugin creates a new inventory worker plugin.
func NewPlugin(context context.T, pluginConfig pluginutil.PluginConfig) (*Plugin, error) {
	var err error
	var p = Plugin{}

	//setting up default worker plugin config
	p.MaxStdoutLength = pluginConfig.MaxStdoutLength
	p.MaxStderrLength = pluginConfig.MaxStderrLength
	p.StdoutFileName = pluginConfig.StdoutFileName
	p.StderrFileName = pluginConfig.StderrFileName
	p.OutputTruncatedSuffix = pluginConfig.OutputTruncatedSuffix
	p.ExecuteUploadOutputToS3Bucket = pluginutil.UploadOutputToS3BucketExecuter(p.UploadOutputToS3Bucket)

	// since this is initialization - lastExecutedAssociations should be empty.
	// NOTE: this will get populated when inventory plugin gets invoked via association later.
	p.lastExecutedAssociations = make(map[string]string)

	c := context.With("[" + Name() + "]")
	log := c.Log()

	//get machineID - return if not able to detect machineID
	if p.machineID, err = machineIDProvider(); err != nil {
		err = fmt.Errorf("Unable to detect machineID because of %v - this will hamper execution of inventory plugin",
			err.Error())
		return &p, err
	}

	p.context = c
	p.stopPolicy = sdkutil.NewStopPolicy(Name(), model.ErrorThreshold)

	//loads all registered gatherers (for now only a dummy application gatherer is loaded in memory)
	p.supportedGatherers, p.installedGatherers = gatherers.InitializeGatherers(p.context)
	//initializes SSM Inventory uploader
	if p.uploader, err = datauploader.NewInventoryUploader(c); err != nil {
		err = log.Errorf("Unable to configure SSM Inventory uploader - %v", err.Error())
	}

	return &p, err
}

// ApplyInventoryPolicy applies given inventory policy regarding which gatherers to run
func (p *Plugin) ApplyInventoryPolicy(context context.T, inventoryInput PluginInput) (inventoryOutput contracts.PluginOutput) {
	log := p.context.Log()
	var optimizedInventoryItems, nonOptimizedInventoryItems []*ssm.InventoryItem
	var status, retryWithNonOptimized bool
	var items []model.Item
	var err error

	//map of all valid gatherers & respective configs to run
	var gatherers map[gatherers.T]model.Config

	//validate all gatherers
	if gatherers, err = p.ValidateInventoryInput(context, inventoryInput); err != nil {
		log.Info(err.Error())
		inventoryOutput.ExitCode = 1
		inventoryOutput.Stderr = err.Error()
		return
	}

	//execute all eligible gatherers with their respective config
	if items, err = p.RunGatherers(gatherers); err != nil {
		log.Info(err.Error())
		inventoryOutput.ExitCode = 1
		inventoryOutput.Stderr = err.Error()
		return
	}

	//check if there is data to send to SSM
	if len(items) == 0 {
		//no data to send to ssm - no need to call PutInventory API
		log.Info(msgWhenNoDataToReturnForInventoryPlugin)
		inventoryOutput.ExitCode = 0
		inventoryOutput.Stdout = msgWhenNoDataToReturnForInventoryPlugin
		return
	}

	//log collected data before sending
	d, _ := json.Marshal(items)
	log.Debugf("Collected Inventory data: %v", string(d))

	if optimizedInventoryItems, nonOptimizedInventoryItems, err = p.uploader.ConvertToSsmInventoryItems(p.context, items); err != nil {
		log.Infof("Encountered error in converting data to SSM InventoryItems - %v. Skipping upload to SSM", err.Error())
		inventoryOutput.ExitCode = 1
		inventoryOutput.Stderr = err.Error()
		return
	}

	log.Debugf("Optimized data - \n%v \n Non-optimized data - \n%v",
		optimizedInventoryItems,
		nonOptimizedInventoryItems)

	//first send data in optimized fashion
	if status, retryWithNonOptimized = p.SendDataToInventory(context, optimizedInventoryItems); !status {

		if retryWithNonOptimized {
			//call putinventory again with non-optimized dataset
			if status, _ = p.SendDataToInventory(context, nonOptimizedInventoryItems); !status {
				//sending non-optimized data also failed
				log.Info(errorMsgForInabilityToSendDataToSSM)
				inventoryOutput.ExitCode = 1
				inventoryOutput.Stderr = errorMsgForInabilityToSendDataToSSM
				return
			}
		} else {
			//some other error happened for which there is no need to retry - upload failed
			log.Info(errorMsgForInabilityToSendDataToSSM)
			inventoryOutput.ExitCode = 1
			inventoryOutput.Stderr = errorMsgForInabilityToSendDataToSSM
			return
		}
	}

	log.Infof("%v uploaded inventory data to SSM", Name())
	inventoryOutput.ExitCode = 0
	inventoryOutput.Stdout = successfulMsgForInventoryPlugin

	return
}

// SendDataToInventory sends data to SSM and returns if data was sent successfully or not. If data is not uploaded successfully,
// it parses the error message and determines if it should be sent again.
func (p *Plugin) SendDataToInventory(context context.T, items []*ssm.InventoryItem) (status, retryWithFullData bool) {
	var err error
	log := context.Log()

	if err = p.uploader.SendDataToSSM(context, items); err != nil {
		status = false
		if aerr, ok := err.(awserr.Error); ok {
			//NOTE: awserr.Code -> is not the error code but the exception name itself!!!!
			if aerr.Code() == "ItemContentMismatchException" || aerr.Code() == "InvalidItemContentException" {
				log.Debugf("%v encountered - inventory plugin will try sending data again", aerr.Code())
				retryWithFullData = true
			} else {
				log.Debugf("Unexpected error encountered - %v. No point trying to send data again", aerr.Code())
			}
		}
	} else {
		status = true
	}

	return
}

// CanGathererRun returns true if the gatherer can run on given OS, else it returns false. It throws error if the
// gatherer is not recognized.
func (p *Plugin) CanGathererRun(context context.T, name string) (status bool, gatherer gatherers.T, err error) {

	log := context.Log()
	var isGathererSupported, isGathererInstalled bool

	if gatherer, isGathererSupported = p.supportedGatherers[name]; !isGathererSupported {
		if _, isGathererInstalled = p.installedGatherers[name]; isGathererInstalled {
			log.Infof("%v inventory gatherer is installed but not supported to run on this platform", name)
			status = false
		} else {
			err = fmt.Errorf("Unrecognized inventory gatherer - %v ", name)
		}
	} else {
		log.Infof("%v inventory gatherer is supported to run on this platform", name)
		status = true
	}

	return
}

func (p *Plugin) validatePredefinedGatherer(context context.T, collectionPolicy, gathererName string) (status bool, gatherer gatherers.T, policy model.Config, err error) {

	if collectionPolicy == model.Enabled {
		if status, gatherer, err = p.CanGathererRun(context, gathererName); err != nil {
			return
		}

		// check if gatherer can run - if not then no need to set policy
		if status {
			policy = model.Config{Collection: collectionPolicy}
		}
	}

	return
}

func (p *Plugin) validateCustomGatherer(context context.T, collectionPolicy, location string) (status bool, gatherer gatherers.T, policy model.Config, err error) {

	if collectionPolicy == model.Enabled {
		if status, gatherer, err = p.CanGathererRun(context, custom.GathererName); err != nil {
			return
		}

		// check if gatherer can run - if not then no need to set policy
		if status {
			policy = model.Config{Collection: collectionPolicy, Location: location}
		}
	}

	return
}

// ValidateInventoryInput validates inventory input and returns a map of eligible gatherers & their corresponding config.
// It throws an error if gatherer is not recognized/installed.
func (p *Plugin) ValidateInventoryInput(context context.T, input PluginInput) (configuredGatherers map[gatherers.T]model.Config, err error) {
	var dataB []byte
	var canGathererRun bool
	var gatherer gatherers.T
	var cfg model.Config
	configuredGatherers = make(map[gatherers.T]model.Config)

	log := context.Log()
	dataB, _ = json.Marshal(input)
	log.Debugf("Validating gatherers from inventory input - \n%v", jsonutil.Indent(string(dataB)))

	//NOTE:
	// If the gatherer is installed but not supported by current platform, we will skip that gatherer. If the
	// gatherer is not installed,  we error out & don't send the data collected from other supported gatherers
	// - this is because we don't send partial inventory data as part of 1 inventory policy.

	//checking application gatherer
	if canGathererRun, gatherer, cfg, err = p.validatePredefinedGatherer(context, input.Applications, application.GathererName); err != nil {
		return
	} else if canGathererRun {
		configuredGatherers[gatherer] = cfg
	}

	//checking awscomponents gatherer
	if canGathererRun, gatherer, cfg, err = p.validatePredefinedGatherer(context, input.AWSComponents, awscomponent.GathererName); err != nil {
		return
	} else if canGathererRun {
		configuredGatherers[gatherer] = cfg
	}

	//checking network gatherer
	if canGathererRun, gatherer, cfg, err = p.validatePredefinedGatherer(context, input.NetworkConfig, network.GathererName); err != nil {
		return
	} else if canGathererRun {
		configuredGatherers[gatherer] = cfg
	}

	//checking windows updates gatherer
	if canGathererRun, gatherer, cfg, err = p.validatePredefinedGatherer(context, input.WindowsUpdates, windowsUpdate.GathererName); err != nil {
		return
	} else if canGathererRun {
		configuredGatherers[gatherer] = cfg
	}

	//checking instance detailed information gatherer
	if canGathererRun, gatherer, cfg, err = p.validatePredefinedGatherer(context, input.InstanceDetailedInformation, instancedetailedinformation.GathererName); err != nil {
		return
	} else if canGathererRun {
		configuredGatherers[gatherer] = cfg
	}

	//checking custom gatherer
	if canGathererRun, gatherer, cfg, err = p.validateCustomGatherer(context, input.CustomInventory, input.CustomInventoryDirectory); err != nil {
		return
	} else if canGathererRun {
		configuredGatherers[gatherer] = cfg
	}

	return
}

// RunGatherers execute given array of gatherers and accordingly returns. It returns error if gatherer is not
// registered or if at any stage the data returned breaches size limit
func (p *Plugin) RunGatherers(gatherers map[gatherers.T]model.Config) (items []model.Item, err error) {

	//NOTE: Currently all gatherers will be invoked in synchronous & sequential fashion.
	//Parallel execution of gatherers hinges upon inventory plugin becoming a long running plugin - which will be
	//mainly for custom inventory gatherer to send data independently of associate.

	var gItems []model.Item

	log := p.context.Log()

	for gatherer, config := range gatherers {
		name := gatherer.Name()
		log.Infof("Invoking gatherer - %v", name)

		if gItems, err = gatherer.Run(p.context, config); err != nil {
			err = fmt.Errorf("Encountered error while executing %v. Error - %v", name, err.Error())
			break

		} else {
			items = append(items, gItems...)

			//TODO: Each gather shall check each item's size and stop collecting if size exceed immediately
			//TODO: only check the total item size at this function, whenever total size exceed, stop
			//TODO: immediately and raise association error
			//return error if collected data breaches size limit
			for _, v := range gItems {
				if !p.VerifyInventoryDataSize(v, items) {
					err = log.Errorf("Size limit exceeded for collected data.")
					break
				}
			}
		}
	}

	return
}

// VerifyInventoryDataSize returns true if size of collected inventory data is within size restrictions placed by SSM,
// else false.
func (p *Plugin) VerifyInventoryDataSize(item model.Item, items []model.Item) bool {
	var itemSize, itemsSize float32
	log := p.context.Log()

	//calculating sizes
	itemB, _ := json.Marshal(item)
	itemSize = float32(len(itemB))

	log.Debugf("Size (Bytes) of %v - %v", item.Name, itemSize)

	itemsSizeB, _ := json.Marshal(items)
	itemsSize = float32(len(itemsSizeB))
	log.Debugf("Total size (Bytes) of inventory items after including %v - %v", item.Name, itemsSize)

	//Refer to https://wiki.ubuntu.com/UnitsPolicy regarding KiB to bytes conversion.
	//TODO: 200 KB limit might be too less for certain inventory types like Patch - we might have to revise that and
	//use different limits for different category.
	if (itemSize/1024) > model.SizeLimitKBPerInventoryType || (itemsSize/1024) > model.TotalSizeLimitKB {
		return false
	}

	return true
}

// ConvertToCurrentAssociationsMap converts a list of current association to a map of association.
func ConvertToCurrentAssociationsMap(input []*associateModel.InstanceAssociation) (currentAssociations map[string]string) {
	currentAssociations = make(map[string]string)

	for _, v := range input {
		currentAssociations[*v.Association.AssociationId] = v.CreateDate.String()
	}

	return
}

// RefreshLastTrackedAssociationExecutions refreshes map of tracked association executions.
func RefreshLastTrackedAssociationExecutions(oldTrackedExecutions, currentAssociations map[string]string) (newTrackedExecutions map[string]string) {
	newTrackedExecutions = make(map[string]string)

	//iterate over oldExecutions and see if any doc is not associated anymore - if so don't include that doc in the
	//new map of tracked association execution

	for doc := range oldTrackedExecutions {
		if _, associationFound := currentAssociations[doc]; associationFound {
			//the execution time of inventory remains the same so copy over that data
			newTrackedExecutions[doc] = oldTrackedExecutions[doc]
		}
	}

	return
}

// IsMulitpleAssociationPresent returns true if there are multiple associations for inventory plugin else it returns false.
// It also refreshes map of tracked association executions accordingly.
func (p *Plugin) IsMulitpleAssociationPresent(currentAssociationID string) (status bool, otherAssociationID string) {
	var otherAssociationFound bool

	// we might end up changing value of p.lastAssociationId
	associationLock.Lock()
	defer associationLock.Unlock()

	log := p.context.Log()
	executionTime := time.Now().String()

	log.Debugf("Detecting multiple association - when executing - %v at time - %v",
		currentAssociationID, executionTime)

	if len(p.lastExecutedAssociations) == 0 {
		//lastExecutedAssociations is empty - which means this must be the first association run - return false
		//but before returning - add the current association to the map with execution time.
		p.lastExecutedAssociations[currentAssociationID] = executionTime
		status = false

		log.Debugf("There are 0 older association executions tracked - this is the first run - no multiple associations for inventory")
	} else {
		//There have been earlier associations so need to compare with current associations

		//Get all current associations
		p.currentAssociations = ConvertToCurrentAssociationsMap(associationsProvider())

		log.Debugf("Map of all current associations - %v", p.currentAssociations)

		for associationID := range p.currentAssociations {
			if associationID == currentAssociationID {
				//we are not interested in current association under whose context inventory plugin
				//is currently executing
				continue
			}

			if _, otherAssociationFound = p.lastExecutedAssociations[associationID]; otherAssociationFound {
				//There exists a document which:
				// - is not the current association
				// - is currently associated with instance
				// - has been previously executed by inventory plugin
				//This is a multiple association scenario which is not supported by inventory plugin
				status = true

				//even though this execution run would fail we should still add this execution in the map
				//of lastAssociationExecutions to fail executions of other associations
				p.lastExecutedAssociations[currentAssociationID] = executionTime

				//need to return the detected multiple association ID
				otherAssociationID = associationID

				//no need to check for any other associations from current associations - since we
				//already found a multiple association
				log.Debugf("Found another association - %v that executed inventory plugin at - %v",
					associationID, p.lastExecutedAssociations[associationID])
				break
			}
		}

		if !otherAssociationFound {
			//there wasn't any multiple association that was found -> return false
			status = false

			//we should add the current execution as well to the list of earlier tracked associations
			p.lastExecutedAssociations[currentAssociationID] = executionTime

			log.Debugf("Found no multiple associations for inventory plugin")
		}

		//refresh last tracked executions to ensure we delete old entries of association executions that aren't
		//associated anymore.
		p.lastExecutedAssociations = RefreshLastTrackedAssociationExecutions(p.lastExecutedAssociations, p.currentAssociations)
	}

	return
}

// IsInventoryBeingInvokedAsAssociation returns true if inventory plugin is invoked via ssm-associate or else it returns false.
// It throws error if the detection itself fails
func (p *Plugin) IsInventoryBeingInvokedAsAssociation(fileName string) (status bool, err error) {
	var content string
	var docState docmanagerModel.DocumentState
	log := p.context.Log()

	//since the document is still getting executed - it must be in Current folder
	path := filepath.Join(appconfig.DefaultDataStorePath,
		p.machineID,
		appconfig.DefaultDocumentRootDirName,
		appconfig.DefaultLocationOfState,
		appconfig.DefaultLocationOfCurrent)

	absPathOfDoc := filepath.Join(path, fileName)

	//read file & then determine if document is of association type
	if fileutil.Exists(absPathOfDoc) {
		log.Debugf("Found the document that's executing inventory plugin - %v", absPathOfDoc)

		//read file
		if content, err = fileutil.ReadAllText(absPathOfDoc); err == nil {
			if err = json.Unmarshal([]byte(content), &docState); err == nil {
				status = docState.IsAssociation()
			}
		}

	} else {
		err = fmt.Errorf("Inventory plugin can't locate the execution document which invoked it. The doc should have been in the location - %v", absPathOfDoc)
	}

	return
}

// ParseAssociationIdFromFileName parses associationID from the given input
// NOTE: Input will be of format - AssociationID.RunID -> as per the format of bookkeepingfilename for associate documents
func (p *Plugin) ParseAssociationIdFromFileName(input string) string {
	return strings.Split(input, ".")[0]
}

// Worker plugin implementation

// Execute runs the inventory plugin
func (p *Plugin) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag, subDocumentRunner runpluginutil.PluginRunner) (res contracts.PluginResult) {
	log := context.Log()
	res.StartDateTime = time.Now()
	defer func() { res.EndDateTime = time.Now() }()

	var errorMsg, associationID string
	var dataB []byte
	var err error
	var isAssociation bool
	var inventoryInput PluginInput
	var inventoryOutput contracts.PluginOutput

	pluginName := Name()
	dataB, _ = json.Marshal(config)
	log.Debugf("Starting %v with configuration \n%v", pluginName, jsonutil.Indent(string(dataB)))

	//set up orchestration dir where stdout & stderr files will be stored to be later uploaded to S3.
	orchestrationDir := fileutil.BuildPath(config.OrchestrationDirectory, Name())
	log.Debugf("Setting up orchestrationDir %v for Inventory Plugin's execution", orchestrationDir)

	// create orchestration dir if needed - only with read-write access (no need for execute access on the folder)
	// because inventory plugin only writes stdout, stderr in files unlike runshellscript plugin which also creates
	// an executable script file in the orchestration dir
	if err = fileutil.MakeDirs(orchestrationDir); err != nil {
		inventoryOutput.MarkAsFailed(log, fmt.Errorf("failed to create orchestrationDir directory, %v", orchestrationDir))
		res = setPluginResult(inventoryOutput, res)
		return
	}

	//TODO: take care of cancel flag (SSM-INV-233)

	associationID = p.ParseAssociationIdFromFileName(config.BookKeepingFileName)

	// Check if the inventory plugin is being invoked as association, if not or if detection fails for some reason,
	// then fail association - because inventory plugin currently supports invocation via ssm associate only.
	if isAssociation, err = p.IsInventoryBeingInvokedAsAssociation(config.BookKeepingFileName); err != nil || !isAssociation {
		if err != nil {
			errorMsg = fmt.Sprintf(errorMsgForUnableToDetectInvocationType, pluginName, err.Error())
		} else {
			errorMsg = fmt.Sprintf(errorMsgForExecutingInventoryViaAssociate, pluginName)
		}

		log.Error(errorMsg)
		//setting up plugin output
		inventoryOutput.ExitCode = 1
		inventoryOutput.Stderr = errorMsg
		inventoryOutput.Status = contracts.ResultStatusFailed

		res = setPluginResult(inventoryOutput, res)
		p.uploadOutputToS3(context, config.PluginID, orchestrationDir, config.OutputS3BucketName, config.OutputS3KeyPrefix, res.StandardOutput, res.StandardError)
		pluginutil.PersistPluginInformationToCurrent(log, config.PluginID, config, res)

		return
	}

	log.Debugf("%v plugin is being invoked via ssm-associate - proceeding ahead with execution", pluginName)

	// Check if there exists multiple associations for software inventory plugin, if so - then fail association - because
	// inventory plugin supports single association only.

	// NOTE: as per contract with associate functionality - bookkeepingfilename will always contain associationId.
	// bookkeepingfilename will be of format - associationID.RunID for associations, for command it will simply be commandID

	if status, extraAssociationId := p.IsMulitpleAssociationPresent(associationID); status {
		errorMsg = fmt.Sprintf(errorMsgForMultipleAssociations,
			pluginName,
			associationID,
			extraAssociationId)

		log.Error(errorMsg)

		//setting up plugin output
		inventoryOutput.ExitCode = 1
		inventoryOutput.Stderr = errorMsg
		inventoryOutput.Status = contracts.ResultStatusFailed

		res = setPluginResult(inventoryOutput, res)
		p.uploadOutputToS3(context, config.PluginID, orchestrationDir, config.OutputS3BucketName, config.OutputS3KeyPrefix, res.StandardOutput, res.StandardError)
		pluginutil.PersistPluginInformationToCurrent(log, config.PluginID, config, res)

		return
	}

	//loading Properties as map since aws:softwareInventory gets configuration in form of map
	if dataB, err = json.Marshal(config.Properties); err != nil {
		errorMsg = fmt.Sprintf("Unable to marshal plugin input to %v due to %v", pluginName, err.Error())
		log.Error(errorMsg)
		//setting up plugin output
		inventoryOutput.ExitCode = 1
		inventoryOutput.Stderr = errorMsg
		inventoryOutput.Status = contracts.ResultStatusFailed

		res = setPluginResult(inventoryOutput, res)
		p.uploadOutputToS3(context, config.PluginID, orchestrationDir, config.OutputS3BucketName, config.OutputS3KeyPrefix, res.StandardOutput, res.StandardError)
		pluginutil.PersistPluginInformationToCurrent(log, config.PluginID, config, res)

		return
	}

	if err = json.Unmarshal(dataB, &inventoryInput); err != nil {
		errorMsg = fmt.Sprintf(errorMsgForInvalidInventoryInput, pluginName)
		log.Error(errorMsg)
		//setting up plugin output
		inventoryOutput.ExitCode = 1
		inventoryOutput.Stderr = errorMsg
		inventoryOutput.Status = contracts.ResultStatusFailed

		res = setPluginResult(inventoryOutput, res)
		p.uploadOutputToS3(context, config.PluginID, orchestrationDir, config.OutputS3BucketName, config.OutputS3KeyPrefix, res.StandardOutput, res.StandardError)

		pluginutil.PersistPluginInformationToCurrent(log, config.PluginID, config, res)

		return
	}

	dataB, _ = json.Marshal(inventoryInput)
	log.Debugf("Inventory configuration after parsing - %v", string(dataB))

	inventoryOutput = p.ApplyInventoryPolicy(context, inventoryInput)
	res = setPluginResult(inventoryOutput, res)
	res.StandardOutput = pluginutil.StringPrefix(res.StandardOutput, p.MaxStdoutLength, p.OutputTruncatedSuffix)
	res.StandardError = pluginutil.StringPrefix(res.StandardError, p.MaxStderrLength, p.OutputTruncatedSuffix)

	//check inventory plugin output
	if inventoryOutput.ExitCode != 0 {
		log.Debugf("Execution of %v failed with configuration - %v because of - %v", pluginName, config, res.Output)
		res.Status = contracts.ResultStatusFailed
	} else {
		log.Debugf("Execution of %v was successful with configuration - %v with output - %v", pluginName, config, res.Output)
		res.Status = contracts.ResultStatusSuccess
	}

	p.uploadOutputToS3(context, config.PluginID, orchestrationDir, config.OutputS3BucketName, config.OutputS3KeyPrefix, res.StandardOutput, res.StandardError)

	return
}

// setPluginResult sets plugin result that is returned to Agent framework from inventory plugin output
func setPluginResult(pluginOutput contracts.PluginOutput, result contracts.PluginResult) contracts.PluginResult {
	result.Code = pluginOutput.ExitCode
	result.Status = pluginOutput.Status
	result.StandardError = pluginOutput.Stderr
	result.StandardOutput = pluginOutput.Stdout

	//pluginOutput.String -> concatenates stdout & stderr - keeping in mind 2500 chars limit, which will reflect in
	//association status/command result
	result.Output = pluginOutput.String()

	return result
}

// uploadOutputToS3 uploads inventory output to S3
func (p *Plugin) uploadOutputToS3(context context.T, pluginID string, orchestrationDir, s3bucketKey, s3Key, stdout, stderr string) {

	var status bool
	log := context.Log()

	// store stdout & stderr in files under orchestrationDir - because uploadToS3 utility works by uploading the
	// concerned files to S3.

	// Create output file paths - where stdout & stderr files will be stored
	stdoutFilePath := filepath.Join(orchestrationDir, p.StdoutFileName)
	stderrFilePath := filepath.Join(orchestrationDir, p.StderrFileName)
	log.Debugf("stdout file %v, stderr file %v", stdoutFilePath, stderrFilePath)

	if status, _ = fileutil.WriteIntoFileWithPermissions(stdoutFilePath, stdout, appconfig.ReadWriteAccess); !status {
		log.Errorf("Unable to store %s output (stdout) to files", Name())
	}

	if status, _ = fileutil.WriteIntoFileWithPermissions(stderrFilePath, stderr, appconfig.ReadWriteAccess); !status {
		log.Errorf("Unable to store %s output (stderr) to files", Name())
	}

	uploadOutputToS3BucketErrors := p.ExecuteUploadOutputToS3Bucket(log, pluginID, orchestrationDir, s3bucketKey, s3Key, false, "", stdout, stderr)
	if len(uploadOutputToS3BucketErrors) > 0 {
		log.Errorf("Unable to upload the logs: %s", uploadOutputToS3BucketErrors)
	}
}
