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

// +build freebsd linux netbsd openbsd

// Package platform contains platform specific utilities.
package platform

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
)

const (
	systemReleaseCommand   = "/etc/system-release"
	redhatReleaseCommand   = "/etc/redhat-release"
	lsbReleaseCommand      = "lsb_release"
	fetchingDetailsMessage = "fetching platform details from %v"
	errorOccurredMessage   = "There was an error running %v, err: %v"
)

func getPlatformName(log log.T) (value string, err error) {
	value, _, err = getPlatformDetails(log)
	return
}

func getPlatformVersion(log log.T) (value string, err error) {
	_, value, err = getPlatformDetails(log)
	return
}

func getPlatformSku(log log.T) (value string, err error) {
	return
}

func getPlatformDetails(log log.T) (name string, version string, err error) {
	log.Debugf(gettingPlatformDetailsMessage)
	contents := ""
	var contentsBytes []byte
	name = notAvailableMessage
	version = notAvailableMessage

	if fileutil.Exists(systemReleaseCommand) {
		log.Debugf(fetchingDetailsMessage, systemReleaseCommand)

		contents, err = fileutil.ReadAllText(systemReleaseCommand)
		log.Debugf(commandOutputMessage, contents)

		if err != nil {
			log.Debugf(errorOccurredMessage, systemReleaseCommand, err)
			return
		}
		if strings.Contains(contents, "Amazon") {
			data := strings.Split(contents, "release")
			name = strings.TrimSpace(data[0])
			version = strings.TrimSpace(data[1])
		} else if strings.Contains(contents, "Red Hat") {
			data := strings.Split(contents, "release")
			name = strings.TrimSpace(data[0])
			version = strings.TrimSpace(data[1])
		} else if strings.Contains(contents, "CentOS") {
			data := strings.Split(contents, "release")
			name = strings.TrimSpace(data[0])
			version = strings.TrimSpace(data[1])
		}
	} else if fileutil.Exists(redhatReleaseCommand) {
		log.Debugf(fetchingDetailsMessage, redhatReleaseCommand)

		contents, err = fileutil.ReadAllText(redhatReleaseCommand)
		log.Debugf(commandOutputMessage, contents)

		if err != nil {
			log.Debugf(errorOccurredMessage, redhatReleaseCommand, err)
			return
		}
		if strings.Contains(contents, "Red Hat") {
			data := strings.Split(contents, "release")
			name = strings.TrimSpace(data[0])
			versionData := strings.Split(data[1], "(")
			version = strings.TrimSpace(versionData[0])
		}
	} else {
		log.Debugf(fetchingDetailsMessage, lsbReleaseCommand)

		// platform name
		if contentsBytes, err = exec.Command(lsbReleaseCommand, "-i").Output(); err != nil {
			log.Debugf(fetchingDetailsMessage, lsbReleaseCommand, err)
			return
		}
		name = strings.TrimSpace(string(contentsBytes))
		log.Debugf(commandOutputMessage, name)
		name = strings.TrimSpace(string(contentsBytes))
		name = strings.TrimLeft(name, "Distributor ID:")
		name = strings.TrimSpace(name)
		log.Debugf("platform name %v", name)

		// platform version
		if contentsBytes, err = exec.Command(lsbReleaseCommand, "-r").Output(); err != nil {
			log.Debugf(errorOccurredMessage, lsbReleaseCommand, err)
			return
		}
		version = strings.TrimSpace(string(contentsBytes))
		log.Debugf(commandOutputMessage, version)
		version = strings.TrimLeft(version, "Release:")
		version = strings.TrimSpace(version)
		log.Debugf("platform version %v", version)
	}
	return
}

var hostNameCommand = filepath.Join("bin", "hostname")

// fullyQualifiedDomainName returns the Fully Qualified Domain Name of the instance, otherwise the hostname
func fullyQualifiedDomainName() string {
	var hostName, fqdn string
	var err error

	if hostName, err = os.Hostname(); err != nil {
		return ""
	}

	var contentBytes []byte
	if contentBytes, err = exec.Command(hostNameCommand, "--fqdn").Output(); err == nil {
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
