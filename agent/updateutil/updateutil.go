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
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/s3util"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateinfo"
	"github.com/aws/amazon-ssm-agent/agent/versionutil"
	"github.com/aws/amazon-ssm-agent/common/identity"
	identity2 "github.com/aws/amazon-ssm-agent/common/identity/identity"
	"github.com/aws/amazon-ssm-agent/core/executor"
	"github.com/aws/amazon-ssm-agent/core/workerprovider/longrunningprovider/model"
)

// T represents the interface for Update utility
type T interface {
	CreateUpdateDownloadFolder() (folder string, err error)
	ExeCommand(log log.T, cmd string, workingDir string, updaterRoot string, stdOut string, stdErr string, isAsync bool) (pid int, exitCode updateconstants.UpdateScriptExitCode, err error)
	ExecCommandWithOutput(log log.T, cmd string, workingDir string, updaterRoot string, stdOut string, stdErr string) (pId int, updExitCode updateconstants.UpdateScriptExitCode, stdoutBytes *bytes.Buffer, errorBytes *bytes.Buffer, cmdErr error)
	IsServiceRunning(log log.T, i updateinfo.T) (result bool, err error)
	IsWorkerRunning(log log.T) (result bool, err error)
	WaitForServiceToStart(log log.T, i updateinfo.T, targetVersion string) (result bool, err error)
	SaveUpdatePluginResult(log log.T, updaterRoot string, updateResult *UpdatePluginResult) (err error)
	IsDiskSpaceSufficientForUpdate(log log.T) (bool, error)
	UpdateInstallDelayer(ctx context.T, updateRoot string) error
	LoadUpdateDocumentState(ctx context.T, commandId string) error
	UpdateExecutionTimeOut(int)
	GetExecutionTimeOut() int
}

// Utility implements interface T
type Utility struct {
	Context                               context.T
	CustomUpdateExecutionTimeoutInSeconds int
	ProcessExecutor                       executor.IExecutor
	UpdateDocState                        contracts.DocumentState
}

var getDiskSpaceInfo = fileutil.GetDiskSpaceInfo
var getRemoteProvider = identity2.GetRemoteProvider
var mkDirAll = os.MkdirAll
var openFile = os.OpenFile
var execCommand = exec.Command
var cmdStart = (*exec.Cmd).Start
var cmdOutput = (*exec.Cmd).Output

const (
	stateJson = "updatestate.json"
)

func NewUpdaterUtilWithLoadedDocContent(ctx context.T, commandId string) *Utility {
	updateUtil := &Utility{
		Context: ctx,
	}
	err := updateUtil.LoadUpdateDocumentState(ctx, commandId)
	if err != nil {
		ctx.Log().Warnf("Could not load doc content for commandID /%v/: %v", commandId, err)
	}
	return updateUtil
}

// CreateInstanceInfo create instance related information such as region, platform and arch
func (util *Utility) CreateInstanceInfo(log log.T) (context updateinfo.T, err error) {
	return nil, nil
}

// CreateUpdateDownloadFolder creates folder for storing update downloads
func (util *Utility) CreateUpdateDownloadFolder() (folder string, err error) {
	root := filepath.Join(appconfig.DownloadRoot, "update")
	if err = mkDirAll(root, os.ModePerm|os.ModeDir); err != nil {
		return "", err
	}

	return root, nil
}

// UpdateExecutionTimeOut updates the command execution timeout
func (util *Utility) UpdateExecutionTimeOut(timeout int) {
	util.CustomUpdateExecutionTimeoutInSeconds = timeout
}

// GetExecutionTimeOut returns the command execution timeout
func (util *Utility) GetExecutionTimeOut() int {
	return util.CustomUpdateExecutionTimeoutInSeconds
}

// ExecCommandWithOutput executes shell command and returns output and error of command execution
func (util *Utility) ExecCommandWithOutput(
	log log.T,
	cmd string,
	workingDir string,
	outputRoot string,
	stdOut string,
	stdErr string) (pId int, updExitCode updateconstants.UpdateScriptExitCode, stdoutBytes *bytes.Buffer, errorBytes *bytes.Buffer, cmdErr error) {

	parts := strings.Fields(cmd)
	pid := -1
	var updateExitCode updateconstants.UpdateScriptExitCode = -1
	tempCmd := setPlatformSpecificCommand(parts)
	command := execCommand(tempCmd[0], tempCmd[1:]...)
	command.Dir = workingDir
	stdoutWriter, stderrWriter, err := setExeOutErr(outputRoot, stdOut, stdErr)
	if err != nil {
		return pid, updateExitCode, nil, nil, err
	}
	defer stdoutWriter.Close()
	defer stderrWriter.Close()
	var errBytes, stdOutBytes bytes.Buffer
	command.Stdout = &stdOutBytes
	command.Stderr = &errBytes

	err = cmdStart(command)
	if err != nil {
		return pid, updateExitCode, &stdOutBytes, &errBytes, err
	}

	pid = GetCommandPid(command)

	var timeout = updateconstants.DefaultUpdateExecutionTimeoutInSeconds
	if util.CustomUpdateExecutionTimeoutInSeconds != 0 {
		timeout = util.CustomUpdateExecutionTimeoutInSeconds
	}
	timer := time.NewTimer(time.Duration(timeout) * time.Second)
	go killProcessOnTimeout(log, command, timer)
	err = command.Wait()
	if errBytes.Len() != 0 {
		stderrWriter.Write(errBytes.Bytes())
	}
	if stdOutBytes.Len() != 0 {
		stdoutWriter.Write(stdOutBytes.Bytes())
	}
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
		return pid, updateExitCode, &stdOutBytes, &errBytes, err
	}
	return pid, updateExitCode, &stdOutBytes, nil, nil
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
		util.setCommandEnvironmentVariables(command)
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
	util.setCommandEnvironmentVariables(command)
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
	output = string(out)
	// Write the returned output so that we can upload it if needed
	stdoutWriter.Write(out)
	if err != nil {
		return
	}

	return output, err
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
	util.setCommandEnvironmentVariables(command)
	// Don't set command.Stdout - we're going to return it instead of writing it
	command.Stderr = stderrWriter

	// Run the command and return its output
	var out []byte
	out, err = cmdOutput(command)
	output = string(out)
	// Write the returned output so that we can upload it if needed
	stdoutWriter.Write(out)
	if err != nil {
		return
	}

	return output, err
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

