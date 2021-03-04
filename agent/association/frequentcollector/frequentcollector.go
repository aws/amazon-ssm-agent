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

// Package frequentcollector enable customers to detect changed inventory types and upload the changed inventory data to SSM service between 2 scheduled collections.
package frequentcollector

import (
	"encoding/json"
	"math"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/association/scheduleexpression"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/application"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/awscomponent"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/file"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/instancedetailedinformation"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/network"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/registry"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/role"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/service"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/windowsUpdate"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"

	AssociationModel "github.com/aws/amazon-ssm-agent/agent/association/model"
	InventoryModel "github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
)

const (
	minIntervalInSeconds          float64 = 300
	paramChangeDetectionFrequency         = "changeDetectionFrequency"
	paramChangeDetectionTypes             = "changeDetectionTypes"
)

type FrequentCollector struct {
	tickerForFrequentCollector *time.Ticker
	mutex                      sync.RWMutex
}

var frequentCollector *FrequentCollector
var once sync.Once

func init() {
	once.Do(func() {
		frequentCollector = &FrequentCollector{}
	})
}

// GetFrequentCollector returns a singleton instance of FrequentCollector
func GetFrequentCollector() *FrequentCollector {
	frequentCollector.mutex.RLock()
	defer frequentCollector.mutex.RUnlock()
	return frequentCollector
}

//ClearTicker stops the ticker for frequent collector, and sets it to nil
func (collector *FrequentCollector) ClearTicker() {
	collector.mutex.RLock()
	defer collector.mutex.RUnlock()

	if collector.tickerForFrequentCollector != nil {
		collector.tickerForFrequentCollector.Stop()
		collector.tickerForFrequentCollector = nil
	}
}

//resetTicker stop the old ticker for frequent collector and creates a new one
func (collector *FrequentCollector) resetTicker(d time.Duration) *time.Ticker {
	collector.ClearTicker()

	collector.mutex.RLock()
	defer collector.mutex.RUnlock()

	collector.tickerForFrequentCollector = time.NewTicker(d)
	return collector.tickerForFrequentCollector
}

//StartFrequentCollector starts the frequent collector per association configuration.
func (collector *FrequentCollector) StartFrequentCollector(context context.T, docState *contracts.DocumentState, scheduledAssociation *AssociationModel.InstanceAssociation) {
	collector.mutex.RLock()
	defer collector.mutex.RUnlock()

	log := context.Log()
	changeDetectionFrequency, _ := collector.getFrequentCollectInformation(context, docState)
	_, intervalInSeconds := collector.GetIntervalInSeconds(changeDetectionFrequency, scheduledAssociation.ParsedExpression)

	defer func() {
		// recover in case the init panics
		if msg := recover(); msg != nil {
			log.Errorf("something is wrong in FrequentCollector")
		}
	}()

	log.Infof("start frequent collector, intervalInSeconds: %d", intervalInSeconds)
	ticker := collector.resetTicker(time.Duration(intervalInSeconds) * time.Second)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("Frequent collector panic: %v", r)
				log.Errorf("Stacktrace:\n%s", debug.Stack())
			}
		}()
		for t := range ticker.C {
			log.Infof("Frequent collector, tick at %s, ticker address : %p", t.Format(time.UnixDate), collector.tickerForFrequentCollector)
			collector.collect(context, docState)
		}
	}()
}

//IsFrequentCollectorEnabled return a boolean indicating if the frequent collector is enabled per association configuration.
func (collector *FrequentCollector) IsFrequentCollectorEnabled(context context.T, docState *contracts.DocumentState, scheduledAssociation *AssociationModel.InstanceAssociation) bool {
	log := context.Log()

	if scheduledAssociation.IsRunOnceAssociation() {
		log.Infof("frequent collector is not enabled. scheduled association is a run once association.")
		return false
	}

	// frequent collector is only enabled if the association is scheduled with Rate expression.
	if !strings.HasPrefix(strings.ToLower(*scheduledAssociation.Association.ScheduleExpression), "rate") {
		log.Infof("frequent collector is not enabled. assoc schedule expression: %s , is not a rate expression", *scheduledAssociation.Association.ScheduleExpression)
		return false
	}

	changeDetectionFrequency, namesOfGatherers := collector.getFrequentCollectInformation(context, docState)
	if changeDetectionFrequency <= 0 || len(namesOfGatherers) <= 0 {
		log.Infof(" frequent collector is not enabled. changeDetectionFrequency: %d, len of gatherers: %d", changeDetectionFrequency, len(namesOfGatherers))
		return false
	}

	frequencyPresent, intervalInSeconds := collector.GetIntervalInSeconds(changeDetectionFrequency, scheduledAssociation.ParsedExpression)
	return frequencyPresent && intervalInSeconds > 0
}

