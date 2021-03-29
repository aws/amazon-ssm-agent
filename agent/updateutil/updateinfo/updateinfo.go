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
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/context"
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

var execCommand = exec.Command
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

//GetInstallScriptName returns the name of the install script
func (i *updateInfoImpl) GetInstallScriptName() string {
	return i.installScriptName
}

//GetUninstallScriptName returns the name of the uninstall script
func (i *updateInfoImpl) GetUninstallScriptName() string {
	return i.uninstallScriptName
}

//GetPlatform returns the name of the current platform
func (i *updateInfoImpl) GetPlatform() string {
	return i.platform
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
	platformReplacement := i.platform
	if i.downloadPlatformOverride != "" {
		platformReplacement = i.downloadPlatformOverride
	}

	fileName := "{PackageName}-{Platform}-{Arch}.{Compressed}"
	fileName = strings.Replace(fileName, updateconstants.PackageNameHolder, packageName, -1)
	fileName = strings.Replace(fileName, updateconstants.PlatformHolder, platformReplacement, -1)
	fileName = strings.Replace(fileName, updateconstants.ArchHolder, i.arch, -1)
	fileName = strings.Replace(fileName, updateconstants.CompressedHolder, i.compressFormat, -1)

	return fileName
}

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
	var installScriptName, uninstallScriptName, platformName, platformVersion, downloadPlatformOverride string
	if platformName, err = getPlatformName(log); err != nil {
		return nil, err
	}

	installScriptName = updateconstants.InstallScript
	uninstallScriptName = updateconstants.UninstallScript
	// TODO: Change this structure to a switch and inject the platform name from another method.
	platformName = strings.ToLower(platformName)
	if strings.Contains(platformName, updateconstants.PlatformAmazonLinux) {
		log.Info("Detected platform Amazon Linux")
		platformName = updateconstants.PlatformLinux
	} else if strings.Contains(platformName, updateconstants.PlatformRedHat) {
		log.Info("Detected platform RedHat")
		platformName = updateconstants.PlatformRedHat
		downloadPlatformOverride = updateconstants.PlatformLinux
	} else if strings.Contains(platformName, updateconstants.PlatformOracleLinux) {
		log.Info("Detected platform Oracle Linux")
		platformName = updateconstants.PlatformOracleLinux
		downloadPlatformOverride = updateconstants.PlatformLinux
	} else if strings.Contains(platformName, updateconstants.PlatformUbuntu) {
		platformName = updateconstants.PlatformUbuntu
		log.Info("Detected platform Ubuntu")
		if isSnap, err := isAgentInstalledUsingSnap(log); err == nil && isSnap {
			log.Info("Detected agent installed with snap")
			installScriptName = updateconstants.SnapInstaller
			uninstallScriptName = updateconstants.SnapUnInstaller

			// TODO: when versions below 2.2.546.0 have been deprecated, add line below
			//  Versions below 2.2.546 don't have *-snap-* download packages
			//  with these names and version below would be unable to update
			// downloadPlatformOverride = updateconstants.PlatformUbuntuSnap
		}
	} else if strings.Contains(platformName, updateconstants.PlatformCentOS) {
		log.Info("Detected platform CentOS")
		platformName = updateconstants.PlatformCentOS
		downloadPlatformOverride = updateconstants.PlatformLinux
	} else if strings.Contains(platformName, updateconstants.PlatformSuseOS) {
		log.Info("Detected platform SuseOS")
		platformName = updateconstants.PlatformSuseOS
		downloadPlatformOverride = updateconstants.PlatformLinux
	} else if strings.Contains(platformName, updateconstants.PlatformRaspbian) {
		log.Info("Detected platform Raspbian")
		platformName = updateconstants.PlatformRaspbian
		downloadPlatformOverride = updateconstants.PlatformUbuntu
	} else if strings.Contains(platformName, updateconstants.PlatformDebian) {
		log.Info("Detected platform Debian")
		platformName = updateconstants.PlatformDebian
		downloadPlatformOverride = updateconstants.PlatformUbuntu
	} else if strings.Contains(platformName, updateconstants.PlatformMacOsX) {
		log.Info("Detected platform MacOS")
		platformName = updateconstants.PlatformMacOsX
		downloadPlatformOverride = updateconstants.PlatformDarwin
	} else if isNano, _ := platform.IsPlatformNanoServer(log); isNano {
		log.Info("Detected platform Windows Nano")
		platformName = updateconstants.PlatformWindowsNano
	} else {
		log.Info("Detected platform Windows")
		platformName = updateconstants.PlatformWindows
	}

	if platformVersion, err = getPlatformVersion(log); err != nil {
		return nil, err
	}
	updateInfo = &updateInfoImpl{
		context:                  context,
		platform:                 platformName,
		platformVersion:          platformVersion,
		downloadPlatformOverride: downloadPlatformOverride,
		arch:                     runtime.GOARCH,
		compressFormat:           updateconstants.CompressFormat,
		installScriptName:        installScriptName,
		uninstallScriptName:      uninstallScriptName,
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
