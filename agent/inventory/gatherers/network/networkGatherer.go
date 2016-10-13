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

// Package network contains a network gatherer.
package network

import (
	"encoding/json"
	"net"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/inventory/model"
)

const (
	// GathererName captures name of network gatherer
	GathererName = "AWS:Network"
	// SchemaVersionOfApplication represents schema version of network gatherer
	SchemaVersionOfApplication = "1.0"
)

// setNetworkData sets network data using the given interface
func setNetworkData(context context.T, networkInterface net.Interface) inventory.NetworkData {
	var addresses []net.Addr
	var err error

	log := context.Log()
	var networkData = inventory.NetworkData{}

	networkData.Name = networkInterface.Name
	networkData.MacAddress = networkInterface.HardwareAddr.String()

	//getting addresses associated with network interface
	if addresses, err = networkInterface.Addrs(); err != nil {
		log.Infof("Can't find address associated with %v", networkInterface.Name)
	} else {
		//TODO: current implementation is tied to inventory model where IPaddress is a string
		//if there are multiple ip addresses attached to an interface - we would overwrite the
		//ipaddresses. This behavior might be changed.
		for _, addr := range addresses {
			var ip net.IP

			switch v := addr.(type) {
			case *net.IPAddr:
				ip = v.IP
			case *net.IPNet:
				ip = v.IP
			}

			//To4 - return nil if address is not IPV4 address
			//we leverage this to determine if address is IPV4 or IPV6
			v4 := ip.To4()

			if len(v4) == 0 {
				networkData.IPV6 = ip.To16().String()
			} else {
				networkData.IPV4 = v4.String()
			}
		}
	}

	return networkData
}

// GetBasicNetworkData gathers basic network data using go libraries - https://golang.org/pkg/net/
func GetBasicNetworkData(context context.T) (data []inventory.NetworkData) {
	var interfaces []net.Interface
	var err error

	log := context.Log()

	log.Info("Detecting all network interfaces")

	interfaces, err = net.Interfaces()

	if err != nil {
		log.Infof("Unable to get network interface information")
		return
	}

	for _, i := range interfaces {
		var networkData inventory.NetworkData

		if i.Flags&net.FlagLoopback != 0 {
			log.Infof("Ignoring loopback interface")
			continue
		}

		networkData = setNetworkData(context, i)

		dataB, _ := json.Marshal(networkData)

		log.Debugf("Detected interface %v - %v", networkData.Name, string(dataB))
		data = append(data, networkData)
	}

	return
}

// CollectNetworkData collects network information
func CollectNetworkData(context context.T) (data []inventory.NetworkData) {

	var dataB []byte
	log := context.Log()

	data = GetBasicNetworkData(context)

	dataB, _ = json.Marshal(data)
	log.Debugf("Basic set of network Data collected - %v", string(dataB))

	// get advanced network info only if we have some basic info
	if len(data) > 0 {
		data = GetAdvancedNetworkData(context, data)
	}

	dataB, _ = json.Marshal(data)
	log.Debugf("Network Data collected- %v", string(dataB))

	return
}

// T represents network gatherer which implements all contracts for gatherers.
type T struct{}

// Gatherer returns new network gatherer
func Gatherer(context context.T) *T {
	return new(T)
}

// Name returns name of network gatherer
func (t *T) Name() string {
	return GathererName
}

// Run executes network gatherer and returns list of inventory.Item comprising of network data
func (t *T) Run(context context.T, configuration inventory.Config) (items []inventory.Item, err error) {

	var result inventory.Item

	//CaptureTime must comply with format: 2016-07-30T18:15:37Z to comply with regex at SSM.
	currentTime := time.Now().UTC()
	captureTime := currentTime.Format(time.RFC3339)

	result = inventory.Item{
		Name:          t.Name(),
		SchemaVersion: SchemaVersionOfApplication,
		Content:       CollectNetworkData(context),
		CaptureTime:   captureTime,
	}

	items = append(items, result)
	return
}

// RequestStop stops the execution of application gatherer.
func (t *T) RequestStop(stopType contracts.StopType) error {
	var err error
	return err
}
