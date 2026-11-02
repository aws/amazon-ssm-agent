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

//go:build freebsd || linux || netbsd || openbsd
// +build freebsd linux netbsd openbsd

// Package platform contains platform specific utilities.
package platform

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"gopkg.in/ini.v1"
)

const (
	osReleaseFile                 = "/etc/os-release"
	systemReleaseFile             = "/etc/system-release"
	centosReleaseFile             = "/etc/centos-release"
	redhatReleaseFile             = "/etc/redhat-release"
	bottlerocketReleaseFile       = "/etc/bottlerocket-release"
	unameCommand                  = "/usr/bin/uname"
	lsbReleaseCommand             = "lsb_release"
	fetchingDetailsMessage        = "fetching platform details from %v"
	errorOccurredMessage          = "There was an error running %v, err: %v"
	NitroVendorSystemInfoParamKey = "/sys/class/dmi/id/sys_vendor"
	NitroUuidSystemInfoParamKey   = "/sys/class/dmi/id/product_uuid"
	XenVersionSystemInfoParamKey  = "/sys/hypervisor/version/extra"
	XenUuidSystemInfoParamKey     = "/sys/hypervisor/uuid"
)

var (
	readAllText = fileutil.ReadAllText
	fileExists  = fileutil.Exists
	writeFile   = os.WriteFile

	ErrFileNotFound   = errors.New("file not found")
	ErrFilePermission = errors.New("no sufficient permissions")
	ErrFileRead       = errors.New("error reading from file")
)

// this structure is similar to the /etc/os-release file
type osRelease struct {
	NAME       string
	VERSION_ID string
}

func isPlatformWindowsServer2012OrEarlier(_ log.T) (bool, error) {
	return false, nil
}

func isPlatformWindowsServer2025OrLater(_ log.T) (bool, error) {
	return false, nil
}

func isWindowsServer2025OrLater(_ string, _ log.T) (bool, error) {
	return false, nil
}

func getPlatformData(log log.T) (PlatformData, error) {
	platformName, platformVersion, err := getPlatformDetails(log)
	return PlatformData{
		Name:    platformName,
		Version: platformVersion,
		Type:    "linux",
	}, err
}

