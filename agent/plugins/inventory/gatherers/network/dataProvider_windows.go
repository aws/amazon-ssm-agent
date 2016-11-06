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

// +build windows

// Package network contains a network gatherer.
package network

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
)

var validIPV4Address *regexp.Regexp

const (
	cmd                                       = "powershell"
	cmdArgsToGetFullDetailsForGivenMacAddress = `Get-wmiobject -class Win32_NetworkAdapterConfiguration | where-object {$_.MACAddress -eq "%s"} | Select-object @{Name="IPAddresses";Expression={$_.IPAddress}}, @{Name="DefaultIPGateway";Expression={$_.DefaultIPGateway}}, @{Name="MacAddress";Expression={$_.MACAddress}}, @{Name="DHCPServer";Expression={$_.DHCPServer}}, @{Name="DNSServers";Expression={$_.DNSServerSearchOrder}} ,@{Name="IPSubnet";Expression={$_.IPSubnet}} | ConvertTo-Json`

	//We list only ethernet & wireless type of network interfaces. For more details refer to https://msdn.microsoft.com/en-us/library/aa394217%28v=vs.85%29.aspx
	cmdArgsToGetListAllInterfaces = `Get-wmiobject -class Win32_NetworkAdapter | where-object {$_.AdapterTypeID -eq 0 -or $_.AdapterTypeID -eq 9} | Select-object @{Name="MACAddress";Expression={$_.MACAddress}}, @{Name="Description";Expression={$_.Description}}, @{Name="ProductName";Expression={$_.ProductName}}| ConvertTo-Json`
	regexForIpV4Addresses         = `^(?:[0-9]{1,3}\.){3}[0-9]{1,3}$`
)

func init() {
	validIPV4Address = regexp.MustCompile(regexForIpV4Addresses)
}

// NetworkInterfaceConfiguration captures advanced info related to a network interface that we get from
// Win32_NetworkAdapterConfiguration class in windows.
//
// Note: Since Go doesn't provide an easy way to get information like DHCP server address, DNS server address, Default IP
// Gateway, Subnet Mask for a given network interface - we use powershell command in windows to get that data using
// Win32_NetworkAdapterConfiguration class - https://msdn.microsoft.com/en-us/library/aa394217%28v=vs.85%29.aspx
//
// Following fields are of interest to us:
// - DHCPServer (type string)
// - DNSServerSearchOrder (type string array)
// - DefaultIPGateway (type string array)
// - IPSubnet (type string array)
//
// In order to successfully read the data - certain fields are defined as interface{}. That's because if there is only
// 1 item in the string array - the command output shows the json object as a string instead of an array. If there are
// multiple entries in the array then the data shows up as a jsonMap with fields similar to Format.
//
// Since inventory.NetworkData supports only string attributes, we only pick the 1st entry in string array.
// E.g: If there are 2 DefaultIPGateway - ['a.b.c.d','a.b.c.e'] -> we will show 'a.b.c.d' as Gateway in
// inventory.NetworkData for that interface. This could change in future.
// TODO: current implementation only allows 1 dns server address, gateway address per network interface
// however, this may not be the case in reality. We might change this behavior to allow comma separated values or even
// have them as an array.
type NetworkInterfaceConfiguration struct {
	MacAddress       string
	DHCPServer       string
	IPAddresses      interface{}
	DNSServers       interface{}
	IPSubnet         interface{}
	DefaultIPGateway interface{}
}

// Format captures fields required for parsing windows command output.
// A string array of wmi class - when converted to json in powershell shows the fields mentioned here.
type Format struct {
	Count int
	Value []string
}

// NwInterface captures all relevant network interfaces from Win32_NetworkAdapter class in windows.
//
// NOTE: amazon-ssm-agent uses GoLang 1.5 version. There is a known issue in net package where
// interface.HardwareAddr.String() appends 00:00 at the end on windows platform. Until we move to version > 1.6, we will
// use Win32_NetworkAdapter class to get information regarding all network interfaces in windows. For more details
// please refer to following link: https://msdn.microsoft.com/en-us/library/aa394216%28v=vs.85%29.aspx
type NwInterface struct {
	MACAddress  string
	Description string
	ProductName string
}

// decoupling exec.Command for easy testability
var cmdExecutor = executeCommand

func executeCommand(command string, args ...string) ([]byte, error) {
	return exec.Command(command, args...).CombinedOutput()
}

func convertToNetworkData(nwInterface NwInterface) (basicNwData model.NetworkData) {

	basicNwData.Name = nwInterface.ProductName
	basicNwData.MacAddress = nwInterface.MACAddress

	return
}

