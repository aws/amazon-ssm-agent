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
	"strconv"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
)

const (
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

// decoupling exec.Command for easy testability
var cmdExecutor = executeCommand

func executeCommand(command string, args ...string) ([]byte, error) {
	return exec.Command(command, args...).CombinedOutput()
}

func platformInfoProvider(log log.T) (name string, err error) {
	return platform.PlatformName(log)
}

// collectPlatformDependentApplicationData collects all application data from the system using rpm or dpkg query.
func collectPlatformDependentApplicationData(context context.T) (appData []model.ApplicationData) {

	var err error
	log := context.Log()

	args := []string{dpkgArgsToGetAllApplications, dpkgQueryFormat}
	cmd := dpkgCmd

	// try dpkg first, if any error occurs, use rpm
	if appData, err = getApplicationData(context, cmd, args); err != nil {
		log.Info("Getting applications information using dpkg failed, trying rpm now")
		cmd = rpmCmd
		args = []string{rpmCmdArgToGetAllApplications, rpmQueryFormat, rpmQueryFormatArgs}
		if appData, err = getApplicationData(context, cmd, args); err != nil {
			log.Errorf("Unable to detect package manager - hence no inventory data for %v", GathererName)
		}
	}

	return
}

// getApplicationData runs a shell command and gets information about all packages/applications
func getApplicationData(context context.T, command string, args []string) (data []model.ApplicationData, err error) {

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

	log.Debugf("Executing command: %v %v", command, args)

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

		if data, err = convertToApplicationData(cmdOutput); err != nil {
			err = fmt.Errorf("Unable to convert query output to ApplicationData - %v", err.Error())
		} else {
			log.Infof("Number of applications detected - %v", len(data))
		}
	}

	return
}

// convertToApplicationData converts query output into json string so that it can be deserialized easily
func convertToApplicationData(input string) (data []model.ApplicationData, err error) {

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

		//transform the date & architecture - by iterating over all elements
		for i, item := range data {
			if item.InstalledTime != "" {
				if sec, err := strconv.ParseInt(item.InstalledTime, 10, 64); err == nil {
					//InstalledTime must comply with format: 2016-07-30T18:15:37Z to provide better search experience for customers
					tm := time.Unix(sec, 0).UTC()
					item.InstalledTime = tm.Format(time.RFC3339)
				}
				//ignore the date transformation if error is encountered
			}
			item.CompType = componentType(item.Name)

			/*
				dpkg reports applications architecture as amd64, i386, all
				rpm reports applications architecture as x86_64, i386, noarch

				For consistency, we want to ensure that architecture is reported as x86_64, i386 for
				64bit & 32bit applications across all platforms.
			*/
			item.Architecture = model.FormatArchitecture(item.Architecture)

			data[i] = item
		}
	}

	return
}
