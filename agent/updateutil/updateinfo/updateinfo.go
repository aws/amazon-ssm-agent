// Copyright 2021 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package updateinfo

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
	"github.com/aws/amazon-ssm-agent/agent/versionutil"
)

var isUsingSystemD map[string]string
var updateInfoObj T

var possiblyUsingSystemD = map[string]bool{
	updateconstants.PlatformRaspbian: true,
	updateconstants.PlatformLinux:    true,
}

var mkDirAll = os.MkdirAll
var openFile = os.OpenFile
var execCommand = exec.Command
var cmdStart = (*exec.Cmd).Start
var cmdOutput = (*exec.Cmd).Output
var once = new(sync.Once)
var mutex = new(sync.Mutex)

// IsPlatformUsingSystemD returns if SystemD is the default Init for the Linux platform
func (i *updateInfoImpl) IsPlatformUsingSystemD() (result bool, err error) {
	compareResult := 0
	systemDVersions := getMinimumVersionForSystemD()

	// check if current platform has systemd
	if val, ok := (*systemDVersions)[i.platform]; ok {
		// compare current agent version with minimum supported version
		if compareResult, err = versionutil.VersionCompare(i.platformVersion, val); err != nil {
			return false, err
		}
		if compareResult >= 0 {
			return true, nil
		}
	} else if _, ok := possiblyUsingSystemD[i.platform]; ok {
		// attempt to execute 'systemctl --version' to verify systemd
		if _, commandErr := execCommand("systemctl", "--version").Output(); commandErr != nil {
			return false, nil
		}

		return true, nil
	}

	return false, nil
}

//IsPlatformDarwin returns true for Mac OS
func (i *updateInfoImpl) IsPlatformDarwin() (result bool) {
	return 0 == strings.Compare(i.platform, updateconstants.PlatformMacOsX)
}

//GetInstaller returns the name of the install script
func (i *updateInfoImpl) GetInstaller() string {
	return i.installer
}

//GetUnInstaller returns the name of the uninstall script
func (i *updateInfoImpl) GetUnInstaller() string {
	return i.unInstaller
}

//GetUnInstaller returns the name of the current platform
func (i *updateInfoImpl) GetPlatform() string {
	return i.platform
}

//GetInstallerName returns the name of the instance install type
func (i *updateInfoImpl) GetInstallerName() string {
	return i.installerName
}

func getMinimumVersionForSystemD() (systemDMap *map[string]string) {
	once.Do(func() {
		isUsingSystemD = make(map[string]string)
		isUsingSystemD[updateconstants.PlatformCentOS] = "7"
		isUsingSystemD[updateconstants.PlatformRedHat] = "7"
		isUsingSystemD[updateconstants.PlatformOracleLinux] = "7"
		isUsingSystemD[updateconstants.PlatformUbuntu] = "15"
		isUsingSystemD[updateconstants.PlatformSuseOS] = "12"
		isUsingSystemD[updateconstants.PlatformDebian] = "8"
	})
	return &isUsingSystemD
}

// GenerateCompressedFileName generates downloadable file name base on agreed convension
func (i *updateInfoImpl) GenerateCompressedFileName(packageName string) string {
	fileName := "{PackageName}-{Platform}-{Arch}.{Compressed}"
	fileName = strings.Replace(fileName, updateconstants.PackageNameHolder, packageName, -1)
	fileName = strings.Replace(fileName, updateconstants.PlatformHolder, i.installerName, -1)
	fileName = strings.Replace(fileName, updateconstants.ArchHolder, i.arch, -1)
	fileName = strings.Replace(fileName, updateconstants.CompressedHolder, i.compressFormat, -1)

	return fileName
}

var getDiskSpaceInfo = fileutil.GetDiskSpaceInfo
var getPlatformName = platform.PlatformName
var getPlatformVersion = platform.PlatformVersion

