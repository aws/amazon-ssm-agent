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
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateinfo"
	"github.com/aws/amazon-ssm-agent/agent/versionutil"
	"github.com/aws/amazon-ssm-agent/core/executor"
	"github.com/aws/amazon-ssm-agent/core/workerprovider/longrunningprovider/model"
)

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
	CreateUpdateDownloadFolder() (folder string, err error)
	ExeCommand(log log.T, cmd string, workingDir string, updaterRoot string, stdOut string, stdErr string, isAsync bool) (pid int, exitCode updateconstants.UpdateScriptExitCode, err error)
	IsServiceRunning(log log.T, i updateinfo.T) (result bool, err error)
	IsWorkerRunning(log log.T) (result bool, err error)
	WaitForServiceToStart(log log.T, i updateinfo.T, targetVersion string) (result bool, err error)
	SaveUpdatePluginResult(log log.T, updaterRoot string, updateResult *UpdatePluginResult) (err error)
	IsDiskSpaceSufficientForUpdate(log log.T) (bool, error)
	DownloadManifestFile(log log.T, updateDownloadFolder string, manifestUrl string, region string) (*artifact.DownloadOutput, string, error)
}

// Utility implements interface T
type Utility struct {
	Context                               context.T
	CustomUpdateExecutionTimeoutInSeconds int
	ProcessExecutor                       executor.IExecutor
}

var getDiskSpaceInfo = fileutil.GetDiskSpaceInfo
var getPlatformName = platform.PlatformName
var getPlatformVersion = platform.PlatformVersion
var mkDirAll = os.MkdirAll
var openFile = os.OpenFile
var execCommand = exec.Command
var cmdStart = (*exec.Cmd).Start
var cmdOutput = (*exec.Cmd).Output
var isUsingSystemD map[string]string
var once sync.Once

// CreateInstanceContext create instance related information such as region, platform and arch
func (util *Utility) CreateInstanceInfo(log log.T) (context updateinfo.T, err error) {
	return nil, nil
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
	isAsync bool) (int, updateconstants.UpdateScriptExitCode, error) { // pid, exitCode, error

	parts := strings.Fields(cmd)
	pid := -1
	var updateExitCode updateconstants.UpdateScriptExitCode = -1

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

		var timeout = updateconstants.DefaultUpdateExecutionTimeoutInSeconds
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
					updateExitCode = updateconstants.UpdateScriptExitCode(exitCode)
					err = fmt.Errorf("The execution of command returned Exit Status: %d \n %v", exitCode, err.Error())
				}
			}

			return pid, updateExitCode, err
		}
	}
	return pid, updateExitCode, nil
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
func (util *Utility) IsServiceRunning(log log.T, i updateinfo.T) (result bool, err error) {
	commandOutput := []byte{}
	expectedOutput := ""
	isSystemD := false
	isDarwin := false

	// For mac OS check the running processes
	isDarwin = i.IsPlatformDarwin()
	var allProcesses []executor.OsProcess
	if util.ProcessExecutor == nil {
		util.ProcessExecutor = executor.NewProcessExecutor(log)
	}

	// isSystemD will always be false for Windows
	if isSystemD, err = i.IsPlatformUsingSystemD(); err != nil {
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
		agentStatus := strings.TrimSpace(string(commandOutput))
		return strings.Contains(agentStatus, expectedOutput), nil
	} else if isDarwin {
		if allProcesses, err = util.ProcessExecutor.Processes(); err != nil {
			return false, err
		}
		for _, process := range allProcesses {
			if process.Executable == updateconstants.DarwinBinaryPath {
				return true, nil
			}
		}
		return false, nil
	}

	return isAgentServiceRunning(log)
}

// IsWorkerRunning returns true if ssm-agent-worker running
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
	if _, err := os.Stat(updateconstants.SnapServiceFile); err == nil {
		log.Infof("snap is installed")
		file, err := os.Open(updateconstants.SnapServiceFile)
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

	return false, nil
}

