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
<<<<<<< 6cb9189dc984d14ab7497b36d90de8a0d1df14a4
	"strings"
=======
>>>>>>> 1) converting inventory plugin from core plugin to worker plugin, 2) integrating with associate functionality, 3) detecting multiple associations for inventory, 4) minor go lint fixes
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	associateModel "github.com/aws/amazon-ssm-agent/agent/association/model"
	"github.com/aws/amazon-ssm-agent/agent/association/schedulemanager"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/inventory/datauploader"
	"github.com/aws/amazon-ssm-agent/agent/inventory/gatherers"
	"github.com/aws/amazon-ssm-agent/agent/inventory/gatherers/application"
	"github.com/aws/amazon-ssm-agent/agent/inventory/gatherers/awscomponent"
	"github.com/aws/amazon-ssm-agent/agent/inventory/gatherers/custom"
	"github.com/aws/amazon-ssm-agent/agent/inventory/gatherers/network"
	"github.com/aws/amazon-ssm-agent/agent/inventory/gatherers/windowsUpdate"
	"github.com/aws/amazon-ssm-agent/agent/inventory/model"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
)

//TODO: add more unit tests.

const (
	errorMsgForMultipleAssociations             = "%v doesn't support multiple associations"
	errorMsgForInvalidInventoryInput            = "Unrecongnized input for %v plugin"
	errorMsgForExecutingInventoryThroughCommand = "Run command is not supported for %v plugin"
	successfulMsgForInventoryPlugin             = "Inventory policy has been successfully applied and collected inventory data has been uploaded to SSM"
)

var (
	//associationLock ensures that all read & write for lastExecutedAssociations & currentAssociations is thread-safe
	associationLock sync.RWMutex
)

// PluginInput represents configuration which is applied to inventory plugin during execution.
type PluginInput struct {
	contracts.PluginInput
	Applications             string
	AWSComponents            string
	NetworkConfig            string
	WindowsUpdates           string
	CustomInventory          string
	CustomInventoryDirectory string
}

// decoupling schedulemanager.Schedules() for easy testability
var associationsProvider = getCurrentAssociations

func getCurrentAssociations() []*associateModel.InstanceAssociation {
	return schedulemanager.Schedules()
}

// PluginOutput represents the output of inventory plugin
type PluginOutput struct {
	contracts.PluginOutput
}

// Plugin encapsulates the logic of configuring, starting and stopping inventory plugin
type Plugin struct {
	pluginutil.DefaultPlugin

	context    context.T
	stopPolicy *sdkutil.StopPolicy
	ssm        *ssm.SSM

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
}

// Name returns the plugin name
func Name() string {
	return appconfig.PluginNameAwsSoftwareInventory
}

// NewPlugin creates a new inventory worker plugin.
func NewPlugin(context context.T, pluginConfig pluginutil.PluginConfig) (*Plugin, error) {
	var appCfg appconfig.SsmagentConfig
	var err error
	var p = Plugin{}

	//setting up default worker plugin config
	p.MaxStdoutLength = pluginConfig.MaxStdoutLength
	p.MaxStderrLength = pluginConfig.MaxStderrLength
	p.StdoutFileName = pluginConfig.StdoutFileName
	p.StderrFileName = pluginConfig.StderrFileName
	p.OutputTruncatedSuffix = pluginConfig.OutputTruncatedSuffix
	p.Uploader = pluginutil.GetS3Config()
	p.ExecuteUploadOutputToS3Bucket = pluginutil.UploadOutputToS3BucketExecuter(p.UploadOutputToS3Bucket)

	// since this is initialization - lastExecutedAssociations should be empty.
	// NOTE: this will get populated when inventory plugin gets invoked via association later.
	p.lastExecutedAssociations = make(map[string]string)

	c := context.With("[" + Name() + "]")
	log := c.Log()

	// reading agent appconfig
	if appCfg, err = appconfig.Config(false); err != nil {
		return &p, err
	}

	// setting ssm client config
	cfg := sdkutil.AwsConfig()
	cfg.Region = &appCfg.Agent.Region
	cfg.Endpoint = &appCfg.Ssm.Endpoint

	p.context = c
	p.stopPolicy = sdkutil.NewStopPolicy(Name(), model.ErrorThreshold)
	p.ssm = ssm.New(session.New(cfg))

	//loads all registered gatherers (for now only a dummy application gatherer is loaded in memory)
	p.supportedGatherers, p.installedGatherers = gatherers.InitializeGatherers(p.context)
	//initializes SSM Inventory uploader
	if p.uploader, err = datauploader.NewInventoryUploader(c); err != nil {
		err = log.Errorf("Unable to configure SSM Inventory uploader - %v", err.Error())
	}

	return &p, err
}

