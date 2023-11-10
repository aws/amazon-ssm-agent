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

//go:build windows
// +build windows

// Package packagemanagers holds functions querying using local package manager
package packagemanagers

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/common"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/servicemanagers"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/verificationmanagers"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/utility"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
	"golang.org/x/sys/windows/registry"
)

type windowsManager struct {
	managerHelper common.IManagerHelper
}

var (
	RegistryNotExistErr     = fmt.Errorf("%v", "RegistryNotExistErrCode")
	FailedToOpenRegistryErr = fmt.Errorf("%v", "FailedToOpenRegistryErr")

	isNanoServer                 = platform.IsPlatformNanoServer
	timeSleep                    = time.Sleep
	fileUnCompress               = fileutil.Uncompress
	fileMoveFiles                = fileutil.MoveFiles
	ioUtilReadDir                = ioutil.ReadDir
	deleteDir                    = fileutil.DeleteDirectory
	getInstalledAgentVersionFunc = getInstalledAgentVersion

	nanoPackageZip = "package.zip"
)

// GetFilesReqForInstall returns all the files the package manager needs to install the agent
func (m *windowsManager) GetFilesReqForInstall(log log.T) []string {
	if isNano, _ := isNanoServer(log); isNano {
		return []string{
			nanoPackageZip,
		}
	}
	return []string{
		common.AmazonWindowsSetupFile,
	}
}

// InstallAgent installs the agent using package manager, folderPath should contain all files required for installation
func (m *windowsManager) InstallAgent(log log.T, installedAgentVersionPath string) (err error) {
	isNano, _ := isNanoServer(log)
	if isNano {
		log.Infof("Windows Nano platform detected during install")
	}
	if isNano {
		err = m.installOnWindowsNano(log, installedAgentVersionPath)
	} else {
		err = m.installOnWindows(installedAgentVersionPath)
	}
	if err == nil {
		timeSleep(5 * time.Second)
		return
	}
	timeSleep(5 * time.Second)
	if err != nil {
		installedAgentVersion, _ := m.GetInstalledAgentVersion()
		if !strings.Contains(installedAgentVersionPath, installedAgentVersion) {
			return err
		}
	}
	return
}

// UninstallAgent uninstalls the agent using the package manager
func (m *windowsManager) UninstallAgent(log log.T, installedAgentVersionPath string) (err error) {
	isNano, _ := isNanoServer(log)
	if isNano {
		log.Infof("Windows Nano platform detected")
	}
	if isNano {
		err = m.uninstallOnWindowsNano(log, installedAgentVersionPath)
		return
	}
	err = m.uninstallOnWindows(installedAgentVersionPath)
	timeSleep(2 * time.Second)
	return
}

// IsAgentInstalled returns true if agent is installed using package manager, returns error for any unexpected errors
func (m *windowsManager) IsAgentInstalled() (bool, error) {
	_, err := getInstalledAgentVersionFunc(m)
	if err == nil {
		return true, nil
	}
	if strings.HasPrefix(err.Error(), FailedToOpenRegistryErr.Error()) {
		return false, fmt.Errorf("failed to open registry for agent to determine if agent is installed")
	}
	return !strings.HasPrefix(err.Error(), RegistryNotExistErr.Error()), nil
}

// IsManagerEnvironment returns true if all commands required by the package manager are available
func (m *windowsManager) IsManagerEnvironment() bool {
	return runtime.GOOS == "windows"
}

// GetSupportedServiceManagers returns all the service manager types that the package manager supports
func (m *windowsManager) GetSupportedServiceManagers() []servicemanagers.ServiceManager {
	return []servicemanagers.ServiceManager{servicemanagers.Windows}
}

// GetName returns the package manager name
func (m *windowsManager) GetName() string {
	return "windows"
}

// GetType returns the package manager type
func (m *windowsManager) GetType() PackageManager {
	return Windows
}

// GetInstalledAgentVersion returns the version of the installed agent
func (m *windowsManager) GetInstalledAgentVersion() (string, error) {
	return getInstalledAgentVersionFunc(m)
}

func getInstalledAgentVersion(m *windowsManager) (string, error) {
	ssmKey, err := registry.OpenKey(registry.LOCAL_MACHINE, appconfig.ItemPropertyPath, registry.QUERY_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return "", RegistryNotExistErr
		} else {
			return "", FailedToOpenRegistryErr
		}
	}
	defer ssmKey.Close()

	version, _, err := ssmKey.GetStringValue("Version")
	if err != nil {
		if err == registry.ErrNotExist {
			output, err := m.managerHelper.RunCommand(appconfig.DefaultSSMAgentBinaryPath, "--version")
			output = utility.CleanupVersion(output)
			if output != "" {
				return output, nil
			}
			return "", fmt.Errorf("failed to get agent version from registry as well as agent binary: %v", err)
		} else {
			return "", fmt.Errorf("failed to open agent version attribute from registry: %v", err)
		}
	}
	return utility.CleanupVersion(version), nil
}

// GetFileExtension returns the file extension of the agent using the package manager
func (m *windowsManager) GetFileExtension() string {
	return ".exe"
}

// GetSupportedVerificationManager returns verification manager types that the package manager supports
func (m *windowsManager) GetSupportedVerificationManager() verificationmanagers.VerificationManager {
	return verificationmanagers.Windows
}

