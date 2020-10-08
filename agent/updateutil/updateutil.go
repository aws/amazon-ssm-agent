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

// Package updateutil contains updater specific utilities.
package updateutil

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"errors"
	"strconv"

	"io"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/core/workerprovider/longrunningprovider/executor"
	"github.com/aws/amazon-ssm-agent/core/workerprovider/longrunningprovider/model"
)

const (
	// UpdaterPackageNamePrefix represents the name of Updater Package
	UpdaterPackageNamePrefix = "-updater"

	// HashType represents the default hash type
	HashType = "sha256"

	// Updater represents Updater name
	Updater = "updater"

	// Directory containing older versions of agent during update
	UpdateAmazonSSMAgentDir = "amazon-ssm-agent/"

	// UpdateContextFileName represents Update context json file
	UpdateContextFileName = "updatecontext.json"

	// UpdatePluginResultFileName represents Update plugin result file name
	UpdatePluginResultFileName = "updatepluginresult.json"

	// DefaultOutputFolder represents default location for storing output files
	DefaultOutputFolder = "awsupdateSsmAgent"

	// DefaultStandOut represents the default file name for update stand output
	DefaultStandOut = "stdout"

	// DefaultStandErr represents the default file name for update stand error
	DefaultStandErr = "stderr"

	// RegionHolder represents Place holder for Region
	RegionHolder = "{Region}"

	// PackageNameHolder represents Place holder for package name
	PackageNameHolder = "{PackageName}"

	// PackageVersionHolder represents Place holder for package version
	PackageVersionHolder = "{PackageVersion}"

	// FileNameHolder represents Place holder for file name
	FileNameHolder = "{FileName}"

	// PlatformHolder represents Place holder for platform
	PlatformHolder = "{Platform}"

	// ArchHolder represents Place holder for Arch
	ArchHolder = "{Arch}"

	// CompressedHolder represents Place holder for compress format
	CompressedHolder = "{Compressed}"

	// PlatformLinux represents linux
	PlatformLinux = "linux"

	// PlatformAmazonLinux represents amazon linux
	PlatformAmazonLinux = "amazon"

	// PlatformRedHat represents RedHat
	PlatformRedHat = "red hat"

	// PlatformOracleLinux represents oracle linux
	PlatformOracleLinux = "oracle"

	// PlatformUbuntu represents Ubuntu
	PlatformUbuntu = "ubuntu"

	// PlatformUbuntuSnap represents Ubuntu
	PlatformUbuntuSnap = "snap"

	//PlatformDarwin represents darwin
	PlatformDarwin = "darwin"

	// PlatformCentOS represents CentOS
	PlatformCentOS = "centos"

	// PlatformSuse represents SLES(SUSe)
	PlatformSuseOS = "sles"

	// PlatformRaspbian represents Raspbian
	PlatformRaspbian = "raspbian"

	// PlatformDebian represents Debian
	PlatformDebian = "debian"

	// PlatformWindows represents windows
	PlatformWindows = "windows"

	//PlatformWindowsNano represents windows nano
	PlatformWindowsNano = "windows-nano"

	//PlatformMacOsX represents mac os
	PlatformMacOsX = "mac os x"

	// DefaultUpdateExecutionTimeoutInSeconds represents default timeout time for execution update related scripts in seconds
	DefaultUpdateExecutionTimeoutInSeconds = 150

	// PipelineTestVersion represents fake version for pipeline tests
	PipelineTestVersion = "255.0.0.0"

	SSMAgentWorkerMinVersion = "3.0.0.0"

	// version status of SSM agent
	Active     = "Active"
	Inactive   = "Inactive"
	Deprecated = "Deprecated"

	minimumVersion = "0"

	// Lock file expiry minutes
	UpdateLockFileMinutes = int64(60)

	snapServiceFile = "/etc/systemd/system/snap.amazon-ssm-agent.amazon-ssm-agent.service"

	ManifestPath = "/amazon-ssm-{Region}/ssm-agent-manifest.json"

	// CommonManifestURL is the Manifest URL for regular regions
	CommonManifestURL = "https://s3.{Region}.amazonaws.com" + ManifestPath
)

// error status codes returned from the update scripts
type UpdateScriptExitCode int

const (
	// exit code represents exit code when there is no service manager
	ExitCodeUnsupportedPlatform UpdateScriptExitCode = 124

	// exit code represents exit code from agent update install script
	ExitCodeUpdateUsingPkgMgr UpdateScriptExitCode = 125
)

// SUb status values
const (
	// installRollback represents rollback code flow occurring during installation
	InstallRollback = "InstallRollback_"

	// verificationRollback represents rollback code flow occurring during verification
	VerificationRollback = "VerificationRollback_"

	// downgrade represents that the respective error code was logged during agent downgrade
	Downgrade = "downgrade_"
)

//ErrorCode is types of Error Codes
type ErrorCode string

