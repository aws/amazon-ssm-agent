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

// Package platform contains platform specific utilities.
package platform

import (
	"fmt"
	"net"
	"sort"

	"github.com/aws/amazon-ssm-agent/agent/log"
)

const (
	gettingPlatformDetailsMessage = "getting platform details"
	notAvailableMessage           = "NotAvailable"
	commandOutputMessage          = "Command output %v"
)

// PlatformName gets the OS specific platform name.
func PlatformName(log log.T) (name string, err error) {
	return getPlatformName(log)
}

// PlatformType gets the OS specific platform type, valid values are windows and linux.
func PlatformType(log log.T) (name string, err error) {
	return getPlatformType(log)
}

// PlatformVersion gets the OS specific platform version.
func PlatformVersion(log log.T) (version string, err error) {
	return getPlatformVersion(log)
}

// PlatformSku gets the OS specific platform SKU number
func PlatformSku(log log.T) (sku string, err error) {
	return getPlatformSku(log)
}

// Hostname of the computer.
func Hostname(log log.T) (name string, err error) {
	return fullyQualifiedDomainName(log), nil
}

// IP of the network interface
func IP() (ip string, err error) {

	var interfaces []net.Interface
	if interfaces, err = net.Interfaces(); err != nil {
		return "", fmt.Errorf("Failed to load network interfaces. %v", err)
	}

	interfaces = filterInterface(interfaces)
	sort.Sort(byIndex(interfaces))

	var foundIP net.IP

	// search for IPv4
	for _, i := range interfaces {
		var addrs []net.Addr
		if addrs, err = i.Addrs(); err != nil {
			continue
		}
		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPAddr:
				foundIP = v.IP.To4()
			case *net.IPNet:
				foundIP = v.IP.To4()
			}

			if foundIP != nil {
				return foundIP.String(), nil
			}
		}
	}

	// search for IPv6
	for _, i := range interfaces {
		var addrs []net.Addr
		if addrs, err = i.Addrs(); err != nil {
			continue
		}
		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPAddr:
				foundIP = v.IP.To16()
			case *net.IPNet:
				foundIP = v.IP.To16()
			}

			if foundIP != nil {
				return foundIP.String(), nil
			}
		}
	}

	return "", fmt.Errorf("No IP addresses found.")
}

// filterInterface removes interface that's not up or is a loopback/p2p
func filterInterface(interfaces []net.Interface) (i []net.Interface) {
	for _, v := range interfaces {
		if (v.Flags&net.FlagUp != 0) && (v.Flags&net.FlagLoopback == 0) && (v.Flags&net.FlagPointToPoint == 0) {
			i = append(i, v)
		}
	}
	return
}

// byIndex implements sorting for net.Interface.
type byIndex []net.Interface

func (b byIndex) Len() int           { return len(b) }
func (b byIndex) Less(i, j int) bool { return b[i].Index < b[j].Index }
func (b byIndex) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }

func IsPlatformNanoServer(log log.T) (bool, error) {
	return isPlatformNanoServer(log)
}