//GetIntervalInSeconds return the interval in seconds for frequent collector while the change detection frequency is specified
func (collector *FrequentCollector) GetIntervalInSeconds(changeDetectionFrequency int, parsedExpression scheduleexpression.ScheduleExpression) (bool, int) {
	if changeDetectionFrequency > 1 {
		//get the rate in seconds
		currentTime := time.Now()
		nextTime := parsedExpression.Next(currentTime)
		assocInterval := nextTime.Sub(currentTime).Seconds()

		//calculate the interval in seconds
		intervalSeconds := assocInterval / float64(changeDetectionFrequency)
		return true, int(math.Max(intervalSeconds, minIntervalInSeconds))
	} else {
		return false, 0
	}
}

//getFrequentCollectInformation return frequent collector's frequency and watched inventory types.
func (collector *FrequentCollector) getFrequentCollectInformation(context context.T, docState *contracts.DocumentState) (changeDetectionFrequency int, listOfGathererNames []string) {
	var pluginState *contracts.PluginState = collector.getInventoryPluginState(docState)
	log := context.Log()
	if pluginState != nil {
		output := iohandler.NewDefaultIOHandler(context, docState.IOConfig)
		parameterMap := pluginutil.LoadParametersAsMap(log, pluginState.Configuration.Properties, output)
		gathererParameterMap := collector.getGathererParameterMap()

		// parse the "changeDetectionFrequency"
		strchangeDetectionFrequency, paramFrequencyPresent := parameterMap[paramChangeDetectionFrequency]
		if !paramFrequencyPresent {
			log.Debugf("frequent collector, param '%s' doesn't exist.", paramChangeDetectionFrequency)
			return 0, listOfGathererNames
		}

		intChangeDetectionFrequency, parseFrequencyErr := strconv.Atoi(strchangeDetectionFrequency.(string))
		if parseFrequencyErr != nil {
			return 0, listOfGathererNames
		}

		// parse the "changeDetectionTypes"
		watchedTypes, paramTypesPresent := parameterMap[paramChangeDetectionTypes]
		if !paramTypesPresent {
			log.Debugf("frequent collector, param '%s' doesn't exist", paramChangeDetectionTypes)
			return 0, listOfGathererNames
		}

		log.Debugf("parameter: changeDetectionTypes : %#v", watchedTypes)
		for _, v := range watchedTypes.([]interface{}) {
			var gathererName = v.(string)
			log.Debugf("watched gathererName: %s ", gathererName)

			parameterName, gathererPresent := gathererParameterMap[strings.ToLower(gathererName)]
			if !gathererPresent {
				continue
			}

			if valueOfItem, itemPresent := parameterMap[parameterName]; itemPresent {
				log.Debugf("watched inventory type : %s, valueOfItem: %s", parameterName, valueOfItem.(string))
				if strings.EqualFold(valueOfItem.(string), "Enabled") {
					listOfGathererNames = append(listOfGathererNames, gathererName)
				}
			}
		}

		log.Infof("changeDetectionFrequency: %d, listOfGathererNames: %#v", intChangeDetectionFrequency, listOfGathererNames)
		return intChangeDetectionFrequency, listOfGathererNames
	}

	return 0, listOfGathererNames
}

//getInventoryPluginState return the pointer to the inventory plugin state
func (collector *FrequentCollector) getInventoryPluginState(docState *contracts.DocumentState) *contracts.PluginState {
	for _, plugin := range docState.InstancePluginsInformation {
		if plugin.Name == appconfig.PluginNameAwsSoftwareInventory {
			return &plugin
		}
	}
	return nil
}

//IsSoftwareInventoryAssociation return true if it's a software inventory association.
func (collector *FrequentCollector) IsSoftwareInventoryAssociation(docState *contracts.DocumentState) bool {
	return collector.getInventoryPluginState(docState) != nil
}