// CollectNetworkData collects network information for all relevant network interfaces in windows using powershell
func CollectNetworkData(context context.T) (data []model.NetworkData) {
	var output, dataB []byte
	var err error
	var singleInterface NwInterface
	var multipleInterfaces []NwInterface

	log := context.Log()

	log.Infof("Collecting all networking interfaces by executing command:\n%v %v", cmd, cmdArgsToGetListAllInterfaces)

	if output, err = cmdExecutor(cmd, cmdArgsToGetListAllInterfaces); err == nil {
		cmdOutput := string(output)
		log.Debugf("Command output: %v", cmdOutput)

		//windows command can either return a single network interface or an array of network interfaces
		if err = json.Unmarshal(output, &singleInterface); err == nil {

			data = append(data, convertToNetworkData(singleInterface))

		} else if err = json.Unmarshal(output, &multipleInterfaces); err == nil {

			for _, nwInterface := range multipleInterfaces {
				data = append(data, convertToNetworkData(nwInterface))
			}

		} else {
			log.Infof("Unable to get network interface info because of unexpected command output - %v",
				cmdOutput)
			return
		}

		dataB, _ = json.Marshal(data)
		log.Debugf("Basic network interface data collected so far: %v", jsonutil.Indent(string(dataB)))

		//collecting advanced network information for those interfaces
		data = GetAdvancedNetworkData(context, data)

	} else {
		log.Debugf("Failed to execute command : %v %v with error - %v",
			cmd,
			cmdArgsToGetListAllInterfaces,
			err.Error())
		log.Errorf("Command failed with error: %v", string(output))
		log.Infof("Unable to get network data on windows platform")
	}

	return
}

// GetAdvancedNetworkData returns advanced network information in windows platform using powershell commands
func GetAdvancedNetworkData(context context.T, data []model.NetworkData) []model.NetworkData {
	var dataB []byte
	var modifiedData []model.NetworkData
	log := context.Log()

	for _, datum := range data {

		dataB, _ = json.Marshal(datum)
		log.Debugf("Network interface information of - %v: \n%v", datum.Name, string(dataB))

		datum = GetNetworkDataUsingPowershell(context, datum)

		modifiedData = append(modifiedData, datum)
	}

	dataB, _ = json.Marshal(modifiedData)
	log.Debugf("Modified Network Interface information - %v", string(dataB))

	return modifiedData
}

// GetNetworkDataUsingPowershell gets network data by executing powershell command
func GetNetworkDataUsingPowershell(context context.T, networkData model.NetworkData) model.NetworkData {

	var dataB, output []byte
	var err error

	log := context.Log()

	commandArgs := fmt.Sprintf(cmdArgsToGetFullDetailsForGivenMacAddress, networkData.MacAddress)
	log.Debugf("Powershell command being run - %v", commandArgs)

	log.Infof("Executing command: %v %v", cmd, commandArgs)

	if output, err = cmdExecutor(cmd, commandArgs); err == nil {
		cmdOutput := string(output)
		log.Debugf("Command output: %v", cmdOutput)

		if networkData, err = EditNetworkData(log, networkData, cmdOutput); err == nil {
			dataB, _ = json.Marshal(networkData)
			log.Debugf("Modified Network Interface information - %v", string(dataB))
		} else {
			log.Errorf("Unable to add further information to network data because of error - %v", err.Error())
			log.Infof("No modification to network data")
		}
	} else {
		log.Debugf("Failed to execute command : %v %v with error - %v",
			cmd,
			commandArgs,
			err.Error())
		log.Errorf("Command failed with error: %v", string(output))
		log.Infof("No modification to network data")
	}

	return networkData
}

// EditNetworkData returns the modified set of data after parsing the command output. In case it fails to parse the data,
// it returns the unmodified data.
func EditNetworkData(log log.T, data model.NetworkData, cmdOutput string) (model.NetworkData, error) {
	var dataB []byte
	var err error
	var config NetworkInterfaceConfiguration
	var dnsServerAddress, gatewayAddress, subnetMask, ipV4, ipV6 string

	dataB, _ = json.Marshal(data)

	//trim spaces
	str := strings.TrimSpace(cmdOutput)

	if err = json.Unmarshal([]byte(str), &config); err != nil {
		err = fmt.Errorf("Failed to read data from powershell command output because of error - %v", err.Error())
		return data, nil
	}

	dataB, _ = json.Marshal(config)
	log.Debugf("Advanced network data of macaddress - %v - \n%v", data.MacAddress, jsonutil.Indent(string(dataB)))

	data.DHCPServer = config.DHCPServer

	if gatewayAddress, err = GetParsedData(log, config.DefaultIPGateway); err != nil {
		log.Debugf("Unable to get gateway address for macaddress - %v due to error - %v", data.MacAddress, err.Error())
	} else {
		data.Gateway = gatewayAddress
	}

	if dnsServerAddress, err = GetParsedData(log, config.DNSServers); err != nil {
		log.Debugf("Unable to get dns server for macaddress - %v due to error - %v", data.MacAddress, err.Error())
	} else {
		data.DNSServer = dnsServerAddress
	}

	if subnetMask, err = GetParsedData(log, config.IPSubnet); err != nil {
		log.Debugf("Unable to get gateway address for macaddress - %v due to error - %v", data.MacAddress, err.Error())
	} else {
		data.SubnetMask = subnetMask
	}

	if ipV4, ipV6, err = GetIPAddresses(log, config.IPAddresses); err != nil {
		log.Debugf("Unable to get ip addresses for macaddress - %v due to error - %v", data.MacAddress, err.Error())
	} else {
		data.IPV4 = ipV4
		data.IPV6 = ipV6
	}

	dataB, _ = json.Marshal(data)
	log.Debugf("logging modified data - %v", string(dataB))

	return data, nil
}