// WaitForServiceToStart wait for service to start and returns is service started
func (util *Utility) WaitForServiceToStart(log log.T, i updateinfo.T, targetVersion string) (result bool, svcRunningErr error) {
	const (
		verifyAttemptCount              = 36
		verifyRetryIntervalMilliseconds = 5000
	)
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

		compareResult, err := versionutil.VersionCompare(targetVersion, updateconstants.SSMAgentWorkerMinVersion)
		if err == nil && compareResult >= 0 {
			isWorkerRunning, workRunningErr = util.IsWorkerRunning(log)
			if isWorkerRunning {
				log.Infof("health check: %s is running", model.SSMAgentWorkerName)
			} else {
				if workRunningErr != nil {
					log.Warnf("health check: failed to get state of %s: %v", model.SSMAgentWorkerName, workRunningErr)
				} else {
					log.Infof("health check: %s is not running", model.SSMAgentWorkerName)
				}
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
	if diskSpaceInfo.AvailBytes < updateconstants.MinimumDiskSpaceForUpdate {
		log.Infof("Insufficient available disk space - %d Mb", diskSpaceInfo.AvailBytes/int64(1024*1024))
		return false, nil
	}

	// Return true otherwise
	return true, nil
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
	return filepath.Join(updateRoot, updateconstants.UpdateContextFileName)
}

// UpdateOutputDirectory returns output directory
func UpdateOutputDirectory(updateRoot string) string {
	return filepath.Join(updateRoot, updateconstants.DefaultOutputFolder)
}

// UpdateStdOutPath returns stand output file path
func UpdateStdOutPath(updateRoot string, fileName string) string {
	if fileName == "" {
		fileName = updateconstants.DefaultStandOut
	}
	return filepath.Join(updateRoot, fileName)
}

// UpdateStdErrPath returns stand error file path
func UpdateStdErrPath(updateRoot string, fileName string) string {
	if fileName == "" {
		fileName = updateconstants.DefaultStandErr
	}
	return filepath.Join(updateRoot, fileName)
}

// UpdatePluginResultFilePath returns update plugin result file path
func UpdatePluginResultFilePath(updateRoot string) (filePath string) {
	return filepath.Join(updateRoot, updateconstants.UpdatePluginResultFileName)
}

// UpdaterFilePath returns updater file path
func UpdaterFilePath(updateRoot string, updaterPackageName string, version string) (filePath string) {
	return filepath.Join(UpdateArtifactFolder(updateRoot, updaterPackageName, version), updateconstants.Updater)
}

// InstallerFilePath returns Installer file path
func InstallerFilePath(updateRoot string, packageName string, version string, installer string) (file string) {
	return filepath.Join(UpdateArtifactFolder(updateRoot, packageName, version), installer)
}

// UnInstallerFilePath returns UnInstaller file path
func UnInstallerFilePath(updateRoot string, packageName string, version string, uninstaller string) (file string) {
	return filepath.Join(UpdateArtifactFolder(updateRoot, packageName, version), uninstaller)
}

func killProcessOnTimeout(log log.T, command *exec.Cmd, timer *time.Timer) {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Kill process on timeout panic: \n%v", r)
			log.Errorf("Stacktrace:\n%s", debug.Stack())
		}
	}()
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
	context context.T,
	updateInfo updateinfo.T,
	manifestURL string,
	version string) (sourceLocation, sourceHash, targetVersion, targetLocation, targetHash, manifestFinalURL, manifestFilePath string, err error) {

	util := &Utility{
		Context: context,
	}
	logger := context.Log()
	var parsedManifest *Manifest
	var manifestDownloadOutput *artifact.DownloadOutput
	var updateDownloadFolder string
	var isDeprecated bool

	SelfUpdateResult := &UpdatePluginResult{
		StandOut:      "",
		StartDateTime: time.Now(),
	}

	if err = util.SaveUpdatePluginResult(logger, appconfig.UpdaterArtifactsRoot, SelfUpdateResult); err != nil {
		logger.WriteEvent(log.AgentUpdateResultMessage, version, GenerateSelUpdateErrorEvent(updateconstants.ErrorInitializationFailed))
		return "", "", "", "", "", "", "",
			logger.Errorf("Failed to update plugin result for selfupdate %v", err)
	}

	region, _ := context.Identity().Region()

	logger.Infof("manifest url is %v : ", manifestURL)
	if manifestDownloadOutput, manifestFinalURL, err = util.DownloadManifestFile(logger, updateDownloadFolder, manifestURL, region); err != nil {
		logger.WriteEvent(log.AgentUpdateResultMessage, version, GenerateSelUpdateErrorEvent(updateconstants.ErrorDownloadManifest))
		return "", "", "", "", "", "", "",
			logger.Errorf("Failed to generate manifest file local path, %v", err)
	}
	manifestFilePath = manifestDownloadOutput.LocalFilePath
	logger.Infof("manifest file path is %v: ", manifestFilePath)

	if parsedManifest, err = ParseManifest(logger, manifestFilePath, updateInfo, appconfig.DefaultAgentName); err != nil {
		logger.WriteEvent(log.AgentUpdateResultMessage, version, GenerateSelUpdateErrorEvent(updateconstants.ErrorManifestURLParse))
		return "", "", "", "", "", "", "",
			logger.Errorf("Failed to parse Manifest for preparing resource for selfupdate, %v", err)
	}

	// get the latest active version and it's location
	if isDeprecated, err = isVersionDeprecated(logger, parsedManifest, appconfig.DefaultAgentName, version, updateInfo); err != nil {
		logger.WriteEvent(log.AgentUpdateResultMessage, version, GenerateSelUpdateErrorEvent(updateconstants.ErrorVersionNotFoundInManifest))
		return "", "", "", "", "", "", "",
			logger.Errorf("Failed to find version in manifest for selfupdate, %v", err)
	}

	if isDeprecated {
		// get the latest active version and it's location
		logger.Infof("Agent version %v is deprecated", version)
		if targetVersion, err = latestActiveVersion(logger, parsedManifest, updateInfo); err != nil {
			logger.WriteEvent(log.AgentUpdateResultMessage, version, GenerateSelUpdateErrorEvent(updateconstants.ErrorVersionCompare))
			return "", "", "", "", "", "", "",
				logger.Errorf("Failed to generate the target information, fail to get latest active version from manifest file, %v", err)
		}
	} else {
		targetVersion = version
	}

	// target version download url location
	logger.Infof("target version is %v", version)
	if targetLocation, targetHash, err = downloadURLandHash(logger,
		parsedManifest, updateInfo, targetVersion, appconfig.DefaultAgentName, region); err != nil {
		logger.WriteEvent(log.AgentUpdateResultMessage, version, GenerateSelUpdateErrorEvent(updateconstants.ErrorTargetPkgDownload))
		return "", "", "", "", "", "", "",
			logger.Errorf("Failed to get the sourceHash from manifest file, %v", err)
	}

	// source version download url location
	logger.Infof("source version is %v", version)
	if sourceLocation, sourceHash, err = downloadURLandHash(logger,
		parsedManifest, updateInfo, version, appconfig.DefaultAgentName, region); err != nil {
		logger.WriteEvent(log.AgentUpdateResultMessage, version, GenerateSelUpdateErrorEvent(updateconstants.ErrorSourcePkgDownload))
		return "", "", "", "", "", "", "",
			logger.Errorf("Failed to get the sourceHash from manifest file for version %v", version)
	}

	return
}