// ConvertToUpdateErrorCode joins the error string array passed and converts to update error code
func ConvertToUpdateErrorCode(errorCodes ...string) updateconstants.ErrorCode {
	var updateErrorCode bytes.Buffer
	for _, errCode := range errorCodes {
		updateErrorCode.WriteString(errCode)
	}
	return updateconstants.ErrorCode(updateErrorCode.String())
}

// GetManifestURLFromSourceUrl parses source url passed to the updater and generates the url for manifest
func GetManifestURLFromSourceUrl(sourceURL string) (string, error) {
	u, err := url.Parse(sourceURL)

	if err != nil {
		return "", err
	}

	pathArgs := strings.Split(u.Path, "/")
	if len(pathArgs) < 4 || len(pathArgs) > 5 {
		return "", fmt.Errorf("URL does not have expected path structure: %s", sourceURL)
	} else if len(pathArgs) == 4 {
		// Case for: https://{bucket}.s3.{region}.amazonaws.com/amazon-ssm-agent/{version}/amazon-ssm-agent.tar.gz
		u.Path = ""
	} else {
		// Case for: https://s3.{region}.amazonaws.com/{bucket}/amazon-ssm-agent/{version}/amazon-ssm-agent.tar.gz
		u.Path = "/" + pathArgs[1]
	}

	u.Path += "/" + updateconstants.ManifestFile

	return u.String(), nil
}

func GetStableURLFromManifestURL(manifestURL string, identity identity.IAgentIdentity) (string, error) {
	if !strings.HasSuffix(manifestURL, "/"+updateconstants.ManifestFile) {
		return "", fmt.Errorf("unexpected manifest url does not end with manifest file: %s", manifestURL)
	}
	stableVersionPath := "stable/VERSION"
	region, err := identity.Region()
	if err != nil {
		return "", err
	}

	stableVersionURL := strings.TrimRight(manifestURL, updateconstants.ManifestFile) + stableVersionPath
	stableVersionURL = strings.Replace(stableVersionURL, updateconstants.RegionHolder, region, -1)
	return stableVersionURL, nil
}

// ResolveAgentReleaseBucketURL makes best effort to generate an url for the ssm agent bucket
func ResolveAgentReleaseBucketURL(region string, identity identity.IAgentIdentity) string {
	s3Url := ""
	if dynamicS3Endpoint := identity.GetServiceEndpoint("s3"); dynamicS3Endpoint != "" {
		s3Url = "https://" + dynamicS3Endpoint
	} else if strings.HasPrefix(region, s3util.ChinaRegionPrefix) {
		s3Url = updateconstants.ChinaS3URL
	} else {
		s3Url = updateconstants.CommonS3URL
	}

	return strings.Replace(s3Url+updateconstants.BucketPath, updateconstants.RegionHolder, region, -1)
}

// IsV1UpdatePlugin returns true if source agent version is equal or below 3.0.882.0, any error defaults to false
//
//	this logic is required since moving logic from plugin to updater would otherwise lead
//	to duplicate aws console logging when upgrading from V1UpdatePlugin agents
func IsV1UpdatePlugin(SourceVersion string) bool {
	const LastV1UpdatePluginAgentVersion = "3.0.882.0"

	comp, err := versionutil.VersionCompare(SourceVersion, LastV1UpdatePluginAgentVersion)
	return err == nil && comp <= 0
}

// IsIdentityRuntimeConfigSupported returns true if source agent version is greater than 3.1.282.0, any error defaults to false
// this logic is required to ensure that updater does not rely on an old runtime config if an agent has been downgraded
// from a version that supported identity runtimeconfig and also to skip the runtimeconfig check when we know it does not exist
func IsIdentityRuntimeConfigSupported(sourceVersion string) bool {
	const LastAgentVersionBeforeIdentityRuntimeConfig = "3.1.282.0"

	comp, err := versionutil.VersionCompare(sourceVersion, LastAgentVersionBeforeIdentityRuntimeConfig)
	return err == nil && comp > 0
}

func (util *Utility) setCommandEnvironmentVariables(command *exec.Cmd) {
	credentialProvider, ok := getRemoteProvider(util.Context.Identity())
	if !ok {
		return
	}

	// Don't set environment variables if credentials are not being shared
	if !credentialProvider.SharesCredentials() {
		return
	}

	osEnv := os.Environ()
	command.Env = osEnv
	if _, ok := os.LookupEnv("AWS_SHARED_CREDENTIALS_FILE"); !ok && credentialProvider.ShareFile() != "" {
		command.Env = append(command.Env, fmt.Sprintf("%s=%s", "AWS_SHARED_CREDENTIALS_FILE", credentialProvider.ShareFile()))
	}

	if _, ok := os.LookupEnv("AWS_PROFILE"); !ok && credentialProvider.ShareProfile() != "" {
		command.Env = append(command.Env, fmt.Sprintf("%s=%s", "AWS_PROFILE", credentialProvider.ShareProfile()))
	}

}