const (
	// ErrorInvalidSourceVersion represents Source version is not supported
	ErrorInvalidSourceVersion ErrorCode = "ErrorInvalidSourceVersion"

	// ErrorInvalidTargetVersion represents Target version is not supported
	ErrorInvalidTargetVersion ErrorCode = "ErrorInvalidTargetVersion"

	// ErrorSourcePkgDownload represents source version not able to download
	ErrorSourcePkgDownload ErrorCode = "ErrorSourcePkgDownload"

	// ErrorCreateInstanceContext represents the error code while loading the initial context
	ErrorCreateInstanceContext ErrorCode = "ErrorCreateInstanceContext"

	// ErrorTargetPkgDownload represents target version not able to download
	ErrorTargetPkgDownload ErrorCode = "ErrorTargetPkgDownload"

	// ErrorUnexpected represents Unexpected Error from panic
	ErrorUnexpectedThroughPanic ErrorCode = "ErrorUnexpectedThroughPanic"

	// ErrorManifestURLParse represents manifest url parse error
	ErrorManifestURLParse ErrorCode = "ErrorManifestURLParse"

	// ErrorDownloadManifest represents download manifest error
	ErrorDownloadManifest ErrorCode = "ErrorDownloadManifest"

	// ErrorCreateUpdateFolder represents error when creating the download directory
	ErrorCreateUpdateFolder ErrorCode = "ErrorCreateUpdateFolder"

	// ErrorDownloadUpdater represents error when download and unzip the updater
	ErrorDownloadUpdater ErrorCode = "ErrorDownloadUpdater"

	// ErrorExecuteUpdater represents error when execute the updater
	ErrorExecuteUpdater ErrorCode = "ErrorExecuteUpdater"

	// ErrorUnsupportedVersion represents version less than minimum supported version by OS
	ErrorUnsupportedVersion ErrorCode = "ErrorUnsupportedVersion"

	// ErrorUpdateFailRollbackSuccess represents rollback succeeded but update process failed
	ErrorUpdateFailRollbackSuccess ErrorCode = "ErrorUpdateFailRollbackSuccess"

	// ErrorAttemptToDowngrade represents An update is attempting to downgrade Ec2Config to a lower version
	ErrorAttemptToDowngrade ErrorCode = "ErrorAttempToDowngrade"

	// ErrorInitializationFailed represents An update is failed to initialize
	ErrorInitializationFailed ErrorCode = "ErrorInitializationFailed"

	// ErrorInvalidPackage represents Installation package file is invalid
	ErrorInvalidPackage ErrorCode = "ErrorInvalidPackage"

	// ErrorPackageNotAccessible represents Installation package file is not accessible
	ErrorPackageNotAccessible ErrorCode = "ErrorPackageNotAccessible"

	// ErrorInvalidCertificate represents Installation package file doesn't contain valid certificate
	ErrorInvalidCertificate ErrorCode = "ErrorInvalidCertificate"

	// ErrorVersionNotFoundInManifest represents version is not found in the manifest
	ErrorVersionNotFoundInManifest ErrorCode = "ErrorVersionNotFoundInManifest"

	// ErrorInvalidManifest represents Invalid manifest file
	ErrorInvalidManifest ErrorCode = "ErrorInvalidManifest"

	// ErrorInvalidManifestLocation represents Invalid manifest file location
	ErrorInvalidManifestLocation ErrorCode = "ErrorInvalidManifestLocation"

	// ErrorUninstallFailed represents Uninstall failed
	ErrorUninstallFailed ErrorCode = "ErrorUninstallFailed"

	// ErrorUnsupportedServiceManager represents unsupported service manager
	ErrorUnsupportedServiceManager ErrorCode = "ErrorUnsupportedServiceManager"

	// ErrorInstallFailed represents Install failed
	ErrorInstallFailed ErrorCode = "ErrorInstallFailed"

	// ErrorCannotStartService represents Cannot start Ec2Config service
	ErrorCannotStartService ErrorCode = "ErrorCannotStartService"

	// ErrorCannotStopService represents Cannot stop Ec2Config service
	ErrorCannotStopService ErrorCode = "ErrorCannotStopService"

	// ErrorTimeout represents Installation time-out
	ErrorTimeout ErrorCode = "ErrorTimeout"

	// ErrorVersionCompare represents version compare error
	ErrorVersionCompare ErrorCode = "ErrorVersionCompare"

	// ErrorUnexpected represents Unexpected Error
	ErrorUnexpected ErrorCode = "ErrorUnexpected"

	// ErrorUpdaterLockBusy represents message when updater lock is acquired by someone else
	ErrorUpdaterLockBusy ErrorCode = "ErrorUpdaterLockBusy"

	// ErrorEnvironmentIssue represents Unexpected Error
	ErrorEnvironmentIssue ErrorCode = "ErrorEnvironmentIssue"

	// ErrorLoadingAgentVersion represents failed for loading agent version
	ErrorLoadingAgentVersion ErrorCode = "ErrorLoadingAgentVersion"

	SelfUpdatePrefix = "SelfUpdate_"

	// we have same below fields in processor package without underscore
	UpdateFailed    = "UpdateFailed_"
	UpdateSucceeded = "UpdateSucceeded_"
)

type SelfUpdateState string

const (
	Stage SelfUpdateState = "Stage"
)

const (
	// WarnInactiveVersion represents the warning message when inactive version is used for update
	WarnInactiveVersion string = "InactiveAgentVersion"

	// WarnUpdaterLockFail represents warning message that the lock could not be acquired because of system issues
	WarnUpdaterLockFail string = "WarnUpdaterLockFail"
)

// MinimumDiskSpaceForUpdate represents 100 Mb in bytes
const MinimumDiskSpaceForUpdate int64 = 104857600

const (
	verifyAttemptCount              = 36
	verifyRetryIntervalMilliseconds = 5000
)

// InstanceContext holds information for the instance
type InstanceContext struct {
	Region          string
	Platform        string
	PlatformVersion string
	InstallerName   string
	Arch            string
	CompressFormat  string
}

// Manifest represents the json structure of online manifest file.
type Manifest struct {
	SchemaVersion string            `json:"SchemaVersion"`
	URIFormat     string            `json:"UriFormat"`
	Packages      []*PackageContent `json:"Packages"`
}

// PackageContent section in the Manifest json.
type PackageContent struct {
	Name  string         `json:"Name"`
	Files []*FileContent `json:"Files"`
}

// FileContent holds the file name and available versions
type FileContent struct {
	Name              string            `json:"Name"`
	AvailableVersions []*PackageVersion `json:"AvailableVersions"`
}

// PackageVersion section in the PackageContent
type PackageVersion struct {
	Version  string `json:"Version"`
	Checksum string `json:"Checksum"`
	Status   string `json:"Status"`
}

// T represents the interface for Update utility
type T interface {
	CreateInstanceContext(log log.T) (context *InstanceContext, err error)
	CreateUpdateDownloadFolder() (folder string, err error)
	ExeCommand(log log.T, cmd string, workingDir string, updaterRoot string, stdOut string, stdErr string, isAsync bool) (pid int, exitCode UpdateScriptExitCode, err error)
	CleanupCommand(log log.T, pid int) error
	IsServiceRunning(log log.T, i *InstanceContext) (result bool, err error)
	IsWorkerRunning(log log.T) (result bool, err error)
	IsProcessRunning(log log.T, pid int) (result bool, err error)
	WaitForServiceToStart(log log.T, i *InstanceContext, targetVersion string) (result bool, err error)
	SaveUpdatePluginResult(log log.T, updaterRoot string, updateResult *UpdatePluginResult) (err error)
	IsDiskSpaceSufficientForUpdate(log log.T) (bool, error)
	DownloadManifestFile(log log.T, updateDownloadFolder string, manifestUrl string, region string) (*artifact.DownloadOutput, string, error)
}