// New create instance related information such as region, platform and arch
func New(context context.T) (T, error) {
	mutex.Lock()
	defer mutex.Unlock()

	if updateInfoObj != nil {
		return updateInfoObj, nil
	}
	var err error
	updateInfoObj, err = newInner(context)

	return updateInfoObj, err
}

func newInner(context context.T) (updateInfo *updateInfoImpl, err error) {
	log := context.Log()
	var installer, unInstaller, platformName, platformVersion, installerName string
	if platformName, err = getPlatformName(log); err != nil {
		return nil, err
	}

	installer = updateconstants.InstallScript
	unInstaller = updateconstants.UninstallScript
	// TODO: Change this structure to a switch and inject the platform name from another method.
	platformName = strings.ToLower(platformName)
	if strings.Contains(platformName, updateconstants.PlatformAmazonLinux) {
		platformName = updateconstants.PlatformLinux
		installerName = updateconstants.PlatformLinux
	} else if strings.Contains(platformName, updateconstants.PlatformRedHat) {
		platformName = updateconstants.PlatformRedHat
		installerName = updateconstants.PlatformLinux
	} else if strings.Contains(platformName, updateconstants.PlatformOracleLinux) {
		platformName = updateconstants.PlatformOracleLinux
		installerName = updateconstants.PlatformLinux
	} else if strings.Contains(platformName, updateconstants.PlatformUbuntu) {
		platformName = updateconstants.PlatformUbuntu
		if isSnap, err := isAgentInstalledUsingSnap(log); err == nil && isSnap {
			installerName = updateconstants.PlatformUbuntuSnap
			installer = updateconstants.SnapInstaller
			unInstaller = updateconstants.SnapUnInstaller
		} else {
			installerName = updateconstants.PlatformUbuntu
			installer = updateconstants.DebInstaller
			unInstaller = updateconstants.DebUnInstaller
		}
	} else if strings.Contains(platformName, updateconstants.PlatformCentOS) {
		platformName = updateconstants.PlatformCentOS
		installerName = updateconstants.PlatformLinux
	} else if strings.Contains(platformName, updateconstants.PlatformSuseOS) {
		platformName = updateconstants.PlatformSuseOS
		installerName = updateconstants.PlatformLinux
	} else if strings.Contains(platformName, updateconstants.PlatformRaspbian) {
		platformName = updateconstants.PlatformRaspbian
		installerName = updateconstants.PlatformUbuntu
	} else if strings.Contains(platformName, updateconstants.PlatformDebian) {
		platformName = updateconstants.PlatformDebian
		installerName = updateconstants.PlatformUbuntu
	} else if strings.Contains(platformName, updateconstants.PlatformMacOsX) {
		platformName = updateconstants.PlatformMacOsX
		installerName = updateconstants.PlatformDarwin
	} else if isNano, _ := platform.IsPlatformNanoServer(log); isNano {
		platformName = updateconstants.PlatformWindowsNano
		installerName = updateconstants.PlatformWindowsNano
	} else {
		platformName = updateconstants.PlatformWindows
		installerName = updateconstants.PlatformWindows
	}

	if platformVersion, err = getPlatformVersion(log); err != nil {
		return nil, err
	}
	updateInfo = &updateInfoImpl{
		context:         context,
		platform:        platformName,
		platformVersion: platformVersion,
		installerName:   installerName,
		arch:            runtime.GOARCH,
		compressFormat:  updateconstants.CompressFormat,
		installer:       installer,
		unInstaller:     unInstaller,
	}

	return updateInfo, nil
}

// isAgentInstalledUsingSnap returns if snap is used to install the snap
func isAgentInstalledUsingSnap(log log.T) (result bool, err error) {

	if _, commandErr := execCommand("snap", "services", "amazon-ssm-agent").Output(); commandErr != nil {
		log.Debugf("Error checking 'snap services amazon-ssm-agent' - %v", commandErr)
		return false, commandErr
	}
	log.Debug("Agent is installed using snap")
	return true, nil

}