func getPlatformDetails(log log.T) (name string, version string, err error) {
	log.Debugf(gettingPlatformDetailsMessage)
	contents := ""
	var contentsBytes []byte
	name = notAvailableMessage
	version = notAvailableMessage

	if fileExists(centosReleaseFile) {
		// CentOS has incomplete information in the osReleaseFile
		// and there fore needs to be before osReleaseFile exist check
		log.Debugf(fetchingDetailsMessage, centosReleaseFile)
		contents, err = fileutil.ReadAllText(centosReleaseFile)
		log.Debugf(commandOutputMessage, contents)

		if err != nil {
			log.Debugf(errorOccurredMessage, centosReleaseFile, err)
			return
		}

		if strings.Contains(contents, "CentOS") {
			data := strings.Split(contents, "release")
			name = strings.TrimSpace(data[0])
			if len(data) >= 2 {
				versionData := strings.Split(data[1], "(")
				version = strings.TrimSpace(versionData[0])
			}
		}
	}
	if !(strings.EqualFold(name, notAvailableMessage)) || !(strings.EqualFold(version, notAvailableMessage)) {
		return
	} else if fileExists(bottlerocketReleaseFile) {
		// Bottlerocket's osReleaseFile contains information from its
		// control container's base OS, with the Bottlerocket data
		// stored in a separate bottlerocketReleasefile and therefore
		// needs to be before osReleaseFile exist check
		log.Debugf(fetchingDetailsMessage, bottlerocketReleaseFile)
		contents := new(osRelease)
		err = ini.MapTo(contents, bottlerocketReleaseFile)
		log.Debugf(commandOutputMessage, contents)
		if err != nil {
			log.Debugf(errorOccurredMessage, bottlerocketReleaseFile, err)
			return
		}

		name = contents.NAME
		version = contents.VERSION_ID
	} else if fileExists(osReleaseFile) {

		log.Debugf(fetchingDetailsMessage, osReleaseFile)
		contents := new(osRelease)
		err = ini.MapTo(contents, osReleaseFile)
		log.Debugf(commandOutputMessage, contents)
		if err != nil {
			log.Debugf(errorOccurredMessage, osReleaseFile, err)
			return
		}

		name = contents.NAME
		version = contents.VERSION_ID

	} else if fileExists(systemReleaseFile) {
		// We want to fall back to legacy behaviour in case some older versions of
		// linux distributions do not have the or-release file
		log.Debugf(fetchingDetailsMessage, systemReleaseFile)

		contents, err = readAllText(systemReleaseFile)
		log.Debugf(commandOutputMessage, contents)

		if err != nil {
			log.Debugf(errorOccurredMessage, systemReleaseFile, err)
			return
		}
		if strings.Contains(contents, "Amazon") {
			data := strings.Split(contents, "release")
			name = strings.TrimSpace(data[0])
			if len(data) >= 2 {
				version = strings.TrimSpace(data[1])
			}
		} else if strings.Contains(contents, "Red Hat") {
			data := strings.Split(contents, "release")
			name = strings.TrimSpace(data[0])
			if len(data) >= 2 {
				versionData := strings.Split(data[1], "(")
				version = strings.TrimSpace(versionData[0])
			}
		} else if strings.Contains(contents, "CentOS") {
			data := strings.Split(contents, "release")
			name = strings.TrimSpace(data[0])
			if len(data) >= 2 {
				version = strings.TrimSpace(data[1])
			}
		} else if strings.Contains(contents, "SLES") {
			data := strings.Split(contents, "release")
			name = strings.TrimSpace(data[0])
			if len(data) >= 2 {
				version = strings.TrimSpace(data[1])
			}
		} else if strings.Contains(contents, "Raspbian") {
			data := strings.Split(contents, "release")
			name = strings.TrimSpace(data[0])
			if len(data) >= 2 {
				version = strings.TrimSpace(data[1])
			}
		} else if strings.Contains(contents, "Oracle") {
			data := strings.Split(contents, "release")
			name = strings.TrimSpace(data[0])
			if len(data) >= 2 {
				version = strings.TrimSpace(data[1])
			}
		} else if strings.Contains(contents, "Rocky") {
			data := strings.Split(contents, "release")
			name = strings.TrimSpace(data[0])
			if len(data) >= 2 {
				version = strings.TrimSpace(data[1])
			}
		}
	} else if fileExists(redhatReleaseFile) {
		log.Debugf(fetchingDetailsMessage, redhatReleaseFile)

		contents, err = readAllText(redhatReleaseFile)
		log.Debugf(commandOutputMessage, contents)

		if err != nil {
			log.Debugf(errorOccurredMessage, redhatReleaseFile, err)
			return
		}
		if strings.Contains(contents, "Red Hat") {
			data := strings.Split(contents, "release")
			name = strings.TrimSpace(data[0])
			if len(data) >= 2 {
				versionData := strings.Split(data[1], "(")
				version = strings.TrimSpace(versionData[0])
			}
		}
	} else if runtime.GOOS == "freebsd" {
		log.Debugf(fetchingDetailsMessage, unameCommand)

		if contentsBytes, err = exec.Command(unameCommand, "-sr").Output(); err != nil {
			log.Debugf(fetchingDetailsMessage, unameCommand, err)
			return
		}
		log.Debugf(commandOutputMessage, contentsBytes)

		data := strings.Split(string(contentsBytes), " ")
		name = strings.TrimSpace(data[0])
		if len(data) >= 2 {
			version = strings.TrimSpace(data[1])
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

func initSystemInfoCache(log log.T, paramKey string) (string, error) {
	if !fileExists(paramKey) {
		log.Warnf("Could not find file %v. Will skip caching the data", paramKey)
		return "", ErrFileNotFound
	}

	if text, err := readAllText(paramKey); err == nil {
		data := strings.TrimSpace(text)
		cache.Put(paramKey, data)
		return data, nil
	} else if os.IsPermission(err) {
		log.Errorf("No sufficient permissions to read file %v, error %v. Will skip caching the data", paramKey, err)
		return "", errors.Join(ErrFilePermission, err)
	} else {
		log.Errorf("Could not read file %v, error %v. Will skip caching the data", paramKey, err)
		return "", errors.Join(ErrFileRead, err)
	}
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
	if contentBytes, err = exec.Command(hostNameCommand, "--fqdn").Output(); err == nil {
		fqdn = string(contentBytes)
		//trim whitespaces - since by default above command appends '\n' at the end.
		//e.g: 'ip-172-31-7-113.ec2.internal\n'
		fqdn = strings.TrimSpace(fqdn)
	} else {
		log.Debugf("Could not fetch FQDN using command %v, error %v. Ignoring", hostNameCommand, err)
	}

	if fqdn != "" {
		return fqdn
	}

	return strings.TrimSpace(hostName)
}

func isPlatformNanoServer(_ log.T) (bool, error) {
	return false, nil
}

// Set the OOM score adjustment for the current process on supported platforms.
// -1000 makes the process immune to the OOM killer under normal circumstances.
func SetOOMScoreAdjust(log log.T) {
	const oomScoreAdj = -1000
	oomScoreAdjPath := "/proc/self/oom_score_adj"

	if fileExists(oomScoreAdjPath) {
		err := writeFile(oomScoreAdjPath, []byte(fmt.Sprintf("%d", oomScoreAdj)), 0644)
		if err != nil {
			log.Warnf("Failed to set OOM score adjustment: %v. Agent will be vulnerable to OOM killer.", err)
			return
	    }
        log.Debugf("Successfully set OOM score adjustment to %d for SSM agent process", oomScoreAdj)
    }
}