// Utility implements interface T
type Utility struct {
	CustomUpdateExecutionTimeoutInSeconds int
	ProcessExecutor                       executor.IExecutor
}

var getDiskSpaceInfo = fileutil.GetDiskSpaceInfo
var getRegion = platform.Region
var getPlatformName = platform.PlatformName
var getPlatformVersion = platform.PlatformVersion
var mkDirAll = os.MkdirAll
var openFile = os.OpenFile
var execCommand = exec.Command
var cmdStart = (*exec.Cmd).Start
var cmdOutput = (*exec.Cmd).Output
var isUsingSystemD map[string]string
var once sync.Once

// Installer represents Install shell script for linux
var Installer string

// UnInstaller represents Uninstall shell script for linux
var UnInstaller string

const (
	// installer script for debian
	DebInstaller = "install.sh"
	// uninstaller script for debian
	DebUnInstaller = "uninstall.sh"

	// installer script for snap
	SnapInstaller = "snap-install.sh"
	// uninstaller script for snap
	SnapUnInstaller = "snap-uninstall.sh"
)

var possiblyUsingSystemD = map[string]bool{
	PlatformRaspbian: true,
	PlatformLinux:    true,
}

// CreateInstanceContext create instance related information such as region, platform and arch
func (util *Utility) CreateInstanceContext(log log.T) (context *InstanceContext, err error) {
	region := ""
	if region, err = getRegion(); region == "" {
		return context, fmt.Errorf("Failed to get region, %v", err)
	}
	platformName := ""
	platformVersion := ""
	installerName := ""
	if platformName, err = getPlatformName(log); err != nil {
		return
	}
	// TODO: Change this structure to a switch and inject the platform name from another method.
	platformName = strings.ToLower(platformName)
	if strings.Contains(platformName, PlatformAmazonLinux) {
		platformName = PlatformLinux
		installerName = PlatformLinux
		Installer = InstallScript
		UnInstaller = UninstallScript
	} else if strings.Contains(platformName, PlatformRedHat) {
		platformName = PlatformRedHat
		installerName = PlatformLinux
		Installer = InstallScript
		UnInstaller = UninstallScript
	} else if strings.Contains(platformName, PlatformOracleLinux) {
		platformName = PlatformOracleLinux
		installerName = PlatformLinux
		Installer = InstallScript
		UnInstaller = UninstallScript
	} else if strings.Contains(platformName, PlatformUbuntu) {
		platformName = PlatformUbuntu
		if isSnap, err := isAgentInstalledUsingSnap(log); err == nil && isSnap {
			installerName = PlatformUbuntuSnap
			Installer = SnapInstaller
			UnInstaller = SnapUnInstaller
		} else {
			installerName = PlatformUbuntu
			Installer = DebInstaller
			UnInstaller = DebUnInstaller
		}
	} else if strings.Contains(platformName, PlatformCentOS) {
		platformName = PlatformCentOS
		installerName = PlatformLinux
		Installer = InstallScript
		UnInstaller = UninstallScript
	} else if strings.Contains(platformName, PlatformSuseOS) {
		platformName = PlatformSuseOS
		installerName = PlatformLinux
		Installer = InstallScript
		UnInstaller = UninstallScript
	} else if strings.Contains(platformName, PlatformRaspbian) {
		platformName = PlatformRaspbian
		installerName = PlatformUbuntu
		Installer = InstallScript
		UnInstaller = UninstallScript
	} else if strings.Contains(platformName, PlatformDebian) {
		platformName = PlatformDebian
		installerName = PlatformUbuntu
		Installer = InstallScript
		UnInstaller = UninstallScript
	} else if strings.Contains(platformName, PlatformMacOsX) {
		platformName = PlatformMacOsX
		installerName = PlatformDarwin
		Installer = InstallScript
		UnInstaller = UninstallScript
	} else if isNano, _ := platform.IsPlatformNanoServer(log); isNano {
		//TODO move this logic to instance context
		platformName = PlatformWindowsNano
		installerName = PlatformWindowsNano
		Installer = InstallScript
		UnInstaller = UninstallScript
	} else {
		platformName = PlatformWindows
		installerName = PlatformWindows
		Installer = InstallScript
		UnInstaller = UninstallScript
	}

	if platformVersion, err = getPlatformVersion(log); err != nil {
		return
	}
	context = &InstanceContext{
		Region:          region,
		Platform:        platformName,
		PlatformVersion: platformVersion,
		InstallerName:   installerName,
		Arch:            runtime.GOARCH,
		CompressFormat:  CompressFormat,
	}

	return context, nil
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

// CreateUpdateDownloadFolder creates folder for storing update downloads
func (util *Utility) CreateUpdateDownloadFolder() (folder string, err error) {
	root := filepath.Join(appconfig.DownloadRoot, "update")
	if err = mkDirAll(root, os.ModePerm|os.ModeDir); err != nil {
		return "", err
	}

	return root, nil
}

// ExeCommand executes shell command
func (util *Utility) ExeCommand(
	log log.T,
	cmd string,
	workingDir string,
	outputRoot string,
	stdOut string,
	stdErr string,
	isAsync bool) (int, UpdateScriptExitCode, error) { // pid, exitCode, error

	parts := strings.Fields(cmd)
	pid := -1
	var updateExitCode UpdateScriptExitCode = -1

	if isAsync {
		command := execCommand(parts[0], parts[1:]...)
		command.Dir = workingDir
		prepareProcess(command)
		// Start command asynchronously
		err := cmdStart(command)
		if err != nil {
			return pid, updateExitCode, err
		}
		pid = GetCommandPid(command)
	} else {
		tempCmd := setPlatformSpecificCommand(parts)
		command := execCommand(tempCmd[0], tempCmd[1:]...)
		command.Dir = workingDir
		stdoutWriter, stderrWriter, err := setExeOutErr(outputRoot, stdOut, stdErr)
		if err != nil {
			return pid, updateExitCode, err
		}
		defer stdoutWriter.Close()
		defer stderrWriter.Close()

		command.Stdout = stdoutWriter
		command.Stderr = stderrWriter

		err = cmdStart(command)
		if err != nil {
			return pid, updateExitCode, err
		}

		pid = GetCommandPid(command)

		var timeout = DefaultUpdateExecutionTimeoutInSeconds
		if util.CustomUpdateExecutionTimeoutInSeconds != 0 {
			timeout = util.CustomUpdateExecutionTimeoutInSeconds
		}
		timer := time.NewTimer(time.Duration(timeout) * time.Second)
		go killProcessOnTimeout(log, command, timer)
		err = command.Wait()
		timedOut := !timer.Stop()
		if err != nil {
			log.Debugf("command returned error %v", err)
			if exitErr, ok := err.(*exec.ExitError); ok {
				// The program has exited with an exit code != 0
				if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
					exitCode := status.ExitStatus()
					if exitCode == -1 && timedOut {
						// set appropriate exit code based on cancel or timeout
						exitCode = appconfig.CommandStoppedPreemptivelyExitCode
						log.Infof("The execution of command was timedout.")
					}
					updateExitCode = UpdateScriptExitCode(exitCode)
					err = fmt.Errorf("The execution of command returned Exit Status: %d \n %v", exitCode, err.Error())
				}
			}

			return pid, updateExitCode, err
		}
	}
	return pid, updateExitCode, nil
}

