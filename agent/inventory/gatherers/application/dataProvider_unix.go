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

// Package application contains application gatherer.

// +build darwin freebsd linux netbsd openbsd

package application

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/inventory/model"
	"github.com/aws/amazon-ssm-agent/agent/platform"
)

// TODO: add unit tests

var (
	pkgMgr map[string]string
)

const (
	RPMPackageManager  = "rpm"
	DPKGPackageManager = "dpkg"

	//command for listing all applications using rpm
	RPMCmd                        = "rpm"
	RPMCmdArgToGetAllApplications = "-qa"
	RPMQueryFormat                = "--queryformat"
	RPMQueryFormatArgs            = `\{\"Name\":\"%{NAME}\",\"Publisher\":\"%{VENDOR}\",\"Version\":\"%{VERSION}\",\"InstalledTime\":\"%{INSTALLTIME}\",\"ApplicationType\":\"%{GROUP}\",\"Architecture\":\"%{ARCH}\",\"Url\":\"%{URL}\"\},`

	//command for listing all applications using dpkg
	DPKGCmd  = ""
	DPKGArgs = ""
)

func init() {
	//map of package managers in different linux distros
	pkgMgr = make(map[string]string)

	pkgMgr["amazon linux ami"] = RPMPackageManager
	pkgMgr["centos"] = RPMPackageManager
	pkgMgr["redhat"] = RPMPackageManager
	pkgMgr["fedora"] = RPMPackageManager
	pkgMgr["ubuntu"] = DPKGPackageManager
	pkgMgr["debian"] = DPKGPackageManager
}

func CollectApplicationData(context context.T) (appData []inventory.ApplicationData) {

	var plName string
	var err error
	log := context.Log()

	//get platform name
	if plName, err = platform.PlatformName(log); err != nil {
		log.Infof("Unable to detect platform because of %v - hence no inventory data for %v",
			err.Error(),
			GathererName)
		return
	}

	log.Infof("Platform name: %v, small case converstion: %v", plName, strings.ToLower(plName))

	//get package manager
	if mgr, ok := pkgMgr[strings.ToLower(plName)]; ok {

		switch mgr {
		case RPMPackageManager:
			log.Infof("Detected '%v' as package management system", RPMPackageManager)
			if appData, err = GetApplicationDataUsingRPMQuery(context); err != nil {
				log.Errorf("No inventory data because of unexpected errors - %v", err.Error())
			}

		case DPKGPackageManager:
			log.Infof("Detected '%v' as package management system", DPKGPackageManager)
			if appData, err = GetApplicationDataUsingDPKGQuery(context); err != nil {
				log.Errorf("No inventory data because of unexpected errors - %v", err.Error())
			}
		default:
			log.Errorf("Unsupported package management system - %v, hence no inventory data for %v",
				mgr, GathererName)
		}

	} else {
		log.Errorf("Unable to detect package manager of %v - hence no inventory data for %v",
			plName,
			GathererName)
	}

	return
}

// GetApplicationDataUsingDPKGQuery uses dpkg query to get list of all installed applications/packages
func GetApplicationDataUsingDPKGQuery(context context.T) (data []inventory.ApplicationData, err error) {
	log := context.Log()
	log.Infof("Implementation of DPKG is missing - stay tuned")
	return
}