// GenerateSelUpdateErrorEvent constructs error codes for self update
func GenerateSelUpdateErrorEvent(errorCode updateconstants.ErrorCode) string {
	return updateconstants.UpdateFailed + updateconstants.SelfUpdatePrefix + string(errorCode)
}

// GenerateSelUpdateSuccessEvent constructs success codes for self update
func GenerateSelUpdateSuccessEvent(code string) string {
	return updateconstants.UpdateSucceeded + updateconstants.SelfUpdatePrefix + code
}

func ValidateVersion(context context.T, info updateinfo.T, manifestFilePath string, version string) bool {
	log := context.Log()
	var parsedManifest *Manifest
	var isValid bool
	var err error

	if parsedManifest, err = ParseManifest(log, manifestFilePath, info, appconfig.DefaultAgentName); err != nil {
		log.Error("Error during parsed Manifest for validating version")
	}

	if isValid, err = validateVersion(log, parsedManifest, appconfig.DefaultAgentName, version, info); err != nil {
		log.Error("Error during validate version from Manifest")
		return false
	}
	log.Infof("Version %v is %v", version, isValid)

	return isValid
}

// ParseManifest parses the public manifest file to provide agent update information.
func ParseManifest(log log.T,
	fileName string,
	info updateinfo.T,
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

	err = validateManifest(log, parsedManifest, info, packageName)
	return
}