// CleanupCommand cleans up command executed
func (util *Utility) CleanupCommand(log log.T, pid int) error {

	if util.ProcessExecutor == nil {
		util.ProcessExecutor = executor.NewProcessExecutor(log)
	}

	return util.ProcessExecutor.Kill(pid)
}

// TODO move to commandUtil
// ExeCommandOutput executes shell command and returns the stdout
func (util *Utility) ExeCommandOutput(
	log log.T,
	cmd string,
	parameters []string,
	workingDir string,
	outputRoot string,
	stdOutFileName string,
	stdErrFileName string,
	usePlatformSpecificCommand bool) (output string, err error) {

	parts := append([]string{cmd}, parameters...) //strings.Fields(cmd)
	var tempCmd []string
	if usePlatformSpecificCommand {
		tempCmd = setPlatformSpecificCommand(parts)
	} else {
		tempCmd = parts
	}

	command := execCommand(tempCmd[0], tempCmd[1:]...)
	command.Dir = workingDir
	stdoutWriter, stderrWriter, exeErr := setExeOutErr(outputRoot, stdOutFileName, stdErrFileName)
	if exeErr != nil {
		return output, exeErr
	}
	defer stdoutWriter.Close()
	defer stderrWriter.Close()

	// Don't set command.Stdout - we're going to return it instead of writing it
	command.Stderr = stderrWriter

	// Run the command and return its output
	var out []byte
	out, err = cmdOutput(command)
	// Write the returned output so that we can upload it if needed
	stdoutWriter.Write(out)
	if err != nil {
		return
	}

	return string(out), err
}

// TODO move to commandUtil
// ExeCommandOutput executes shell command and returns the stdout
func (util *Utility) NewExeCommandOutput(
	log log.T,
	cmd string,
	parameters []string,
	workingDir string,
	outputRoot string,
	stdoutWriter io.Writer,
	stderrWriter io.Writer,
	usePlatformSpecificCommand bool) (output string, err error) {

	parts := append([]string{cmd}, parameters...) //strings.Fields(cmd)
	var tempCmd []string
	if usePlatformSpecificCommand {
		tempCmd = setPlatformSpecificCommand(parts)
	} else {
		tempCmd = parts
	}

	command := execCommand(tempCmd[0], tempCmd[1:]...)
	command.Dir = workingDir

	// Don't set command.Stdout - we're going to return it instead of writing it
	command.Stderr = stderrWriter

	// Run the command and return its output
	var out []byte
	out, err = cmdOutput(command)
	// Write the returned output so that we can upload it if needed
	stdoutWriter.Write(out)
	if err != nil {
		return
	}

	return string(out), err
}

// IsServiceRunning returns is service running
func (util *Utility) IsServiceRunning(log log.T, i *InstanceContext) (result bool, err error) {
	commandOutput := []byte{}
	expectedOutput := ""
	isSystemD := false
	// isSystemD will always be false for Windows
	if isSystemD, err = i.IsPlatformUsingSystemD(log); err != nil {
		return false, err
	}

	if isSystemD {
		expectedOutput = "Active: active (running)"
		if commandOutput, err = execCommand("systemctl", "status", "amazon-ssm-agent.service").Output(); err != nil {
			//test snap service enabled
			if commandOutput, err = execCommand("systemctl", "status", "snap.amazon-ssm-agent.amazon-ssm-agent.service").Output(); err != nil {
				return false, err
			}
		}
	} else {
		expectedOutput = agentExpectedStatus()
		if commandOutput, err = agentStatusOutput(log); err != nil {
			return false, err
		}
	}

	agentStatus := strings.TrimSpace(string(commandOutput))
	if strings.Contains(agentStatus, expectedOutput) {
		return true, nil
	}

	return false, nil
}

func (util *Utility) IsProcessRunning(log log.T, pid int) (result bool, err error) {
	var allProcesses []executor.OsProcess
	if util.ProcessExecutor == nil {
		util.ProcessExecutor = executor.NewProcessExecutor(log)
	}

	if allProcesses, err = util.ProcessExecutor.Processes(); err != nil {
		return false, err
	}

	for _, process := range allProcesses {
		if process.Pid == pid {
			if process.State == "Z" {
				return false, nil
			}
			return true, nil
		}
	}

	return false, nil
}

