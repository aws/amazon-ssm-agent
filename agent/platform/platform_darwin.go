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
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/log"
)

const (
	platformDetailsCommand = "sw_vers"
	errorOccurredMessage   = "There was an error running %v, err: %v"
)

func getPlatformName(log log.T) (value string, err error) {
	value, err = getPlatformDetail(log, "-productName")
	log.Debugf("platform name: %v", value)
	return
}

func getPlatformType(log log.T) (value string, err error) {
	return "linux", nil
}

func getPlatformVersion(log log.T) (value string, err error) {
	value, err = getPlatformDetail(log, "-productVersion")
	log.Debugf("platform version: %v", value)
	return
}

func getPlatformSku(log log.T) (value string, err error) {
	return
}

func getPlatformDetail(log log.T, param string) (value string, err error) {
	var contentsBytes []byte
	value = notAvailableMessage

	log.Debugf(gettingPlatformDetailsMessage)
	if contentsBytes, err = exec.Command(platformDetailsCommand, param).Output(); err != nil {
		log.Debugf(errorOccurredMessage, platformDetailsCommand, err)
		return
	}
	value = strings.TrimSpace(string(contentsBytes))
	log.Debugf(commandOutputMessage, value)
	return
}

var hostNameCommand = filepath.Join("/bin", "hostname")

// fullyQualifiedDomainName returns the Fully Qualified Domain Name of the instance, otherwise the hostname
func fullyQualifiedDomainName(log log.T) string {
	var hostName, fqdn string
	var err error

	if hostName, err = os.Hostname(); err != nil {
		return ""
	}

	var contentBytes []byte
	if contentBytes, err = exec.Command(hostNameCommand, "-f").Output(); err == nil {
		fqdn = string(contentBytes)
		//trim whitespaces - since by default above command appends '\n' at the end.
		//e.g: 'ip-172-31-7-113.ec2.internal\n'
		fqdn = strings.TrimSpace(fqdn)
	}

	if fqdn != "" {
		return fqdn
	}

	return strings.TrimSpace(hostName)
}

func isPlatformNanoServer(log log.T) (bool, error) {
	return false, nil
}
