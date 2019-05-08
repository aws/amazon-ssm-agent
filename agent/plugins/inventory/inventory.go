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
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/datauploader"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/application"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/awscomponent"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/billinginfo"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/custom"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/file"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/instancedetailedinformation"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/network"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/registry"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/role"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/service"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/windowsUpdate"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ssm"
)

// TODO: add more unit tests.

const (
	errorMsgForMultipleAssociations           = "%v detected multiple inventory configurations associated with one instance. Each instance can be associated with just one inventory configuration. Conflicting inventory configuration IDs: %v and %v"
	errorMsgForInvalidInventoryInput          = "invalid or unrecognized input was received for %v plugin"
	errorMsgForExecutingInventoryViaAssociate = "%v plugin can only be invoked via ssm-associate"
	errorMsgForUnableToDetectInvocationType   = "it could not be detected if %v plugin was invoked via ssm-associate because - %v"
	errorMsgForInabilityToSendDataToSSM       = "inventory data could not be uploaded to Systems Manager. Additional troubleshooting information - %v"
	msgWhenNoDataToReturnForInventoryPlugin   = "Inventory policy has been successfully applied but there is no inventory data to upload to SSM"
	successfulMsgForInventoryPlugin           = "Inventory policy has been successfully applied and collected inventory data has been uploaded to SSM"
)

// PluginInput represents configuration which is applied to inventory plugin during execution.
type PluginInput struct {
	contracts.PluginInput
	Applications                string
	AWSComponents               string
	NetworkConfig               string
	BillingInfo                 string
	Files                       string
	WindowsRoles                string
	Services                    string
	WindowsRegistry             string
	WindowsUpdates              string
	InstanceDetailedInformation string
	CustomInventory             string
	CustomInventoryDirectory    string
}

// decoupling platform.InstanceID for easy testability
var machineIDProvider = machineInfoProvider

func machineInfoProvider() (name string, err error) {
	return platform.InstanceID()
}

// Plugin encapsulates the logic of configuring, starting and stopping inventory plugin
type Plugin struct {
	context    context.T
	stopPolicy *sdkutil.StopPolicy

	//supportedGatherers is a map of all inventory gatherers supported by current OS
	// (e.g. WindowsUpdateGatherer is not included when running on Unix based systems)
	supportedGatherers gatherers.SupportedGatherer

	//installedGatherers is a map of gatherers that can run on all platforms
	installedGatherers gatherers.InstalledGatherer

	//uploader handles uploading inventory data to SSM.
	uploader datauploader.T

	// machineID of the machine where agent is running - useful during command detection
	machineID string
}

// Name returns the plugin name
func Name() string {
	return appconfig.PluginNameAwsSoftwareInventory
}

