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
	"unicode/utf8"

	"github.com/aws/amazon-ssm-agent/agent/log"
)

const (
	gettingPlatformDetailsMessage = "getting platform details"
	notAvailableMessage           = "NotAvailable"
	commandOutputMessage          = "Command output %v"
)

var getPlatformNameFn = getPlatformName

// IsPlatformWindowsServer2012OrEarlier represents whether it is Windows 2012 and earlier or not
func IsPlatformWindowsServer2012OrEarlier(log log.T) (bool, error) {
	return isPlatformWindowsServer2012OrEarlier(log)
}

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

// IP of the network interface
func IP() (selected string, err error) {
	if addrs, err := net.InterfaceAddrs(); err == nil {
		candidates := make([]net.IP, 0)
		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPAddr:
				candidates = append(candidates, v.IP.To4())
				candidates = append(candidates, v.IP.To16())
			case *net.IPNet:
				candidates = append(candidates, v.IP.To4())
				candidates = append(candidates, v.IP.To16())
			}
		}
		selectedIp, err := selectIp(candidates)
		if err == nil {
			selected = selectedIp.String()
		}
	} else {
		err = fmt.Errorf("failed to load network interfaces: %v", err)
	}

	if err != nil {
		err = fmt.Errorf("failed to determine IP address: %v", err)
	}

	return
}

// Selects a single IP address to be reported for this instance.
func selectIp(candidates []net.IP) (result net.IP, err error) {
	for _, ip := range candidates {
		if ip != nil && !ip.IsUnspecified() {
			if result == nil {
				result = ip
			} else if isLoopbackOrLinkLocal(result) {
				// Prefer addresses that are not loopbacks or link-local
				if !isLoopbackOrLinkLocal(ip) {
					result = ip
				}
			} else if !isLoopbackOrLinkLocal(ip) {
				// Among addresses that are not loopback or link-local, prefer IPv4
				if !isIpv4(result) && isIpv4(ip) {
					result = ip
				}
			}
		}
	}

	if result == nil {
		err = fmt.Errorf("no IP addresses found")
	}

	return
}

func isLoopbackOrLinkLocal(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}

func isIpv4(ip net.IP) bool {
	return ip.To4() != nil
}

func IsPlatformNanoServer(log log.T) (bool, error) {
	return isPlatformNanoServer(log)
}