// ApplyInventoryPolicy applies given inventory policy regarding which gatherers to run
func (p *Plugin) ApplyInventoryPolicy(context context.T, inventoryInput PluginInput) (inventoryOutput PluginOutput) {
	log := p.context.Log()
	var inventoryItems []*ssm.InventoryItem
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

	//log collected data before sending
	d, _ := json.Marshal(items)
	log.Infof("Collected Inventory data: %v", string(d))

	if inventoryItems, err = p.uploader.ConvertToSsmInventoryItems(p.context, items); err != nil {
		log.Infof("Encountered error in converting data to SSM InventoryItems - %v. Skipping upload to SSM", err.Error())
		inventoryOutput.ExitCode = 1
		inventoryOutput.Stderr = err.Error()
		return
	}

	p.uploader.SendDataToSSM(p.context, inventoryItems)
	inventoryOutput.ExitCode = 0
	inventoryOutput.Stdout = successfulMsgForInventoryPlugin

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
<<<<<<< 6cb9189dc984d14ab7497b36d90de8a0d1df14a4
=======
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
>>>>>>> 1) converting inventory plugin from core plugin to worker plugin, 2) integrating with associate functionality, 3) detecting multiple associations for inventory, 4) minor go lint fixes
	}

	return
}

<<<<<<< 6cb9189dc984d14ab7497b36d90de8a0d1df14a4
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

=======
>>>>>>> 1) converting inventory plugin from core plugin to worker plugin, 2) integrating with associate functionality, 3) detecting multiple associations for inventory, 4) minor go lint fixes
// ValidateInventoryInput validates inventory input and returns a map of eligible gatherers & their corresponding config.
// It throws an error if gatherer is not recongnized/installed.
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
<<<<<<< 6cb9189dc984d14ab7497b36d90de8a0d1df14a4

	//checking awscomponents gatherer
	if canGathererRun, gatherer, cfg, err = p.validatePredefinedGatherer(context, input.AWSComponents, awscomponent.GathererName); err != nil {
		return
	} else if canGathererRun {
		configuredGatherers[gatherer] = cfg
	}

=======

	//checking awscomponents gatherer
	if canGathererRun, gatherer, cfg, err = p.validatePredefinedGatherer(context, input.AWSComponents, awscomponent.GathererName); err != nil {
		return
	} else if canGathererRun {
		configuredGatherers[gatherer] = cfg
	}

>>>>>>> 1) converting inventory plugin from core plugin to worker plugin, 2) integrating with associate functionality, 3) detecting multiple associations for inventory, 4) minor go lint fixes
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
<<<<<<< 6cb9189dc984d14ab7497b36d90de8a0d1df14a4

	for _, v := range input {
		currentAssociations[*v.Association.AssociationId] = v.CreateDate.String()
	}

=======

	for _, v := range input {
		currentAssociations[*v.Association.AssociationId] = v.CreateDate.String()
	}

>>>>>>> 1) converting inventory plugin from core plugin to worker plugin, 2) integrating with associate functionality, 3) detecting multiple associations for inventory, 4) minor go lint fixes
	return
}

// RefreshLastTrackedAssociationExecutions refreshes map of tracked association executions.
func RefreshLastTrackedAssociationExecutions(oldTrackedExecutions, currentAssociations map[string]string) (newTrackedExecutions map[string]string) {
	newTrackedExecutions = make(map[string]string)

	//iterate over oldExecutions and see if any doc is not associated anymore - if so don't include that doc in the
	//new map of tracked association execution

<<<<<<< 6cb9189dc984d14ab7497b36d90de8a0d1df14a4
	for doc := range oldTrackedExecutions {
=======
	for doc, _ := range oldTrackedExecutions {
>>>>>>> 1) converting inventory plugin from core plugin to worker plugin, 2) integrating with associate functionality, 3) detecting multiple associations for inventory, 4) minor go lint fixes
		if _, associationFound := currentAssociations[doc]; associationFound {
			//the execution time of inventory remains the same so copy over that data
			newTrackedExecutions[doc] = oldTrackedExecutions[doc]
		}
	}

	return
}

