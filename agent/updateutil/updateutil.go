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
	"fmt"
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
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
)

const (
	// UpdaterPackageNamePrefix represents the name of Updater Package
	UpdaterPackageNamePrefix = "-updater"

	// HashType represents the default hash type
	HashType = "sha256"

	// Updater represents Updater name
	Updater = "updater"

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

	// PlatformUbuntu represents Ubuntu
	PlatformUbuntu = "ubuntu"

	// PlatformUbuntuSnap represents Ubuntu
	PlatformUbuntuSnap = "snap"

	// PlatformCentOS represents CentOS
	PlatformCentOS = "centos"

	// PlatformSuse represents SLES(SUSe)
	PlatformSuseOS = "sles"

	// PlatformSuse represents Raspbian
	PlatformRaspbian = "raspbian"

	// PlatformWindows represents windows
	PlatformWindows = "windows"

	//PlatformWindowsNano represents windows nano
	PlatformWindowsNano = "windows-nano"

	// DefaultUpdateExecutionTimeoutInSeconds represents default timeout time for execution update related scripts in seconds
	DefaultUpdateExecutionTimeoutInSeconds = 150

	// PipelineTestVersion represents fake version for pipeline tests
	PipelineTestVersion = "255.0.0.0"
)

//ErrorCode is types of Error Codes
type ErrorCode string

const (
	// ErrorInvalidSourceVersion represents Source version is not supported
	ErrorInvalidSourceVersion ErrorCode = "ErrorInvalidSourceVersion"

	// ErrorInvalidTargetVersion represents Target version is not supported
	ErrorInvalidTargetVersion ErrorCode = "ErrorInvalidTargetVersion"

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

	// ErrorInvalidManifest represents Invalid manifest file
	ErrorInvalidManifest ErrorCode = "ErrorInvalidManifest"

	// ErrorInvalidManifestLocation represents Invalid manifest file location
	ErrorInvalidManifestLocation ErrorCode = "ErrorInvalidManifestLocation"

	// ErrorUninstallFailed represents Uninstall failed
	ErrorUninstallFailed ErrorCode = "ErrorUninstallFailed"

	// ErrorInstallFailed represents Install failed
	ErrorInstallFailed ErrorCode = "ErrorInstallFailed"

	// ErrorCannotStartService represents Cannot start Ec2Config service
	ErrorCannotStartService ErrorCode = "ErrorCannotStartService"

	// ErrorCannotStopService represents Cannot stop Ec2Config service
	ErrorCannotStopService ErrorCode = "ErrorCannotStopService"

	// ErrorTimeout represents Installation time-out
	ErrorTimeout ErrorCode = "ErrorTimeout"

	// ErrorUnexpected represents Unexpected Error
	ErrorUnexpected ErrorCode = "ErrorUnexpected"

	// ErrorEnvironmentIssue represents Unexpected Error
	ErrorEnvironmentIssue ErrorCode = "ErrorEnvironmentIssue"

	// ErrorLoadingAgentVersion represents failed for loading agent version
	ErrorLoadingAgentVersion ErrorCode = "ErrorLoadingAgentVersion"
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

// T represents the interface for Update utility
type T interface {
	CreateInstanceContext(log log.T) (context *InstanceContext, err error)
	CreateUpdateDownloadFolder() (folder string, err error)
	ExeCommand(log log.T, cmd string, workingDir string, updaterRoot string, stdOut string, stdErr string, isAsync bool) (err error)
	IsServiceRunning(log log.T, i *InstanceContext) (result bool, err error)
	WaitForServiceToStart(log log.T, i *InstanceContext) (result bool, err error)
	SaveUpdatePluginResult(log log.T, updaterRoot string, updateResult *UpdatePluginResult) (err error)
	IsDiskSpaceSufficientForUpdate(log log.T) (bool, error)
}

// Utility implements interface T
type Utility struct {
	CustomUpdateExecutionTimeoutInSeconds int
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
	isAsync bool) (err error) {

	parts := strings.Fields(cmd)

	if isAsync {
		command := execCommand(parts[0], parts[1:]...)
		command.Dir = workingDir
		prepareProcess(command)
		// Start command asynchronously
		err = cmdStart(command)
		if err != nil {
			return
		}
	} else {
		tempCmd := setPlatformSpecificCommand(parts)
		command := execCommand(tempCmd[0], tempCmd[1:]...)
		command.Dir = workingDir
		stdoutWriter, stderrWriter, exeErr := setExeOutErr(outputRoot, stdOut, stdErr)
		if exeErr != nil {
			return exeErr
		}
		defer stdoutWriter.Close()
		defer stderrWriter.Close()

		command.Stdout = stdoutWriter
		command.Stderr = stderrWriter

		err = cmdStart(command)
		if err != nil {
			return
		}

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
					err = fmt.Errorf("The execution of command returned Exit Status: %d \n %v", exitCode, err.Error())
				}
			}
			return err
		}
	}
	return nil
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
		if commandOutput, err = agentStatusOutput(); err != nil {
			return false, err
		}
	}

	agentStatus := strings.TrimSpace(string(commandOutput))
	if strings.Contains(agentStatus, expectedOutput) {
		return true, nil
	}

	return false, nil
}

// WaitForServiceToStart wait for service to start and returns is service started
func (util *Utility) WaitForServiceToStart(log log.T, i *InstanceContext) (result bool, err error) {
	isRunning := false
	for attempt := 0; attempt < verifyAttemptCount; attempt++ {
		if attempt > 0 {
			log.Infof("Retrying update health check %v out of %v", attempt+1, verifyAttemptCount)
			time.Sleep(time.Duration(verifyRetryIntervalMilliseconds) * time.Millisecond)
		}
		if isRunning, err = util.IsServiceRunning(log, i); err == nil && isRunning {
			return true, nil
		}
	}
	return false, err
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
		isUsingSystemD[PlatformUbuntu] = "15"
		isUsingSystemD[PlatformSuseOS] = "12"
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
