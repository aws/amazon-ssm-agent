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
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
)

var (
	pkgMgr map[string]string
)

const (
	// RPMPackageManager represents rpm package management
	RPMPackageManager = "rpm"
	// DPKGPackageManager represents dpkg package management
	DPKGPackageManager = "dpkg"

	// rpm commands related constants
	rpmCmd                        = "rpm"
	rpmCmdArgToGetAllApplications = "-qa"
	rpmQueryFormat                = "--queryformat"
	rpmQueryFormatArgs            = `\{\"Name\":\"%{NAME}\",\"Publisher\":\"%{VENDOR}\",\"Version\":\"%{VERSION}\",\"InstalledTime\":\"%{INSTALLTIME}\",\"ApplicationType\":\"%{GROUP}\",\"Architecture\":\"%{ARCH}\",\"Url\":\"%{URL}\"\},`

	// dpkg query commands related constants
	dpkgCmd                      = "dpkg-query"
	dpkgArgsToGetAllApplications = "-W"
	dpkgQueryFormat              = `-f={"Name":"${Package}","Version":"${Version}","Publisher":"${Maintainer}","ApplicationType":"${Section}","Architecture":"${Architecture}","Url":"${Homepage}"},`
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

// decoupling exec.Command for easy testability
var cmdExecutor = executeCommand

func executeCommand(command string, args ...string) ([]byte, error) {
	return exec.Command(command, args...).CombinedOutput()
}

// decoupling platform.PlatformName for easy testability
var osInfoProvider = platformInfoProvider

func platformInfoProvider(log log.T) (name string, err error) {
	return platform.PlatformName(log)
}

// CollectApplicationData collects all application data from the system using rpm or dpkg query.
func CollectApplicationData(context context.T) (appData []model.ApplicationData) {

	var plName string
	var err error
	log := context.Log()

	//get platform name
	if plName, err = osInfoProvider(log); err != nil {
		log.Infof("Unable to detect platform because of %v - hence no inventory data for %v",
			err.Error(),
			GathererName)
		return
	}

	log.Infof("Platform name: %v, small case converstion: %v", plName, strings.ToLower(plName))

	var args []string
	var cmd string

	//detect package manager and then get application data accordingly.
	if mgr, ok := pkgMgr[strings.ToLower(plName)]; ok {

		switch mgr {
		case RPMPackageManager:
			log.Infof("Detected '%v' as package management system", RPMPackageManager)

			//setting up rpm query command:
			cmd = rpmCmd
			args = append(args, rpmCmdArgToGetAllApplications, rpmQueryFormat, rpmQueryFormatArgs)

		case DPKGPackageManager:
			log.Infof("Detected '%v' as package management system", DPKGPackageManager)

			//setting up dpkg query command:
			cmd = dpkgCmd
			args = append(args, dpkgArgsToGetAllApplications, dpkgQueryFormat)

		default:
			log.Errorf("Unsupported package management system - %v, hence no inventory data for %v",
				mgr, GathererName)
			return
		}

		if appData, err = GetApplicationData(context, cmd, args); err != nil {
			log.Infof("No inventory data because of unexpected errors - %v", err.Error())
		}

	} else {
		log.Errorf("Unable to detect package manager of %v - hence no inventory data for %v",
			plName,
			GathererName)
	}

	//sorts the data based on application-name
	sort.Sort(model.ByName(appData))

	return
}

// GetApplicationData runs a shell command and gets information about all packages/applications
func GetApplicationData(context context.T, command string, args []string) (data []model.ApplicationData, err error) {

	/*
		Note: Following are samples of how rpm & dpkg stores package information.

		RPM:
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

		DPKG:

		Package: netcat
		Priority: optional
		Section: universe/net
		Installed-Size: 30
		Maintainer: Ubuntu Developers <ubuntu-devel-discuss@lists.ubuntu.com>
		Original-Maintainer: Ruben Molina <rmolina@udea.edu.co>
		Architecture: all
		Version: 1.10-40
		Depends: netcat-traditional (>= 1.10-39)
		Filename: pool/universe/n/netcat/netcat_1.10-40_all.deb
		Size: 3340
		MD5sum: 37c303f02b260481fa4fc9fb8b2c1004
		SHA1: 0371a3950d6967480985aa014fbb6fb898bcea3a
		SHA256: eeecb4c93f03f455d2c3f57b0a1e83b54dbeced0918ae563784e86a37bcc16c9
		Description-en: TCP/IP swiss army knife -- transitional package
		 This is a "dummy" package that depends on lenny's default version of
		 netcat, to ease upgrades. It may be safely removed.
		Description-md5: 1353f8c1d079348417c2180319bdde09
		Bugs: https://bugs.launchpad.net/ubuntu/+filebug
		Origin: Ubuntu


		Following fields are relevant for inventory type AWS:Application
		- Name
		- Version
		- Publisher
		- Architecture
		- Url
		- InstalledTime
		- ApplicationType

		We use rpm query & dpkg-query to get above fields and then tranform the data to convert into json
		to simplify its processing.

		Sample rpm query is of following format:
		rpm -qa --queryformat "\{\"Name\":\"%{NAME}\"\},"

		For more details on rpm queryformat, refer http://www.rpm.org/wiki/Docs/QueryFormat

		Sample dpkg-query is of following format:
		dpkg-query -W -f='{"Name":${binary:Package}},'

		For more details on dpkg format, refer to http://manpages.ubuntu.com/manpages/trusty/man1/dpkg-query.1.html
	*/

	var output []byte
	log := context.Log()

	log.Infof("Executing command: %v %v", command, args)

	if output, err = cmdExecutor(command, args...); err != nil {
		log.Debugf("Failed to execute command : %v %v with error - %v",
			command,
			args,
			err.Error())
		log.Debugf("Command Stderr: %v", string(output))
		err = fmt.Errorf("Command failed with error: %v", string(output))
	} else {
		cmdOutput := string(output)
		log.Debugf("Command output: %v", cmdOutput)

		if data, err = ConvertToApplicationData(cmdOutput); err != nil {
			err = fmt.Errorf("Unable to convert query output to ApplicationData - %v", err.Error())
		} else {
			log.Infof("Number of applications detected - %v", len(data))
		}
	}

	return
}

// ConvertToApplicationData converts query output into json string so that it can be deserialized easily
func ConvertToApplicationData(input string) (data []model.ApplicationData, err error) {

	//This implementation is closely tied to the kind of rpm/dpkg query. A change in query MUST be accompanied
	//with a change in transform logic or else json formatting will be impacted.

	/*
		Sample format of our rpm queryformat & dpkg format:
		rpm -qa --queryformat "\{\"Name\":\"%{NAME}\"\},"
		dpkg-query -W -f='{"Name":${binary:Package}},'

		Above queries will generate data in following format:
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
	if err = json.Unmarshal([]byte(str), &data); err == nil {

		//transform the date - by iterating over all elements
		for j, item := range data {
			if item.InstalledTime != "" {
				if i, err := strconv.ParseInt(item.InstalledTime, 10, 64); err == nil {
					//InstalledTime must comply with format: 2016-07-30T18:15:37Z to provide better search experience for customers
					tm := time.Unix(i, 0).UTC()
					data[j].InstalledTime = tm.Format(time.RFC3339)
				}
				//ignore the date transformation if error is encountered
			}
		}
	}

	return
}
