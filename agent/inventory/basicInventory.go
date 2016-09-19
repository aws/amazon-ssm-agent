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

// Package inventory contains routines that periodically updates basic instance inventory to Inventory service
package inventory

import (
	"encoding/json"
	"fmt"
	"reflect"
	"runtime"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
	"github.com/aws/amazon-ssm-agent/agent/version"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/carlescere/scheduler"
)

const (
	errorThreshold = 10
	name           = "BasicInventory"
	enabled        = "Enabled"

	// TODO: we might have schemaVersion per inventory type - e.g schemaVersion of AWS:Applications might be
	// different than AWS:File
	schemaVersionOfInventoryItem = "1.0"
)

// BasicInventoryProvider encapsulates the logic of configuring, starting and stopping basic inventory plugin
type BasicInventoryProvider struct {
	context            context.T
	frequencyInMinutes int
	stopPolicy         *sdkutil.StopPolicy
	updateJob          *scheduler.Job
	ssm                *ssm.SSM
	//isEnabled enables basic inventory gatherer, if this is false - then basic inventory gatherer will not run.
	isEnabled bool
	//isOptimizerEnabled ensures PutInventory API is not called if same data is sent, if this is false - then even
	//if instanceInfo is same, every 5 mins data will be sent to SSM Inventory.
	isOptimizerEnabled bool
}

// NewBasicInventoryProvider creates a new basic inventory provider core plugin.
func NewBasicInventoryProvider(context context.T) (*BasicInventoryProvider, error) {
	var appCfg appconfig.SsmagentConfig
	var err error
	var provider = BasicInventoryProvider{}

	c := context.With("[" + name + "]")
	log := c.Log()

	// reading agent appconfig
	if appCfg, err = appconfig.Config(false); err != nil {
		log.Errorf("Could not load config file %v", err.Error())
		return &provider, err
	}

	// setting ssm client config
	cfg := sdkutil.AwsConfig()
	cfg.Region = &appCfg.Agent.Region
	cfg.Endpoint = &appCfg.Ssm.Endpoint

	//setting basic inventory config
	provider.isEnabled = appCfg.Ssm.BasicInventoryGatherer == enabled
	provider.isOptimizerEnabled = appCfg.Ssm.InventoryOptimizer == enabled

	provider.context = c
	provider.stopPolicy = sdkutil.NewStopPolicy(name, errorThreshold)
	provider.ssm = ssm.New(session.New(cfg))
	//for now we are using the same frequency as that of health plugin to send inventory data
	provider.frequencyInMinutes = appCfg.Ssm.HealthFrequencyMinutes

	return &provider, nil
}

//TODO: add this as a utility so that other gatherers will also be able to use this.
func (b *BasicInventoryProvider) ConvertToMap(input interface{}) map[string]*string {
	log := b.context.Log()

	ip := reflect.ValueOf(input)

	if ip.Kind() == reflect.Ptr {
		ip = ip.Elem()
	}

	var key string
	m := make(map[string]*string)

	//iterating over all fields
	for i := 0; i < ip.NumField(); i++ {

		// gets struct's i'th field type
		structField := ip.Type().Field(i)

		// gets struct's i'th field
		v := ip.Field(i)

		key = structField.Name

		//gets the value of the field
		itr := v.Interface()

		value := reflect.ValueOf(itr)
		if value.Kind() == reflect.Ptr {
			// get the value instead
			value = value.Elem()
		}

		switch value.Kind() {
		case reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Float32, reflect.Float64:
			random := fmt.Sprintf("%v", value.Interface())
			m[key] = &random
		default:
			//TODO: for this to be generic - we need to support string conversion of
			//complex data types as well - e.g struct, map etc.
			log.Debugf("String conversion for complex types not yet supported - hence skipping.")
		}
	}

	return m
}

// GetInstanceInformation returns the latest set of instance information
func GetInstanceInformation(context context.T) (InstanceInformation, error) {

	var instanceInfo InstanceInformation

	log := context.Log()

	instanceInfo.AgentStatus = *aws.String(AgentStatus)
	instanceInfo.AgentVersion = *aws.String(version.Version)

	//TODO: detecting OS can be added as an utility.
	goOS := runtime.GOOS
	switch goOS {
	case updateutil.PlatformWindows:
		instanceInfo.PlatformType = *aws.String(ssm.PlatformTypeWindows)
	case updateutil.PlatformLinux:
		instanceInfo.PlatformType = *aws.String(ssm.PlatformTypeLinux)
	default:
		return instanceInfo, fmt.Errorf("Cannot report platform type of unrecognized OS. %v", goOS)
	}

	if ip, err := platform.IP(); err == nil {
		instanceInfo.IPAddress = *aws.String(ip)
	} else {
		log.Warn(err)
	}

	if h, err := platform.Hostname(); err == nil {
		instanceInfo.ComputerName = *aws.String(h)
	} else {
		log.Warn(err)
	}
	if instID, err := platform.InstanceID(); err == nil {
		instanceInfo.InstanceId = *aws.String(instID)
	} else {
		log.Warn(err)
	}

	if n, err := platform.PlatformName(log); err == nil {
		instanceInfo.PlatformName = *aws.String(n)
	} else {
		log.Warn(err)
	}

	if v, err := platform.PlatformVersion(log); err == nil {
		instanceInfo.PlatformVersion = *aws.String(v)
	} else {
		log.Warn(err)
	}

	return instanceInfo, nil
}