// GetParsedData parses the command output and returns the parsedOutput.
//
// NOTE: Parsing logic is specific to the command that is executed. Any change in the command should follow changes here
// too.
func GetParsedData(log log.T, input interface{}) (parsedOutput string, err error) {
	//Note: As per the link - https://msdn.microsoft.com/en-us/library/aa394217%28v=vs.85%29.aspx
	// fields like DNSServerSearchOrder, DefaultGateway are string array. However, on just 1 entry - the data ends up
	// showing as a string. If there are multiple entries - ConvertTo-Json - makes it a map with fields similar to
	// Format struct.

	// there are only 2 possibilities - either given input is a string or a json map with fields similar to Format struct.
	// anything else means - the command executed to get the data has been changed.

	errorMsg := "Unable to read more data from %v due to error - %v"

	if str, possible := input.(string); possible {
		log.Debugf("Input %v can be transformed into string", input)
		parsedOutput = str
	} else {
		log.Debugf("Input %v can't be transformed into string", input)
		var format Format
		dataB, _ := json.Marshal(input)

		if err = json.Unmarshal(dataB, &format); err != nil {
			err = fmt.Errorf(errorMsg, input, err.Error())
		} else {

			//verify if format.Value is not nil
			if len(format.Value) > 0 {
				//currently we return 1st element of string array - since DNSServer and Gateway is string
				//if that changes then we can return format.Value
				parsedOutput = format.Value[0]
			} else {
				err = fmt.Errorf("Unexpected data format")
			}
		}
	}

	log.Debugf("ParsedOutput - %v, error - %v", parsedOutput, err)
	return
}

// GetIPAddresses parses the command output and returns the parsedOutput.
//
// NOTE: Parsing logic is specific to the command that is executed. Any change in the command should follow changes here
// too.
func GetIPAddresses(log log.T, input interface{}) (ipV4, ipV6 string, err error) {
	// Note: As per the link - https://msdn.microsoft.com/en-us/library/aa394217%28v=vs.85%29.aspx IPAddress is
	// string array. However, on just 1 entry - the data ends up showing as a string. If there are multiple entries
	// ConvertTo-Json - makes it a map with fields similar to Format struct.

	// there are only 2 possibilities - either given input is a string or a json map with fields similar to Format
	// struct. Anything else means - the command executed to get the data has been changed.

	var ipV4Addresses, ipV6Addresses []string
	errorMsg := "Unable to read more data from %v due to error - %v"

	log.Debugf("Parsing ip addresses from %v", input)

	if str, possible := input.(string); possible {
		log.Debugf("Input %v can be transformed into string", input)

		if validIPV4Address.MatchString(str) {
			ipV4Addresses = append(ipV4Addresses, str)
		}
	} else {
		log.Debugf("Input %v can't be transformed into string", input)
		var format Format
		dataB, _ := json.Marshal(input)

		if err = json.Unmarshal(dataB, &format); err != nil {
			err = fmt.Errorf(errorMsg, input, err.Error())
		} else {

			//verify if format.Value is not nil
			if len(format.Value) > 0 {
				for _, value := range format.Value {
					if validIPV4Address.MatchString(value) {
						ipV4Addresses = append(ipV4Addresses, value)
					} else {
						ipV6Addresses = append(ipV6Addresses, value)
					}
				}
			} else {
				err = fmt.Errorf("Unexpected data format")
			}
		}
	}

	//all ip addresses returned for a network interface will be returned as a ',' separated string
	ipV4 = strings.Join(ipV4Addresses, ",")
	ipV6 = strings.Join(ipV6Addresses, ",")

	log.Debugf("IPV4 - %v, IPV6 - %v, error - %v", ipV4, ipV6, err)
	return
}