// IsMulitpleAssociationPresent returns true if there are multiple associations for inventory plugin else it returns false.
// It also refreshes map of tracked association executions accordingly.
func (p *Plugin) IsMulitpleAssociationPresent(currentAssociationID string) (status bool) {
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

// IsInventoryBeingInvokedAsSSMCommand returns true if there are multiple associations for inventory plugin else it returns false.
func (p *Plugin) IsInventoryBeingInvokedAsSSMCommand() (status bool) {
	//TODO: implement following algo:
	// NOTE: 2 approaches - both of which would require configuration.bookkeepingfilename or configuration.MessageId (messageId is not really future proof since later implementations might only include doc state management)
	// 1) we know the path where internalCmdState file is stored - check if the file is present there -> if so - simply return true else false
	// 2) we know the path - read the document and then read the property -> isCommand and accordingly return
	return false
}

// ParseAssociationIdFromFileName parses associationID from the given input
// NOTE: Input will be of format - AssociationID.RunID -> as per the format of bookkeepingfilename for associate documents
func (p *Plugin) ParseAssociationIdFromFileName(input string) string {
	return strings.Split(input, ".")[0]
}

// Worker plugin implementation

// Execute runs the inventory plugin
func (p *Plugin) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag) (res contracts.PluginResult) {
	log := context.Log()
	res.StartDateTime = time.Now()
	defer func() { res.EndDateTime = time.Now() }()

	var errorMsg, associationID string
	var dataB []byte
	var err error
	var inventoryInput PluginInput
	var inventoryOutput PluginOutput

	pluginName := Name()
	dataB, _ = json.Marshal(config)
	log.Debugf("Starting %v with configuration \n%v", pluginName, jsonutil.Indent(string(dataB)))

	//TODO: take care of cancel flag

	// Check if there exists multiple associations for software inventory plugin, if so - then fail association - because
	// inventory plugin supports single association only.

	// NOTE: as per contract with associate functionality - bookkeepingfilename will always contain associationId.
	// bookkeepingfilename will be of format - associationID.RunID for associations, for command it will simply be commandID

	associationID = p.ParseAssociationIdFromFileName(config.BookKeepingFileName)

	if p.IsMulitpleAssociationPresent(associationID) {
		errorMsg = fmt.Sprintf(errorMsgForMultipleAssociations, pluginName)
		log.Error(errorMsg)
		res.Code = 1
		res.Output = errorMsg
		res.StandardError = errorMsg
		res.Status = contracts.AssociationErrorCodeInvalidAssociation

		pluginutil.PersistPluginInformationToCurrent(log, config.PluginID, config, res)

		return
	}

	// Check if the inventory plugin is being invoked via run command, if so - then fail association - because
	// inventory plugin currently supports invocation via ssm associate only.
	if p.IsInventoryBeingInvokedAsSSMCommand() {
		errorMsg = fmt.Sprintf(errorMsgForExecutingInventoryThroughCommand, pluginName)
		log.Error(errorMsg)
		res.Code = 1
		res.Output = errorMsg
		res.StandardError = errorMsg
		res.Status = contracts.AssociationErrorCodeInvalidAssociation

		pluginutil.PersistPluginInformationToCurrent(log, config.PluginID, config, res)

		return
	}

	//loading Properties as map since aws:softwareInventory gets configuration in form of map
	if dataB, err = json.Marshal(config.Properties); err != nil {
		errorMsg = fmt.Sprintf("Unable to marshal plugin input to %v due to %v", pluginName, err.Error())
		log.Error(errorMsg)
		res.Code = 1
		res.Output = errorMsg
		res.StandardError = errorMsg
		res.Status = contracts.ResultStatusFailed

		pluginutil.PersistPluginInformationToCurrent(log, config.PluginID, config, res)

		return
	}

	if err = json.Unmarshal(dataB, &inventoryInput); err != nil {
		errorMsg = fmt.Sprintf(errorMsgForInvalidInventoryInput, pluginName)
		log.Error(errorMsg)
		res.Code = 1
		res.Output = errorMsg
		res.StandardError = errorMsg
		res.Status = contracts.ResultStatusFailed

		pluginutil.PersistPluginInformationToCurrent(log, config.PluginID, config, res)

		return
	}

	dataB, _ = json.Marshal(inventoryInput)
	log.Debugf("Inventory configuration after parsing - %v", string(dataB))

	inventoryOutput = p.ApplyInventoryPolicy(context, inventoryInput)
	res.Code = inventoryOutput.ExitCode
	res.StandardError = inventoryOutput.Stderr
	res.StandardOutput = inventoryOutput.Stdout
	res.Output = inventoryOutput.String()

	//check inventory plugin output
	if inventoryOutput.ExitCode != 0 {
		log.Debugf("Execution of %v failed with configuration - %v because of - %v", pluginName, config, res.Output)
		res.Status = contracts.ResultStatusFailed
	} else {
		log.Debugf("Execution of %v was successful with configuration - %v with output - %v", pluginName, config, res.Output)
		res.Status = contracts.ResultStatusSuccess
	}

	return
}