func (util *Utility) DownloadManifestFile(log log.T, updateDownloadFolder string, manifestUrl string, region string) (*artifact.DownloadOutput, string, error) {
	var downloadOutput artifact.DownloadOutput
	var err error

	// best efforts for the old agents
	// download the manifest file from well-known location
	if manifestUrl == "" {
		manifestUrl = updateconstants.CommonManifestURL
		if dynamicS3Endpoint := util.Context.Identity().GetDefaultEndpoint("s3"); dynamicS3Endpoint != "" {
			manifestUrl = "https://" + dynamicS3Endpoint + updateconstants.ManifestPath
		}
	}

	manifestUrl = strings.Replace(manifestUrl, updateconstants.RegionHolder, region, -1)
	log.Infof("manifest download url is %s", manifestUrl)

	downloadInput := artifact.DownloadInput{
		SourceURL:            manifestUrl,
		DestinationDirectory: updateDownloadFolder,
	}

	downloadOutput, err = artifact.Download(util.Context, downloadInput)
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
	info updateinfo.T,
	version, packageName, region string) (downloadURL, hash string, err error) {

	fileName := info.GenerateCompressedFileName(packageName)
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
							downloadURL = strings.Replace(downloadURL, updateconstants.RegionHolder, region, -1)
							downloadURL = strings.Replace(downloadURL, updateconstants.PackageNameHolder, packageName, -1)
							downloadURL = strings.Replace(downloadURL, updateconstants.PackageVersionHolder, version, -1)
							downloadURL = strings.Replace(downloadURL, updateconstants.FileNameHolder, fileName, -1)

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
	m *Manifest, info updateinfo.T) (targetVersion string, err error) {
	targetVersion = updateconstants.MinimumVersion
	var compareResult = 0
	var packageName = "amazon-ssm-agent"

	for _, p := range m.Packages {
		if p.Name == packageName {
			for _, f := range p.Files {
				if f.Name == info.GenerateCompressedFileName(packageName) {
					for _, v := range f.AvailableVersions {
						if !isVersionActive(v.Status) {
							continue
						}
						if compareResult, err = versionutil.VersionCompare(v.Version, targetVersion); err != nil {
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

	if targetVersion == updateconstants.MinimumVersion {
		log.Debugf("Filename: %v", info.GenerateCompressedFileName(packageName))
		log.Debugf("Package Name: %v", packageName)
		log.Debugf("Manifest: %v", m)
		return "", fmt.Errorf("cannot find the latest version for package %v", packageName)
	}

	return targetVersion, nil
}

// validateManifest makes sure all the fields are provided.
func validateManifest(log log.T, parsedManifest *Manifest, info updateinfo.T, packageName string) error {
	if len(parsedManifest.URIFormat) == 0 {
		return fmt.Errorf("folder format cannot be null in the Manifest file")
	}
	fileName := info.GenerateCompressedFileName(packageName)
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
	info updateinfo.T) (isValid bool, err error) {

	fileName := info.GenerateCompressedFileName(packageName)
	for _, p := range parsedManifest.Packages {
		if p.Name == packageName {
			log.Infof("found package %v", packageName)
			for _, f := range p.Files {
				if f.Name == fileName {
					for _, v := range f.AvailableVersions {
						if v.Version == version {
							status := updateconstants.ActiveVersionStatus
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
	info updateinfo.T) (isValid bool, err error) {

	fileName := info.GenerateCompressedFileName(packageName)
	for _, p := range parsedManifest.Packages {
		if p.Name == packageName {
			log.Infof("found package %v", packageName)
			for _, f := range p.Files {
				if f.Name == fileName {
					for _, v := range f.AvailableVersions {
						if v.Version == version {
							status := updateconstants.ActiveVersionStatus
							if v.Status != "" {
								status = v.Status
							}

							log.Infof("Version %v status is %v", version, status)
							return v.Status == updateconstants.DeprecatedVersionStatus, nil
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
	case updateconstants.ActiveVersionStatus:
		return true
	case updateconstants.InactiveVersionStatus:
		return false
	case updateconstants.DeprecatedVersionStatus:
		return false
	case "":
		return true
	default:
		return false
	}
}