// NewPlugin creates a new inventory worker plugin.
func NewPlugin(context context.T) (*Plugin, error) {
	var err error
	var p = Plugin{}

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
func (p *Plugin) ApplyInventoryPolicy(context context.T, inventoryInput PluginInput, output iohandler.IOHandler) {
	log := p.context.Log()
	var optimizedInventoryItems, nonOptimizedInventoryItems []*ssm.InventoryItem
	var items []model.Item
	var err error

	//map of all valid gatherers & respective configs to run
	var gatherers map[gatherers.T]model.Config

	//validate all gatherers
	if gatherers, err = p.ValidateInventoryInput(context, inventoryInput); err != nil {
		log.Info(err.Error())
		output.SetExitCode(1)
		output.AppendError(err.Error())
		return
	}

	//execute all eligible gatherers with their respective config
	if items, err = p.RunGatherers(gatherers); err != nil {
		log.Info(err.Error())
		output.SetExitCode(1)
		output.AppendError(err.Error())
		return
	}

	//check if there is data to send to SSM
	if len(items) == 0 {
		//no data to send to ssm - no need to call PutInventory API
		log.Info(msgWhenNoDataToReturnForInventoryPlugin)
		output.SetExitCode(0)
		output.AppendInfo(msgWhenNoDataToReturnForInventoryPlugin)
		return
	}

	//log collected data before sending
	d, _ := json.Marshal(items)
	log.Debugf("Collected Inventory data: %v", string(d))

	if optimizedInventoryItems, nonOptimizedInventoryItems, err = p.uploader.ConvertToSsmInventoryItems(p.context, items); err != nil {
		log.Infof("Encountered error in converting data to SSM InventoryItems - %v. Skipping upload to SSM", err.Error())
		output.SetExitCode(1)
		output.AppendError(err.Error())
		return
	}

	log.Debugf("Optimized data - \n%v \n Non-optimized data - \n%v",
		optimizedInventoryItems,
		nonOptimizedInventoryItems)

	//first send data in optimized fashion
	if err = p.uploader.SendDataToSSM(context, optimizedInventoryItems); err != nil {

		if shouldRetryWithNonOptimizedData(err, log) {
			//call putinventory again with non-optimized dataset
			if err = p.uploader.SendDataToSSM(context, nonOptimizedInventoryItems); err != nil {
				//sending non-optimized data also failed
				propagateSSMError(output, err, log)
				return
			}
		} else {
			//some other error happened for which there is no need to retry - upload failed
			propagateSSMError(output, err, log)
			return
		}
	}

	log.Infof("%v uploaded inventory data to SSM", Name())
	output.SetExitCode(0)
	output.AppendInfo(successfulMsgForInventoryPlugin)

	return
}

// ApplyInventoryFrequentCollector applies frequent collector regarding which gatherers to run
func (p Plugin) ApplyInventoryFrequentCollector(context context.T, gatherers map[gatherers.T]model.Config, output iohandler.IOHandler) {
	log := p.context.Log()

	var dirtyItems []*ssm.InventoryItem
	var items []model.Item
	var err error

	//execute all specified gatherers with their respective config
	if items, err = p.RunGatherers(gatherers); err != nil {
		log.Debugf("failed at RunGatherers, error : %#v", err)
		log.Info(err.Error())
		output.SetExitCode(1)
		output.AppendError(err.Error())
		return
	}

	//check if there is data to send to SSM
	if len(items) == 0 {
		//no data to send to ssm - no need to call PutInventory API
		log.Info(msgWhenNoDataToReturnForInventoryPlugin)
		output.SetExitCode(0)
		output.AppendInfo(msgWhenNoDataToReturnForInventoryPlugin)
		return
	}

	if dirtyItems, err = p.uploader.GetDirtySsmInventoryItems(context, items); err != nil {
		log.Debugf("Encountered error in collecting dirty Inventory items - %#v. Skipping upload to SSM", err.Error())
		output.SetExitCode(1)
		output.AppendError(err.Error())
		return
	}

	if len(dirtyItems) == 0 {
		log.Debugf("No dirty inventory items found, skipping uploading")
		log.Info(msgWhenNoDataToReturnForInventoryPlugin)
		output.SetExitCode(0)
		output.AppendInfo(msgWhenNoDataToReturnForInventoryPlugin)
		return
	}

	if err = p.uploader.SendDataToSSM(context, dirtyItems); err != nil {
		//some other error happened for which there is no need to retry - upload failed
		log.Debugf(" Error happened while p.uploader.SendDataToSSM")
		propagateSSMError(output, err, log)
		return
	}

	log.Infof("%v uploaded inventory data from frequent collector to SSM", Name())
	output.SetExitCode(0)
	output.AppendInfo(successfulMsgForInventoryPlugin)

	return
}

// shouldRetryWithNonOptimizedData will return true if the Exception occurred is one of ItemContentMismatchException
// or InvalidItemContentException and will retry sending data to SSM. It will return false, if any other error occurs.
func shouldRetryWithNonOptimizedData(err error, log log.T) bool {
	awsErr, isAwsError := err.(awserr.Error)
	if isAwsError {
		//NOTE: awsErr.Code -> is not the error code but the exception name itself!!!!
		if awsErr.Code() == "ItemContentMismatchException" || awsErr.Code() == "InvalidItemContentException" {
			log.Debugf("%v encountered - inventory plugin will try sending data again", awsErr.Code())
			return true
		}
	}
	log.Debugf("Unexpected error encountered - %v. No point trying to send data again", err.Error())
	return false
}

func propagateSSMError(output iohandler.IOHandler, err error, log log.T) {
	message := fmt.Sprintf(errorMsgForInabilityToSendDataToSSM, err.Error())
	log.Info(message)
	output.SetExitCode(1)
	output.AppendError(message)
}

func (p *Plugin) GetSupportedGatherer(gatherName string) (gatherers.T, bool) {
	if gatherer, gathererPresent := p.supportedGatherers[gatherName]; gathererPresent {
		return gatherer, true
	}
	return nil, false
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
			err = fmt.Errorf("inventory gatherer - %v is not installed", name)
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

func (p *Plugin) validateGathererWithFilters(context context.T, collectionPolicy, gathererName string, filters string) (status bool, gatherer gatherers.T, policy model.Config, err error) {
	if filters != "" {
		if status, gatherer, err = p.CanGathererRun(context, gathererName); err != nil {
			return
		}

		if status {
			policy = model.Config{Collection: collectionPolicy, Filters: filters}
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

	predefinedGatherers := map[string]string{
		application.GathererName:                 input.Applications,
		awscomponent.GathererName:                input.AWSComponents,
		role.GathererName:                        input.WindowsRoles,
		service.GathererName:                     input.Services,
		network.GathererName:                     input.NetworkConfig,
		billinginfo.GathererName:                 input.BillingInfo,
		windowsUpdate.GathererName:               input.WindowsUpdates,
		instancedetailedinformation.GathererName: input.InstanceDetailedInformation,
	}

	predefinedGatherersWithFilters := map[string]string{
		file.GathererName:     input.Files,
		registry.GathererName: input.WindowsRegistry,
	}

	//NOTE:
	// If the gatherer is installed but not supported by current platform, we will skip that gatherer. If the
	// gatherer is not installed,  we error out & don't send the data collected from other supported gatherers
	// - this is because we don't send partial inventory data as part of 1 inventory policy.
	for gathererName, collectionPolicy := range predefinedGatherers {
		if canGathererRun, gatherer, cfg, err = p.validatePredefinedGatherer(context, collectionPolicy, gathererName); err != nil {
			log.Errorf("Error while validating gatherer %v", err.Error())
			return
		} else if canGathererRun {
			configuredGatherers[gatherer] = cfg
		}
	}

	for gathererName, filters := range predefinedGatherersWithFilters {
		if canGathererRun, gatherer, cfg, err = p.validateGathererWithFilters(context, "", gathererName, filters); err != nil {
			log.Errorf("Error while validating gatherer %v", err.Error())
			return
		} else if canGathererRun {
			configuredGatherers[gatherer] = cfg
		}
	}

	//checking custom gatherer
	if canGathererRun, gatherer, cfg, err = p.validateCustomGatherer(context, input.CustomInventory, input.CustomInventoryDirectory); err != nil {
		log.Errorf("Error while validating gatherer %v", err.Error())
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
		start := time.Now()

		if gItems, err = gatherer.Run(p.context, config); err != nil {
			err = fmt.Errorf("Encountered error while executing %v. Error - %v", name, err.Error())
			break

		} else {
			elapsed := time.Since(start)
			log.Infof("execution time for gatherer - %v: %s", name, elapsed)

			items = append(items, gItems...)

			//TODO: Each gatherer shall check each item's size and stop collecting if size exceed immediately
			//TODO: only check the total item size at this function, whenever total size exceed, stop
			//TODO: immediately and raise association error
			//return error if collected data breaches size limit
			for _, v := range gItems {
				if !p.VerifyInventoryDataSize(v, items) {
					err = log.Errorf("the size of the collected data exceeded the maximum allowable size")
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

	log.Infof("Size (Bytes) of %v - %v", item.Name, itemSize)
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

// IsMulitpleAssociationPresent returns true if there are multiple associations for inventory plugin else it returns false.
func (p *Plugin) IsMulitpleAssociationPresent(currentAssociationID string, config contracts.Configuration) (status bool, othersfound string) {
	var currentInventoryAssociations []string

	err := jsonutil.Remarshal(config.CurrentAssociations, &currentInventoryAssociations)
	if err != nil {
		p.context.Log().Errorf("failed to remarshal plugin settings: %v", err)
		return false, ""
	}
	//test whether other associations are attached right now
	for _, assocID := range currentInventoryAssociations {
		if assocID != currentAssociationID {
			return true, assocID
		}
	}
	return false, ""
}

// IsInventoryBeingInvokedAsAssociation returns true if inventory plugin is invoked via ssm-associate or else it returns false.
// It throws error if the detection itself fails
func (p *Plugin) IsInventoryBeingInvokedAsAssociation(fileName string) (status bool, err error) {
	var content string
	var docState contracts.DocumentState
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
		err = fmt.Errorf("Inventory plugin could not locate the execution document which invoked it. The document is expected to be in the location - %v", absPathOfDoc)
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
func (p *Plugin) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag, output iohandler.IOHandler) {
	log := context.Log()

	var errorMsg, associationID string
	var dataB []byte
	var err error
	var isAssociation bool
	var inventoryInput PluginInput

	pluginName := Name()
	dataB, _ = json.Marshal(config)
	log.Debugf("Starting %v with configuration \n%v", pluginName, jsonutil.Indent(string(dataB)))

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
		output.SetExitCode(1)
		output.SetStatus(contracts.ResultStatusFailed)
		output.AppendError(errorMsg)
		return
	}

	log.Debugf("%v plugin is being invoked via ssm-associate - proceeding ahead with execution", pluginName)

	// Check if there exists multiple associations for software inventory plugin, if so - then fail association - because
	// inventory plugin supports single association only.

	// NOTE: as per contract with associate functionality - bookkeepingfilename will always contain associationId.
	// bookkeepingfilename will be of format - associationID.RunID for associations, for command it will simply be commandID

	if status, extraAssociationId := p.IsMulitpleAssociationPresent(associationID, config); status {
		errorMsg = fmt.Sprintf(errorMsgForMultipleAssociations,
			pluginName,
			associationID,
			extraAssociationId)

		log.Error(errorMsg)

		//setting up plugin output
		output.SetExitCode(1)
		output.SetStatus(contracts.ResultStatusFailed)
		output.AppendError(errorMsg)
		return
	}

	//loading Properties as map since aws:softwareInventory gets configuration in form of map
	log.Infof("config.Properties %v", config.Properties)
	if dataB, err = json.Marshal(config.Properties); err != nil {
		errorMsg = fmt.Sprintf("Unable to marshal plugin input to %v due to %v", pluginName, err.Error())
		log.Error(errorMsg)
		//setting up plugin output
		output.SetExitCode(1)
		output.SetStatus(contracts.ResultStatusFailed)
		output.AppendError(errorMsg)
		return
	}

	if err = json.Unmarshal(dataB, &inventoryInput); err != nil {
		errorMsg = fmt.Sprintf(errorMsgForInvalidInventoryInput, pluginName)
		log.Error(errorMsg)

		//setting up plugin output
		output.SetExitCode(1)
		output.SetStatus(contracts.ResultStatusFailed)
		output.AppendError(errorMsg)
		return
	}

	dataB, _ = json.Marshal(inventoryInput)
	log.Infof("Inventory configuration after parsing - %v", string(dataB))

	p.ApplyInventoryPolicy(context, inventoryInput, output)

	//check inventory plugin output
	if output.GetExitCode() != 0 {
		log.Debugf("Execution of %v failed with configuration - %v because of - %v", pluginName, config, output.GetStderr())
		output.SetStatus(contracts.ResultStatusFailed)
	} else {
		log.Debugf("Execution of %v was successful with configuration - %v with output - %v", pluginName, config, output.GetStdout())
		output.SetStatus(contracts.ResultStatusSuccess)
	}

	return
}