//collect collects the dirty inventory types and report to SSM if there's any
func (collector *FrequentCollector) collect(context context.T, docState *contracts.DocumentState) {
	log := context.Log()

	if inventoryPlugin, err := inventory.NewPlugin(context); err == nil {
		output := iohandler.NewDefaultIOHandler(context, docState.IOConfig)
		_, inventoryTypes := collector.getFrequentCollectInformation(context, docState)
		gathererMap := collector.getGatherersForFrequentCollectTypes(context, docState, inventoryPlugin, inventoryTypes)
		log.Debugf("show the value of gatherer map for frequent collector : %#v ", *gathererMap)

		if len(*gathererMap) > 0 {
			log.Debug("calling Inventory.Plugin.ApplyInventoryFrequentCollector")
			inventoryPlugin.ApplyInventoryFrequentCollector(*gathererMap, output)
		} else {
			log.Infof("no enabled gatherer found for frequent collector")
		}
	} else {
		log.Errorf("failed to create inventory plugin, error: %#v, plugin : %#v", err, inventoryPlugin)
	}
}

//getGatherersForFrequentCollectTypes return the map of gatherers and InventoryModel.Config
func (collector *FrequentCollector) getGatherersForFrequentCollectTypes(context context.T, docState *contracts.DocumentState, plugin *inventory.Plugin, namesOfItems []string) *map[gatherers.T]InventoryModel.Config {
	log := context.Log()
	var gatherersMap = make(map[gatherers.T]InventoryModel.Config)
	supportedGathererNameMap := collector.getSupportedGathererNames()

	var pluginState *contracts.PluginState = collector.getInventoryPluginState(docState)
	if pluginState == nil {
		return &gatherersMap
	}

	validGatherers := collector.getValidInventoryGathererConfigMap(context, docState, plugin)
	log.Debugf("valid inventory gatherer : %#v", validGatherers)
	if len(validGatherers) == 0 {
		log.Infof("Number of valid inventory gathers is 0")
		return &gatherersMap
	}

	for _, inventoryTypeName := range namesOfItems {
		if gathererName, gathererNamePresent := supportedGathererNameMap[strings.ToLower(inventoryTypeName)]; gathererNamePresent {
			if gatherer, gathererPresent := plugin.GetSupportedGatherer(gathererName); gathererPresent {
				if modelConfig, configPresent := validGatherers[gatherer]; configPresent {
					gatherersMap[gatherer] = modelConfig
				}
			}
		}
	}

	return &gatherersMap
}

//getValidInventoryGathererConfigMap return the map of valid gatherers and their InventoryModel.config
func (collector *FrequentCollector) getValidInventoryGathererConfigMap(context context.T, docState *contracts.DocumentState, plugin *inventory.Plugin) (validGatherers map[gatherers.T]InventoryModel.Config) {
	log := context.Log()
	var dataB []byte
	var err error
	var inventoryInput inventory.PluginInput
	var pluginState *contracts.PluginState = collector.getInventoryPluginState(docState)

	if pluginState == nil {
		return
	}

	if dataB, err = json.Marshal(pluginState.Configuration.Properties); err != nil {
		log.Error("error occurred while json.Marshal(pluginState.Configuration.Properties)")
		return
	}

	if err = json.Unmarshal(dataB, &inventoryInput); err != nil {
		log.Error("error occurred while json.Unmarshal(dataB, &inventoryInput)")
		return
	}
	log.Debugf("unmarshal channel response: %v", inventoryInput)
	validGatherers, err = plugin.ValidateInventoryInput(context, inventoryInput)
	return
}

//getSupportedGathererNames returns a map of gatherer names, using the all lower case as key, the normal gatherer name as value. This is to allow customer to input gatherer name ignoring case.
func (collector *FrequentCollector) getSupportedGathererNames() map[string]string {
	var gathererNameMap = make(map[string]string)
	//only "AWS:Applications" is supported for now
	gathererNameMap[strings.ToLower(application.GathererName)] = application.GathererName
	return gathererNameMap
}

//getGathererParameterMap returns a map taking gatherer name as key and document parameter as value, used to check if the gatherer is enabled in plugin state.
func (collector *FrequentCollector) getGathererParameterMap() map[string]string {
	var paramGathererMap = make(map[string]string)
	paramGathererMap[strings.ToLower(application.GathererName)] = "applications"
	paramGathererMap[strings.ToLower(awscomponent.GathererName)] = "awsComponents"
	paramGathererMap[strings.ToLower(file.GathererName)] = "files"
	paramGathererMap[strings.ToLower(network.GathererName)] = "networkConfig"
	paramGathererMap[strings.ToLower(windowsUpdate.GathererName)] = "windowsUpdates"
	paramGathererMap[strings.ToLower(service.GathererName)] = "services"
	paramGathererMap[strings.ToLower(registry.GathererName)] = "windowsRegistry"
	paramGathererMap[strings.ToLower(role.GathererName)] = "windowsRoles"
	paramGathererMap[strings.ToLower(instancedetailedinformation.GathererName)] = "instanceDetailedInformation"
	return paramGathererMap
}