func (m *windowsManager) installOnWindowsNano(log log.T, installedAgentVersionPath string) error {
	tempFolder := "temp"
	scExePath := "sc.exe"
	tempPath := filepath.Join(installedAgentVersionPath, tempFolder)
	srcZipFile := filepath.Join(installedAgentVersionPath, nanoPackageZip)
	err := fileUnCompress(log, srcZipFile, tempPath)
	if err != nil {
		return fmt.Errorf("uncompress of nano packages failed: %v", err)
	}
	err = m.uninstallOnWindowsNano(log, installedAgentVersionPath)
	if err != nil {
		return err
	}
	err = fileMoveFiles(tempPath, appconfig.DefaultProgramFolder)
	if err != nil {
		return fmt.Errorf("moving files to program folder failed: %v", err)
	}
	amazonExecutable := filepath.Join(appconfig.DefaultProgramFolder, common.AmazonSSMExecutable)
	_, err = m.managerHelper.RunCommand(scExePath, "create", "AmazonSSMAgent", "binpath='"+amazonExecutable+"'", "start=auto", "displayname='Amazon SSM Agent'")
	if err != nil {
		if m.managerHelper.IsTimeoutError(err) {
			return fmt.Errorf("windows scCreate install: command timed out")
		}
		return fmt.Errorf("windows scCreate install: failed to install with error: %v", err)
	}

	_, err = m.managerHelper.RunCommand(scExePath, "description", "AmazonSSMAgent", "'Amazon SSM Agent'")
	if err != nil {
		if m.managerHelper.IsTimeoutError(err) {
			return fmt.Errorf("windows description update: command timed out")
		}
		return fmt.Errorf("windows description update: failed to install with error: %v", err)
	}

	_, err = m.managerHelper.RunCommand(scExePath, "failureflag", "AmazonSSMAgent", "1")
	if err != nil {
		if m.managerHelper.IsTimeoutError(err) {
			return fmt.Errorf("windows service failure flag update: Command timed out")
		}
		return fmt.Errorf("windows service failure flag update: Failed to install with error: %v", err)
	}

	_, err = m.managerHelper.RunCommand(scExePath, "failure", "AmazonSSMAgent", "reset=86400", "actions= restart/30000/restart/30000/restart/30000")
	if err != nil {
		if m.managerHelper.IsTimeoutError(err) {
			return fmt.Errorf("windows restart service: Command timed out")
		}
		return fmt.Errorf("windows restart service: Failed to install with error: %v", err)
	}

	netExecPath := "C:\\Windows\\System32\\net.exe"
	_, err = m.managerHelper.RunCommand(netExecPath, "start", "AmazonSSMAgent")
	if err != nil {
		if m.managerHelper.IsTimeoutError(err) {
			return fmt.Errorf("windows start: Command timed out")
		}
		return fmt.Errorf("windows start: Failed to install with error: %v", err)
	}
	return nil
}

func (m *windowsManager) uninstallOnWindowsNano(log log.T, installedAgentVersionPath string) error {
	netExecPath := "C:\\Windows\\System32\\net.exe"
	_, _ = m.managerHelper.RunCommand(netExecPath, "stop", "AmazonSSMAgent")

	_, _ = m.managerHelper.RunCommand("Get-CimInstance", "-ClassName", "Win32_Service", "-Filter", "'Name=\"AmazonSSMAgent\"'", "|", "Invoke-CimMethod", "-MethodName", "Delete")

	files := make([]string, 0)
	skipFiles := map[string]interface{}{appconfig.SeelogConfigFileName: struct{}{}, appconfig.AppConfigFileName: struct{}{}}
	if list, err := ioUtilReadDir(appconfig.DefaultProgramFolder); err == nil {
		for _, fileInfo := range list {
			filePath := filepath.Join(appconfig.DefaultProgramFolder, fileInfo.Name())
			files = append(files, filePath)
			if _, ok := skipFiles[fileInfo.Name()]; ok {
				continue
			}
			err = deleteDir(filePath)
			if err != nil {
				log.Warnf("error while deleting directory during uninstall: %v", err)
			}
		}
	}
	return nil
}

func (m *windowsManager) installOnWindows(installedAgentVersionPath string) error {
	setupExecPath := filepath.Join(installedAgentVersionPath, common.AmazonWindowsSetupFile)
	output, err := m.managerHelper.RunCommandWithCustomTimeout(time.Duration(updateconstants.DefaultUpdateExecutionTimeoutInSeconds)*time.Second, appconfig.PowerShellPluginCommandName, "(Start-Process", "'"+setupExecPath+"'", "-ArgumentList @('ALLOWEC2INSTALL=YES', 'SKIPSYMLINKSCAN=YES', '/quiet', '/norestart')", "-Wait)")
	if err != nil {
		if m.managerHelper.IsTimeoutError(err) {
			return fmt.Errorf("windows install: Command timed out")
		}
		return fmt.Errorf("windows install: Failed to install with error %v: %v", setupExecPath, output)
	}
	return nil
}

func (m *windowsManager) uninstallOnWindows(installedAgentVersionPath string) error {
	setupExecPath := filepath.Join(installedAgentVersionPath, common.AmazonWindowsSetupFile)
	output, err := m.managerHelper.RunCommandWithCustomTimeout(time.Duration(updateconstants.DefaultUpdateExecutionTimeoutInSeconds)*time.Second, appconfig.PowerShellPluginCommandName, "(Start-Process", "'"+setupExecPath+"'", "-ArgumentList @('/uninstall', '/quiet', '/norestart')", "-Wait)")
	if err != nil {
		if m.managerHelper.IsTimeoutError(err) {
			return fmt.Errorf("windows uninstall: Command timed out")
		}
		return fmt.Errorf("windows uninstall: Failed to uninstall with error: %v", output)
	}

	return nil
}