func (util *Utility) IsWorkerRunning(log log.T) (result bool, err error) {
	var allProcesses []executor.OsProcess
	if util.ProcessExecutor == nil {
		util.ProcessExecutor = executor.NewProcessExecutor(log)
	}
	if allProcesses, err = util.ProcessExecutor.Processes(); err != nil {
		return false, err
	}
	for _, process := range allProcesses {
		if process.Executable == model.SSMAgentWorkerBinaryName {
			log.Infof("Detect SSM Agent worker running")
			return true, nil
		}
	}

	// For snap, find the work directory
	if _, err := os.Stat(snapServiceFile); err == nil {
		log.Infof("snap is installed")
		file, err := os.Open(snapServiceFile)
		if err != nil {
			return false, fmt.Errorf("failed to open amazon-ssm-agent.service file %s", err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()

			if strings.HasPrefix(line, "WorkingDirectory") {
				splitResults := strings.Split(line, "/var")
				if len(splitResults) <= 1 {
					return false, fmt.Errorf("failed to find amazon-ssm-agent working directory %s", line)
				}

				workerName := filepath.Join(splitResults[1], model.SSMAgentWorkerName)
				log.Infof("identified worker name %s", workerName)
				for _, process := range allProcesses {
					if process.Executable == workerName {
						log.Infof("Detect SSM Agent worker running")
						return true, nil
					}
				}
			}
		}
	}

	return false, fmt.Errorf("ssm agent worker failed to start")
}

// WaitForServiceToStart wait for service to start and returns is service started
func (util *Utility) WaitForServiceToStart(log log.T, i *InstanceContext, targetVersion string) (result bool, svcRunningErr error) {
	isRunning := false
	isWorkerRunning := true
	var workRunningErr error
	for attempt := 0; attempt < verifyAttemptCount; attempt++ {
		if attempt > 0 {
			log.Infof("Retrying update health check %v out of %v", attempt+1, verifyAttemptCount)
			time.Sleep(time.Duration(verifyRetryIntervalMilliseconds) * time.Millisecond)
		}

		isRunning, svcRunningErr = util.IsServiceRunning(log, i)
		if isRunning {
			log.Infof("health check: amazon-ssm-agent is running")
		} else {
			log.Infof("health check: amazon-ssm-agent is not running")
		}

		compareResult, err := VersionCompare(targetVersion, SSMAgentWorkerMinVersion)
		if err == nil && compareResult >= 0 {
			isWorkerRunning, workRunningErr = util.IsWorkerRunning(log)
			if isWorkerRunning {
				log.Infof("health check: %s is running", model.SSMAgentWorkerName)
			} else {
				log.Infof("health check: %s is not running", model.SSMAgentWorkerName)
			}
		}

		if svcRunningErr == nil && workRunningErr == nil && isRunning && isWorkerRunning {
			return true, nil
		}

	}

	errorMessage := "checking svc and agent worker running err"
	if svcRunningErr != nil {
		errorMessage += ", " + svcRunningErr.Error()
	}
	if workRunningErr != nil {
		errorMessage += ", " + workRunningErr.Error()
	}

	return false, fmt.Errorf(errorMessage)
}

// IsDiskSpaceSufficientForUpdate loads disk space info and checks the available bytes
// Returns true if the system has at least 100 Mb for available disk space or false if it is less than 100 Mb
func (util *Utility) IsDiskSpaceSufficientForUpdate(log log.T) (bool, error) {
	var diskSpaceInfo fileutil.DiskSpaceInfo
	var err error

	// Get the available disk space
	if diskSpaceInfo, err = getDiskSpaceInfo(); err != nil {
		log.Infof("Failed to load disk space info - %v", err)
		return false, err
	}

	// Return false if available disk space is less than 100 Mb
	if diskSpaceInfo.AvailBytes < MinimumDiskSpaceForUpdate {
		log.Infof("Insufficient available disk space - %d Mb", diskSpaceInfo.AvailBytes/int64(1024*1024))
		return false, nil
	}

	// Return true otherwise
	return true, nil
}

// IsPlatformUsingSystemD returns if SystemD is the default Init for the Linux platform
func (i *InstanceContext) IsPlatformUsingSystemD(log log.T) (result bool, err error) {
	compareResult := 0
	systemDVersions := getMinimumVersionForSystemD()

	// check if current platform has systemd
	if val, ok := (*systemDVersions)[i.Platform]; ok {
		// compare current agent version with minimum supported version
		if compareResult, err = VersionCompare(i.PlatformVersion, val); err != nil {
			return false, err
		}
		if compareResult >= 0 {
			return true, nil
		}
	} else if _, ok := possiblyUsingSystemD[i.Platform]; ok {
		// attempt to execute 'systemctl --version' to verify systemd
		if _, commandErr := execCommand("systemctl", "--version").Output(); commandErr != nil {
			return false, nil
		}

		return true, nil
	}

	return false, nil
}

func getMinimumVersionForSystemD() (systemDMap *map[string]string) {
	once.Do(func() {
		isUsingSystemD = make(map[string]string)
		isUsingSystemD[PlatformCentOS] = "7"
		isUsingSystemD[PlatformRedHat] = "7"
		isUsingSystemD[PlatformOracleLinux] = "7"
		isUsingSystemD[PlatformUbuntu] = "15"
		isUsingSystemD[PlatformSuseOS] = "12"
		isUsingSystemD[PlatformDebian] = "8"
	})
	return &isUsingSystemD
}

// FileName generates downloadable file name base on agreed convension
func (i *InstanceContext) FileName(packageName string) string {
	fileName := "{PackageName}-{Platform}-{Arch}.{Compressed}"
	fileName = strings.Replace(fileName, PackageNameHolder, packageName, -1)
	fileName = strings.Replace(fileName, PlatformHolder, i.InstallerName, -1)
	fileName = strings.Replace(fileName, ArchHolder, i.Arch, -1)
	fileName = strings.Replace(fileName, CompressedHolder, i.CompressFormat, -1)

	return fileName
}

// BuildMessage builds the messages with provided format, error and arguments
func BuildMessage(err error, format string, params ...interface{}) (message string) {
	message = fmt.Sprintf(format, params...)
	if err != nil {
		message = fmt.Sprintf("%v, ErrorMessage=%v", message, err.Error())
	}
	return message
}

// BuildMessages builds the messages with provided format, error and arguments
func BuildMessages(errs []error, format string, params ...interface{}) (message string) {
	message = fmt.Sprintf(format, params...)
	errMessage := ""
	if len(errs) > 0 {
		for _, err := range errs {
			if errMessage == "" {
				errMessage = err.Error()
			} else {
				errMessage = fmt.Sprintf("%v, %v", errMessage, err.Error())
			}
		}

		message = fmt.Sprintf("%v, ErrorMessage=%v", message, errMessage)
	}
	return message
}

// BuildUpdateCommand builds command string with argument and value
func BuildUpdateCommand(cmd string, arg string, value string) string {
	if value == "" || arg == "" {
		return cmd
	}
	return fmt.Sprintf("%v -%v %v", cmd, arg, value)
}

// UpdateArtifactFolder returns the folder path for storing all the update artifacts
func UpdateArtifactFolder(updateRoot string, packageName string, version string) (folder string) {
	return filepath.Join(updateRoot, packageName, version)
}

// UpdateContextFilePath returns Context file path
func UpdateContextFilePath(updateRoot string) (filePath string) {
	return filepath.Join(updateRoot, UpdateContextFileName)
}

// UpdateOutputDirectory returns output directory
func UpdateOutputDirectory(updateRoot string) string {
	return filepath.Join(updateRoot, DefaultOutputFolder)
}

// UpdateStdOutPath returns stand output file path
func UpdateStdOutPath(updateRoot string, fileName string) string {
	if fileName == "" {
		fileName = DefaultStandOut
	}
	return filepath.Join(updateRoot, fileName)
}

// UpdateStdErrPath returns stand error file path
func UpdateStdErrPath(updateRoot string, fileName string) string {
	if fileName == "" {
		fileName = DefaultStandErr
	}
	return filepath.Join(updateRoot, fileName)
}

// UpdatePluginResultFilePath returns update plugin result file path
func UpdatePluginResultFilePath(updateRoot string) (filePath string) {
	return filepath.Join(updateRoot, UpdatePluginResultFileName)
}

// UpdaterFilePath returns updater file path
func UpdaterFilePath(updateRoot string, updaterPackageName string, version string) (filePath string) {
	return filepath.Join(UpdateArtifactFolder(updateRoot, updaterPackageName, version), Updater)
}

// InstallerFilePath returns Installer file path
func InstallerFilePath(updateRoot string, packageName string, version string) (file string) {
	return filepath.Join(UpdateArtifactFolder(updateRoot, packageName, version), Installer)
}

// UnInstallerFilePath returns UnInstaller file path
func UnInstallerFilePath(updateRoot string, packageName string, version string) (file string) {
	return filepath.Join(UpdateArtifactFolder(updateRoot, packageName, version), UnInstaller)
}

func killProcessOnTimeout(log log.T, command *exec.Cmd, timer *time.Timer) {
	<-timer.C
	log.Debug("Process exceeded timeout. Attempting to kill process!")

	// task has been exceeded the allowed execution timeout, kill process
	if err := command.Process.Kill(); err != nil {
		log.Error(err)
		return
	}

	log.Debug("Done kill process!")
}

// setExeOutErr creates stderr and stdout file
func setExeOutErr(
	updaterRoot string,
	stdOutFileName string,
	stdErrFileName string) (stdoutWriter *os.File, stderrWriter *os.File, err error) {

	if err = mkDirAll(UpdateOutputDirectory(updaterRoot), appconfig.ReadWriteExecuteAccess); err != nil {
		return
	}

	stdOutPath := UpdateStdOutPath(updaterRoot, stdOutFileName)
	stdErrPath := UpdateStdErrPath(updaterRoot, stdErrFileName)

	// create stdout file
	// Allow append so that if arrays of run command write to the same file, we keep appending to the file.
	if stdoutWriter, err = openFile(stdOutPath, appconfig.FileFlagsCreateOrAppend, appconfig.ReadWriteAccess); err != nil {
		return
	}

	// create stderr file
	// Allow append so that if arrays of run command write to the same file, we keep appending to the file.
	if stderrWriter, err = openFile(stdErrPath, appconfig.FileFlagsCreateOrAppend, appconfig.ReadWriteAccess); err != nil {
		return
	}

	return stdoutWriter, stderrWriter, nil
}

// getCommandPid returns the pid of the process if present, defaults to pid -1
func GetCommandPid(cmd *exec.Cmd) int {
	if cmd != nil && cmd.Process != nil {
		return cmd.Process.Pid
	}
	return -1
}

func CompareVersion(versionOne string, versionTwo string) (int, error) {
	majorOne, minorOne, buildOne, patchOne, err := parseVersion(versionOne)
	if err != nil {
		return 0, err
	}

	majorTwo, minorTwo, buildTwo, patchTwo, err := parseVersion(versionTwo)
	if err != nil {
		return 0, err
	}

	if majorOne < majorTwo {
		return -1, nil
	} else if majorOne > majorTwo {
		return 1, nil
	}

	if minorOne < minorTwo {
		return -1, nil
	} else if minorOne > minorTwo {
		return 1, nil
	}

	if buildOne < buildTwo {
		return -1, nil
	} else if buildOne > buildTwo {
		return 1, nil
	}

	if patchOne < patchTwo {
		return -1, nil
	} else if patchOne > patchTwo {
		return 1, nil
	}

	return 0, nil
}

func parseVersion(version string) (uint64, uint64, uint64, uint64, error) {
	parts := strings.SplitN(version, ".", 4)
	if len(parts) != 4 {
		return 0, 0, 0, 0, errors.New("No Major.Minor.Build.Patch elements found")
	}

	major, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	minor, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	build, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	patch, err := strconv.ParseUint(parts[3], 10, 64)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	return major, minor, build, patch, nil
}

func PrepareResourceForSelfUpdate(
	logger log.T,
	manifestURL string,
	version string) (sourceLocation, sourceHash, targetVersion, targetLocation, targetHash, manifestFinalURL, manifestFilePath string, err error) {

	util := &Utility{}
	var context *InstanceContext
	var parsedManifest *Manifest
	var manifestDownloadOutput *artifact.DownloadOutput
	var updateDownloadFolder string
	var isDeprecated bool

	SelfUpdateResult := &UpdatePluginResult{
		StandOut:      "",
		StartDateTime: time.Now(),
	}

	if err = util.SaveUpdatePluginResult(logger, appconfig.UpdaterArtifactsRoot, SelfUpdateResult); err != nil {
		logger.WriteEvent(log.AgentUpdateResultMessage, version, GenerateSelUpdateErrorEvent(ErrorInitializationFailed))
		return "", "", "", "", "", "", "",
			logger.Errorf("Failed to update plugin result for selfupdate %v", err)
	}

	if context, err = util.CreateInstanceContext(logger); err != nil {
		logger.WriteEvent(log.AgentUpdateResultMessage, version, GenerateSelUpdateErrorEvent(ErrorCreateInstanceContext))
		return "", "", "", "", "", "", "",
			logger.Errorf("Failed to validate version, %v", err)
	}

	logger.Infof("manifest url is %v : ", manifestURL)
	if manifestDownloadOutput, manifestFinalURL, err = util.DownloadManifestFile(logger, updateDownloadFolder, manifestURL, context.Region); err != nil {
		logger.WriteEvent(log.AgentUpdateResultMessage, version, GenerateSelUpdateErrorEvent(ErrorDownloadManifest))
		return "", "", "", "", "", "", "",
			logger.Errorf("Failed to generate manifest file local path, %v", err)
	}
	manifestFilePath = manifestDownloadOutput.LocalFilePath
	logger.Infof("manifest file path is %v: ", manifestFilePath)

	if parsedManifest, err = ParseManifest(logger, manifestFilePath, context, appconfig.DefaultAgentName); err != nil {
		logger.WriteEvent(log.AgentUpdateResultMessage, version, GenerateSelUpdateErrorEvent(ErrorManifestURLParse))
		return "", "", "", "", "", "", "",
			logger.Errorf("Failed to parse Manifest for preparing resource for selfupdate, %v", err)
	}

	// get the latest active version and it's location
	if isDeprecated, err = isVersionDeprecated(logger, parsedManifest, appconfig.DefaultAgentName, version, context); err != nil {
		logger.WriteEvent(log.AgentUpdateResultMessage, version, GenerateSelUpdateErrorEvent(ErrorVersionNotFoundInManifest))
		return "", "", "", "", "", "", "",
			logger.Errorf("Failed to find version in manifest for selfupdate, %v", err)
	}

	if isDeprecated {
		// get the latest active version and it's location
		logger.Infof("Agent version %v is deprecated", version)
		if targetVersion, err = latestActiveVersion(logger, parsedManifest, context); err != nil {
			logger.WriteEvent(log.AgentUpdateResultMessage, version, GenerateSelUpdateErrorEvent(ErrorVersionCompare))
			return "", "", "", "", "", "", "",
				logger.Errorf("Failed to generate the target information, fail to get latest active version from manifest file, %v", err)
		}
	} else {
		targetVersion = version
	}

	// target version download url location
	logger.Infof("target version is %v", version)
	if targetLocation, targetHash, err = downloadURLandHash(logger,
		parsedManifest, context, targetVersion, appconfig.DefaultAgentName); err != nil {
		logger.WriteEvent(log.AgentUpdateResultMessage, version, GenerateSelUpdateErrorEvent(ErrorTargetPkgDownload))
		return "", "", "", "", "", "", "",
			logger.Errorf("Failed to get the sourceHash from manifest file, %v", err)
	}

	// source version download url location
	logger.Infof("source version is %v", version)
	if sourceLocation, sourceHash, err = downloadURLandHash(logger,
		parsedManifest, context, version, appconfig.DefaultAgentName); err != nil {
		logger.WriteEvent(log.AgentUpdateResultMessage, version, GenerateSelUpdateErrorEvent(ErrorSourcePkgDownload))
		return "", "", "", "", "", "", "",
			logger.Errorf("Failed to get the sourceHash from manifest file for version %v", version)
	}

	return
}

// GenerateSelUpdateErrorEvent constructs error codes for self update
func GenerateSelUpdateErrorEvent(errorCode ErrorCode) string {
	return UpdateFailed + SelfUpdatePrefix + string(errorCode)
}

// GenerateSelUpdateSuccessEvent constructs success codes for self update
func GenerateSelUpdateSuccessEvent(code string) string {
	return UpdateSucceeded + SelfUpdatePrefix + code
}

func ValidateVersion(log log.T, manifestFilePath string, version string) bool {
	util := &Utility{}
	var context *InstanceContext
	var parsedManifest *Manifest
	var isValid bool
	var err error
	if context, err = util.CreateInstanceContext(log); err != nil {
		log.Error("Error during validate version")
	}

	if parsedManifest, err = ParseManifest(log, manifestFilePath, context, appconfig.DefaultAgentName); err != nil {
		log.Error("Error during parsed Manifest for validating version")
	}

	if isValid, err = validateVersion(log, parsedManifest, appconfig.DefaultAgentName, version, context); err != nil {
		log.Error("Error during validate version from Manifest")
		return false
	}
	log.Infof("Version %v is %v", version, isValid)

	return isValid
}

// ParseManifest parses the public manifest file to provide agent update information.
func ParseManifest(log log.T,
	fileName string,
	context *InstanceContext,
	packageName string) (parsedManifest *Manifest, err error) {
	//Load specified file from file system
	var result = []byte{}
	if result, err = ioutil.ReadFile(fileName); err != nil {
		return
	}
	// parse manifest file
	if err = json.Unmarshal([]byte(result), &parsedManifest); err != nil {
		return
	}

	err = validateManifest(log, parsedManifest, context, packageName)
	return
}

func (util *Utility) DownloadManifestFile(log log.T, updateDownloadFolder string, manifestUrl string, region string) (*artifact.DownloadOutput, string, error) {
	var downloadOutput artifact.DownloadOutput
	var err error

	// best efforts for the old agents
	// download the manifest file from well-known location
	if manifestUrl == "" {
		manifestUrl = CommonManifestURL
		if dynamicS3Endpoint := platform.GetDefaultEndPoint(region, "s3"); dynamicS3Endpoint != "" {
			manifestUrl = "https://" + dynamicS3Endpoint + ManifestPath
		}
	}

	manifestUrl = strings.Replace(manifestUrl, RegionHolder, region, -1)
	log.Infof("manifest download url is %s", manifestUrl)

	downloadInput := artifact.DownloadInput{
		SourceURL:            manifestUrl,
		DestinationDirectory: updateDownloadFolder,
	}

	downloadOutput, err = artifact.Download(log, downloadInput)
	if err != nil ||
		downloadOutput.IsHashMatched == false ||
		downloadOutput.LocalFilePath == "" {
		if err != nil {
			return nil, "", fmt.Errorf("failed to download file reliably, %v, %v", downloadInput.SourceURL, err.Error())
		}
		return nil, "", fmt.Errorf("failed to download file reliably, %v", downloadInput.SourceURL)
	}

	log.Infof("Succeed to download the manifest")
	log.Infof("Local file path : %v", downloadOutput.LocalFilePath)
	log.Infof("Is updated: %v", downloadOutput.IsUpdated)
	log.Infof("Is hash matched %v", downloadOutput.IsHashMatched)
	return &downloadOutput, manifestUrl, nil
}

func downloadURLandHash(log log.T,
	m *Manifest,
	context *InstanceContext,
	version, packageName string) (downloadURL, hash string, err error) {

	fileName := context.FileName(packageName)
	downloadURL = m.URIFormat

	for _, p := range m.Packages {
		if p.Name == packageName {
			log.Infof("found package for version hash %v", packageName)
			for _, f := range p.Files {
				if f.Name == fileName {
					log.Infof("found file name for version hash %v", fileName)
					for _, v := range f.AvailableVersions {
						if v.Version == version {
							log.Infof("Version %v checksum is %v", version, v.Checksum)
							downloadURL = strings.Replace(downloadURL, RegionHolder, context.Region, -1)
							downloadURL = strings.Replace(downloadURL, PackageNameHolder, packageName, -1)
							downloadURL = strings.Replace(downloadURL, PackageVersionHolder, version, -1)
							downloadURL = strings.Replace(downloadURL, FileNameHolder, context.FileName(packageName), -1)

							log.Infof("Download resource location is %v", downloadURL)
							return downloadURL, v.Checksum, nil
						}
					}
					break
				}
			}
		}
	}

	return "", "", fmt.Errorf("failed to get the downloadURL and Hash for version %v", version)
}

func latestActiveVersion(log log.T,
	m *Manifest, context *InstanceContext) (targetVersion string, err error) {
	targetVersion = minimumVersion
	var compareResult = 0
	var packageName = "amazon-ssm-agent"

	for _, p := range m.Packages {
		if p.Name == packageName {
			for _, f := range p.Files {
				if f.Name == context.FileName(packageName) {
					for _, v := range f.AvailableVersions {
						if !isVersionActive(v.Status) {
							continue
						}
						if compareResult, err = VersionCompare(v.Version, targetVersion); err != nil {
							return "", err
						}
						if compareResult > 0 {
							targetVersion = v.Version
						}
					}
				}
			}
		}
	}

	if targetVersion == minimumVersion {
		log.Debugf("Filename: %v", context.FileName(packageName))
		log.Debugf("Package Name: %v", packageName)
		log.Debugf("Manifest: %v", m)
		return "", fmt.Errorf("cannot find the latest version for package %v", packageName)
	}

	return targetVersion, nil
}

// validateManifest makes sure all the fields are provided.
func validateManifest(log log.T, parsedManifest *Manifest, context *InstanceContext, packageName string) error {
	if len(parsedManifest.URIFormat) == 0 {
		return fmt.Errorf("folder format cannot be null in the Manifest file")
	}
	fileName := context.FileName(packageName)
	foundPackage := false
	foundFile := false
	for _, p := range parsedManifest.Packages {
		if p.Name == packageName {
			log.Infof("found package %v", packageName)
			foundPackage = true
			for _, f := range p.Files {
				if f.Name == fileName {
					foundFile = true
					if len(f.AvailableVersions) == 0 {
						return fmt.Errorf("at least one available version is required for the %v", fileName)
					}

					log.Infof("found file %v", fileName)
					break
				}
			}
		}
	}

	if !foundPackage {
		return fmt.Errorf("cannot find the %v information in the Manifest file", packageName)
	}
	if !foundFile {
		return fmt.Errorf("cannot find the %v information in the Manifest file", fileName)
	}

	return nil
}

func validateVersion(log log.T,
	parsedManifest *Manifest,
	packageName string,
	version string,
	context *InstanceContext) (isValid bool, err error) {

	fileName := context.FileName(packageName)
	for _, p := range parsedManifest.Packages {
		if p.Name == packageName {
			log.Infof("found package %v", packageName)
			for _, f := range p.Files {
				if f.Name == fileName {
					for _, v := range f.AvailableVersions {
						if v.Version == version {
							status := Active
							if v.Status != "" {
								status = v.Status
							}

							log.Infof("Version %v status is %v", version, status)
							return isVersionActive(v.Status), nil
						}
					}
					break
				}
			}
		}
	}

	// return true for backward compatibility
	return true, nil
}

func isVersionDeprecated(log log.T,
	parsedManifest *Manifest,
	packageName string,
	version string,
	context *InstanceContext) (isValid bool, err error) {

	fileName := context.FileName(packageName)
	for _, p := range parsedManifest.Packages {
		if p.Name == packageName {
			log.Infof("found package %v", packageName)
			for _, f := range p.Files {
				if f.Name == fileName {
					for _, v := range f.AvailableVersions {
						if v.Version == version {
							status := Active
							if v.Status != "" {
								status = v.Status
							}

							log.Infof("Version %v status is %v", version, status)
							return v.Status == Deprecated, nil
						}
					}
					break
				}
			}
		}
	}

	return isValid, fmt.Errorf("cannot find  %v information for %v in Manifest file", fileName, version)
}

func isVersionActive(agentVersionStatus string) bool {

	switch agentVersionStatus {
	case Active:
		return true
	case Inactive:
		return false
	case Deprecated:
		return false
	case "":
		return true
	default:
		return false
	}
}
