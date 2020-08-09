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
	"strings"
	"unicode/utf8"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/log/ssmlog"
)

const (
	gettingPlatformDetailsMessage = "getting platform details"
	notAvailableMessage           = "NotAvailable"
	commandOutputMessage          = "Command output %v"
)

var awsRegionServiceDomainMap = map[string]string{
	"ap-east-1":      "amazonaws.com",
	"ap-northeast-1": "amazonaws.com",
	"ap-northeast-2": "amazonaws.com",
	"ap-south-1":     "amazonaws.com",
	"ap-southeast-1": "amazonaws.com",
	"ap-southeast-2": "amazonaws.com",
	"ca-central-1":   "amazonaws.com",
	"cn-north-1":     "amazonaws.com.cn",
	"cn-northwest-1": "amazonaws.com.cn",
	"eu-central-1":   "amazonaws.com",
	"eu-north-1":     "amazonaws.com",
	"eu-west-1":      "amazonaws.com",
	"eu-west-2":      "amazonaws.com",
	"eu-west-3":      "amazonaws.com",
	"me-south-1":     "amazonaws.com",
	"sa-east-1":      "amazonaws.com",
	"us-east-1":      "amazonaws.com",
	"us-east-2":      "amazonaws.com",
	"us-gov-east-1":  "amazonaws.com",
	"us-gov-west-1":  "amazonaws.com",
	"us-west-1":      "amazonaws.com",
	"us-west-2":      "amazonaws.com",
}

var getPlatformNameFn = getPlatformName

// PlatformName gets the OS specific platform name.
func PlatformName(log log.T) (name string, err error) {
	name, err = getPlatformNameFn(log)
	if err != nil {
		return
	}
	platformName := ""
	for i := range name {
		runeVal, _ := utf8.DecodeRuneInString(name[i:])
		if runeVal == utf8.RuneError {
			// runeVal = rune(value[i]) - using this will convert \xa9 to valid unicode code point
			continue
		}
		platformName = platformName + fmt.Sprintf("%c", runeVal)
	}
	return platformName, nil
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

// getDefaultEndPoint returns the default endpoint for a service, it should be empty unless it's a china region
func GetDefaultEndPoint(region string, service string) string {
	log := ssmlog.SSMLogger(true)
	endpoint := ""

	if val, ok := awsRegionServiceDomainMap[region]; ok {
		endpoint = val
	} else {
		dynamicServiceDomain, err := dynamicData.ServiceDomain()
		if dynamicServiceDomain != "" {
			endpoint = dynamicServiceDomain
			log.Infof("Service Endpoint found from metadata service: %v", endpoint)
		} else {
			log.Warnf("Error when getting Service Domain dynamically: %v", err)
			if strings.HasPrefix(region, "cn-") {
				endpoint = "amazonaws.com.cn"
			}
		}
	}

	if endpoint == "" || endpoint == "amazonaws.com" {
		return ""
	} else {
		return getServiceEndpoint(region, service, endpoint)
	}
}

func getServiceEndpoint(region string, service string, endpoint string) string {
	return service + "." + region + "." + endpoint
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