// GetApplicationDataUsingRPMQuery uses rpm query to get list of all installed applications/packages
func GetApplicationDataUsingRPMQuery(context context.T) (data []inventory.ApplicationData, err error) {

	/*
		Note: Following is a sample of how rpm stores package information.

		Name        : python27
		Version     : 2.7.10
		Release     : 4.120.amzn1
		Architecture: x86_64
		Install Date: Fri 29 Apr 2016 11:58:27 PM UTC
		Group       : Development/Languages
		Size        : 86074
		License     : Python
		Signature   : RSA/SHA256, Sat 12 Dec 2015 03:15:10 AM UTC, Key ID bcb4a85b21c0f39f
		Source RPM  : python27-2.7.10-4.120.amzn1.src.rpm
		Build Date  : Tue 08 Dec 2015 06:38:19 PM UTC
		Build Host  : build-60007.build
		Relocations : (not relocatable)
		Packager    : Amazon.com, Inc. <http://aws.amazon.com>
		Vendor      : Amazon.com
		URL         : http://www.python.org/
		Summary     : An interpreted, interactive, object-oriented programming language
		Description :
		Python is an interpreted, interactive, object-oriented programming
		language often compared to Tcl, Perl, Scheme or Java. Python includes
		modules, classes, exceptions, very high level dynamic data types and
		dynamic typing. Python supports interfaces to many system calls and
		libraries, as well as to various windowing systems (X11, Motif, Tk,
		Mac and MFC).

		Programmers can write new built-in modules for Python in C or C++.
		Python can be used as an extension language for applications that need
		a programmable interface.

		Note that documentation for Python is provided in the python-docs
		package.

		This package provides the "python" executable; most of the actual
		implementation is within the "python-libs" package.

		Following fields are relevant for inventory type AWS:Application
		- Name
		- Version
		- Vendor (mapped to Publisher)
		- Architecture
		- URL
		- InstallTime
		- Group

		We use rpm query to get above fields and then tranform the data to convert into json
		to simplify its processing.

		Our rpm query is of following format:
		rpm -qa --queryformat "\{\"Name\":\"%{NAME}\"\},"

		For more details on rpm queryformat, refer http://www.rpm.org/wiki/Docs/QueryFormat

	*/

	log := context.Log()
	var args []string
	args = append(args, RPMCmdArgToGetAllApplications, RPMQueryFormat, RPMQueryFormatArgs)

	var out bytes.Buffer
	var e bytes.Buffer
	cmd := exec.Command(RPMCmd, args...)
	cmd.Stdout = &out
	cmd.Stderr = &e

	log.Infof("Executing command: %v", cmd.Args)

	if err = cmd.Run(); err != nil {
		log.Debugf("Failed to execute command : %v with error - %v", cmd.Args, err.Error())
		log.Debugf("Command Stderr: %v", e.String())
		err = errors.New(fmt.Sprintf("Command failed with error: %v", e.String()))
	} else {
		cmdOutput := out.String()
		log.Debugf("Command output: %v", cmdOutput)

		if data, err = ConvertToApplicationData(cmdOutput); err != nil {
			err = log.Errorf("Unable to convert query output to ApplicationData - %v", err.Error())
		} else {
			log.Infof("Number of applications detected by %v - %v", GathererName, len(data))
		}
	}

	return
}

// ConvertToApplicationData converts query output into json string so that it can be deserialized easily
func ConvertToApplicationData(input string) (data []inventory.ApplicationData, err error) {
	//This implementation is closely tied to the kind of rpm query. A change in rpm query MUST be accompanied
	//with a change in transform logic or else json formatting will be impacted.

	/*
		Sample format of our rpm queryformat
		rpm -qa --queryformat "\{\"Name\":\"%{NAME}\"\},"

		Data will be generated in following format:
		{"Name":"nss-softokn"},{"Name":"basesystem"},{"Name":"pcre"},

		Keeping above sample in mind - we do following operations:
		- remove trailing white spaces
		- remove trailing ','
		- prefix '[' at the beginning & ']' at the end

		After above operation above sample data will convert to:
		[{"Name":"nss-softokn"},{"Name":"basesystem"},{"Name":"pcre"}]
	*/

	//trim spaces
	str := strings.TrimSpace(input)

	//remove last ',' from string
	str = strings.TrimSuffix(str, ",")

	//add "[" in beginning & "]" at the end to create valid json string
	str = fmt.Sprintf("[%v]", str)

	//unmarshall json string accordingly.
	err = json.Unmarshal([]byte(str), &data)

	return
}
