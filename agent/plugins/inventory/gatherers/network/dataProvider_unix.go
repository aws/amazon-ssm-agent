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

// +build darwin freebsd linux netbsd openbsd

// Package network contains a network gatherer.
package network

import (
	"encoding/json"
	"net"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
)

// CollectNetworkData collects network information for linux
func CollectNetworkData(context context.T) (data []model.NetworkData) {

	//TODO: collect dhcp, dns server info from dhcp lease
	//TODO: collect gateway addresses (possibly from 'route -n')
	//TODO: collect subnetmask

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
		var networkData model.NetworkData

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

// setNetworkData sets network data using the given interface
func setNetworkData(context context.T, networkInterface net.Interface) model.NetworkData {
	var addresses []net.Addr
	var err error

	log := context.Log()
	var networkData = model.NetworkData{}

	networkData.Name = networkInterface.Name
	networkData.MacAddress = networkInterface.HardwareAddr.String()

	//getting addresses associated with network interface
	if addresses, err = networkInterface.Addrs(); err != nil {
		log.Infof("Can't find address associated with %v", networkInterface.Name)
	} else {
		//TODO: current implementation is tied to inventory model where IPaddress is a string
		//if there are multiple ip addresses attached to an interface - we would overwrite the
		//ipaddresses. This behavior will be changed soon.
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
