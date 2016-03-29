// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Amazon Software License (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/asl/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// +build windows

// Package platform contains platform specific utilities.
package platform

import (
	"os/exec"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/log"
)

const caption = "Caption"
const version = "Version"

func getPlatformName(log log.T) (value string, err error) {
	lock.Lock()
	defer lock.Unlock()

	if cachePlatformName != "" {
		return cachePlatformName, nil
	}

	return getPlatformDetails(caption, log)
}

func getPlatformVersion(log log.T) (value string, err error) {
	lock.Lock()
	defer lock.Unlock()

	if cachePlatformVersion != "" {
		return cachePlatformVersion, nil
	}
	return getPlatformDetails(version, log)
}

func getPlatformDetails(property string, log log.T) (value string, err error) {
	log.Debugf(gettingPlatformDetailsMessage)
	value = notAvailableMessage

	cmdName := "wmic"
	cmdArgs := []string{"OS", "get", property, "/format:list"}
	var cmdOut []byte
	if cmdOut, err = exec.Command(cmdName, cmdArgs...).Output(); err != nil {
		log.Debugf("There was an error running %v %v, err:%v", cmdName, cmdArgs, err)
		return
	}
	value = strings.TrimSpace(string(cmdOut))
	value = strings.TrimLeft(value, property+"=")
	log.Debugf(commandOutputMessage, value)
	return
}