// instanceInformationInventoryItem returns latest instance information inventory item
func (b *BasicInventoryProvider) instanceInformationInventoryItem() (Item, error) {
	var err error
	var data InstanceInformation
	var item Item

	if data, err = GetInstanceInformation(b.context); err == nil {
		//CaptureTime must comply with format: 2016-07-30T18:15:37Z or else it will throw error
		t := time.Now().UTC()
		time := t.Format(time.RFC3339)

		item = Item{
			Name:          AWSInstanceInformation,
			Content:       data,
			SchemaVersion: schemaVersionOfInventoryItem,
			//capture time must be in UTC so that formatting to RFC3339 complies with regex at SSM
			CaptureTime: time,
		}
	} else {
		err = fmt.Errorf("Unable to fetch instance information - %v", err.Error())
	}

	return item, err
}

// updateBasicInventory updates basic instance information inventory data in SSM
func (b *BasicInventoryProvider) updateBasicInventory() {
	log := b.context.Log()
	log.Infof("Updating basic inventory information.")

	//get latest instanceInfo inventory item
	i, err := b.instanceInformationInventoryItem()
	if err != nil {
		log.Errorf("Encountered error while fetching instance information - %v", err)
		return
	}

	dataB, _ := json.Marshal(i)

	//Note - behavior of not sending same data again, is customizable. This is only relevant
	//for integrating with awsconfig for now - later this policy will be changed.

	if b.isOptimizerEnabled && !ShouldUpdate(i.Name, string(dataB)) {

		//TODO: there is no checksum field in ssm coral model - so don't send the data now. As soon as checksum
		//is introduced in our coral model - ensure agent sends just the checksum with updated timestamp

		log.Infof("No new instance information data to send to ssm inventory")
	} else {
		//send the data

		var instanceID string
		var err error

		if instanceID, err = platform.InstanceID(); err != nil {
			log.Errorf("Unable to fetch InstanceId, instance information will not be sent to Inventory")
			return
		}

		//set instanceInfo as inventory item
		var content []map[string]*string
		content = append(content, b.ConvertToMap(i.Content))

		//TODO: add the contentHash functionality

		//send inventory data to SSM
		params := &ssm.PutInventoryInput{

			InstanceId: &instanceID,
			Items: []*ssm.InventoryItem{
				{
					CaptureTime:   &i.CaptureTime,
					Content:       content,
					TypeName:      &i.Name,
					SchemaVersion: &i.SchemaVersion,
				},
			},
		}

		log.Debugf("Calling PutInventory API with parameters - %v", params)
		resp, err := b.ssm.PutInventory(params)
		if err != nil {

			//TODO: If API throws ContentHashMismatch Exception -> send the entire data again
			//TODO: If API has other exception -> do reasonable retries and report error.

			log.Errorf("Encountered error while calling PutInventory API %v", err)
		} else {
			log.Debugf("PutInventory was called successfully with response - %v", resp)
		}
	}

	return
}

// ICorePlugin implementation

// Name returns the Plugin Name
func (b *BasicInventoryProvider) Name() string {
	return name
}

// Execute starts the scheduling of the basic inventory plugin
func (b *BasicInventoryProvider) Execute(context context.T) (err error) {

	if b.isEnabled {
		b.context.Log().Debugf("Starting %s plugin", name)
		if b.updateJob, err = scheduler.Every(b.frequencyInMinutes).Minutes().Run(b.updateBasicInventory); err != nil {
			context.Log().Errorf("Unable to schedule basic inventory plugin. %v", err)
		}
	} else {
		b.context.Log().Debugf("Skipping execution of %s plugin since its disabled", name)
	}
	return
}

// RequestStop handles the termination of the basic inventory plugin job
func (b *BasicInventoryProvider) RequestStop(stopType contracts.StopType) (err error) {
	if b.updateJob != nil {
		b.context.Log().Info("Stopping basic inventory job.")
		b.updateJob.Quit <- true
	}
	return nil
}
