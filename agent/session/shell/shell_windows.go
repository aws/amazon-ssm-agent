// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
//
// +build windows

// Package shell implements session shell plugin.
package shell

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/session/shell/execcmd"
	"github.com/google/shlex"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	agentContracts "github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	mgsConfig "github.com/aws/amazon-ssm-agent/agent/session/config"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	"github.com/aws/amazon-ssm-agent/agent/session/utility"
	"github.com/aws/amazon-ssm-agent/agent/session/winpty"
)

var pty *winpty.WinPTY
var u = &utility.SessionUtil{}
var token syscall.Token
var profile syscall.Handle

const (
	defaultConsoleCol                                = 200
	defaultConsoleRow                                = 60
	winptyDllName                                    = "winpty.dll"
	winptyDllFolderName                              = "SessionManagerShell"
	winptyCmd                                        = "powershell"
	startRecordSessionCmd                            = "Start-Transcript"
	newLineCharacter                                 = "\r\n"
	shellProfileNewLineCharacter                     = "\r"
	screenBufferSizeCmd                              = "$host.UI.RawUI.BufferSize = New-Object System.Management.Automation.Host.Size($host.UI.RawUI.BufferSize.Width,%d)%s"
	powerShellTranscriptLoggingSupportedMajorVersion = 5
	powerShellTranscriptLoggingSupportedMinorVersion = 1
	transcriptDirCustomPath                          = `Amazon/SSM/Session/`
	dateformatyyyymmdd                               = "20060102"
)

var (
	winptyDllDir      = fileutil.BuildPath(appconfig.DefaultPluginPath, winptyDllFolderName)
	winptyDllFilePath = filepath.Join(winptyDllDir, winptyDllName)
)

//StartCommandExecutor starts winpty agent and provides handles to stdin and stdout.
// isSessionLogger determines whether its a customer shell or shell used for logging.
func StartCommandExecutor(
	log log.T,
	shellProps mgsContracts.ShellProperties,
	isSessionLogger bool,
	config agentContracts.Configuration,
	plugin *ShellPlugin) (err error) {

	log.Info("Starting command executor")
	if _, err := os.Stat(winptyDllFilePath); os.IsNotExist(err) {
		return fmt.Errorf("Missing %s file.", winptyDllFilePath)
	}

	var finalCmd string
	if strings.TrimSpace(shellProps.Windows.Commands) == "" || isSessionLogger {
		finalCmd = winptyCmd
	} else {
		finalCmd = winptyCmd + " " + shellProps.Windows.Commands
	}

	appConfig, _ := appconfig.Config(false)

	if !shellProps.Windows.RunAsElevated && !isSessionLogger && !appConfig.Agent.ContainerMode {
		// Reset password for default ssm user
		var newPassword string
		newPassword, err = u.GeneratePasswordForDefaultUser()
		if err != nil {
			return err
		}
		var userExists bool
		if userExists, err = u.ChangePassword(appconfig.DefaultRunAsUserName, newPassword); err != nil {
			log.Errorf("Failed to generate new password for %s: %v", appconfig.DefaultRunAsUserName, err)
			return
		}

		// create ssm-user before starting a new session
		if !userExists {
			if newPassword, err = u.CreateLocalAdminUser(log); err != nil {
				return fmt.Errorf("Failed to create user %s: %v", appconfig.DefaultRunAsUserName, err)
			}
		} else {
			// enable user
			if err = u.EnableLocalUser(log); err != nil {
				return fmt.Errorf("Failed to enable user %s: %v", appconfig.DefaultRunAsUserName, err)
			}
		}

		if appconfig.PluginNameNonInteractiveCommands == plugin.name {
			if token, profile, err = u.LoadUserProfile(appconfig.DefaultRunAsUserName, newPassword); err != nil {
				return fmt.Errorf("error loading user profile: %v", err)
			}
			return plugin.startExecCmd(finalCmd, log, config)
		}

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Errorf("Start pty as user panic: \n%v", r)
					log.Errorf("Stacktrace:\n%s", debug.Stack())
				}
			}()
			defer wg.Done()
			plugin.logger.transcriptDirPath, err = plugin.startPtyAsUser(log, config, appconfig.DefaultRunAsUserName, newPassword, finalCmd)
		}()
		wg.Wait()
	} else if !isSessionLogger && appconfig.PluginNameNonInteractiveCommands == plugin.name {
		return plugin.startExecCmd(finalCmd, log, config)
	} else {
		pty, err = winpty.Start(winptyDllFilePath, finalCmd, defaultConsoleCol, defaultConsoleRow, winpty.DEFAULT_WINPTY_FLAGS)
	}

	if err != nil {
		return err
	}

	plugin.stdin = pty.StdIn
	plugin.stdout = pty.StdOut
	plugin.runAsUser = appconfig.DefaultRunAsUserName

	return err
}

func (p *ShellPlugin) startExecCmd(finalCmd string, log log.T, config agentContracts.Configuration) (err error) {
	var cmd *exec.Cmd
	commands, err := shlex.Split(finalCmd)
	if err != nil {
		return fmt.Errorf("Failed to parse commands input: %s\n", err)
	}
	if len(commands) > 1 {
		cmd = exec.Command(commands[0], commands[1:]...)
	} else {
		cmd = exec.Command(commands[0])
	}

	outputPath := filepath.Join(config.OrchestrationDirectory, mgsConfig.ExecOutputFileName)
	outputWriter, err := os.OpenFile(outputPath, appconfig.FileFlagsCreateOrAppendReadWrite, appconfig.ReadWriteAccess)
	if err != nil {
		return fmt.Errorf("Failed to open file for writing command output. error: %s\n", err)
	}
	outputReader, err := os.Open(outputPath)
	if err != nil {
		return fmt.Errorf("Failed to read command output from file %s. error: %s\n", outputPath, err)
	}
	cmd.Stdout = outputWriter
	cmd.Stderr = outputWriter
	cmd.SysProcAttr = &syscall.SysProcAttr{Token: token}
	p.runAsUser = appconfig.DefaultRunAsUserName
	p.stdin = nil
	p.stdout = outputReader
	p.execCmd = execcmd.NewExecCmd(cmd)
	return nil
}

//stop closes winpty process handle and stdin/stdout.
func (p *ShellPlugin) stop(log log.T) (err error) {
	if pty != nil {
		log.Info("Stopping winpty")
		if err = pty.Close(); err != nil {
			return fmt.Errorf("Stop winpty failed: %s", err)
		}
	}

	log.Debugf("Disabling ssm-user")
	u.DisableLocalUser(log)

	if token != 0 && profile != 0 {
		u.UnloadUserProfile(log, token, profile)
	}

	return nil
}

//SetSize sets size of console terminal window.
func SetSize(log log.T, ws_col, ws_row uint32) (err error) {
	if pty == nil {
		return nil
	}
	if err = pty.SetSize(ws_col, ws_row); err != nil {
		return fmt.Errorf("Set winpty size failed: %s", err)
	}

	return nil
}

//startPtyAsUser starts a winpty process in runas user context.
func (p *ShellPlugin) startPtyAsUser(log log.T, config agentContracts.Configuration, user string, pass string, shellCmd string) (transcriptDirPath string, err error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// CloudWatch streaming depends on PowerShell's Transcript logging feature.
	// If streaming enabled:
	// 1) then load user profile and get handle
	// 2) fetch user profile directory
	// 3) use profile directory as transcript output path and enable transcript logging
	if p.logger.streamLogsToCloudWatch {
		log.Debugf("Load UserProfile %s", user)
		if token, profile, err = u.LoadUserProfile(user, pass); err != nil {
			return "", fmt.Errorf("error loading user profile: %v", err)
		}

		var profileDir string
		if profileDir, err = syscall.Token(token).GetUserProfileDirectory(); err != nil {
			return "", fmt.Errorf("error fetching user profile directory: %v", err)
		}
		log.Debugf("Fetched user profile directory, %s", profileDir)
		transcriptDirPath = path.Join(profileDir, transcriptDirCustomPath, config.SessionId)

		log.Debugf("Enable PowerShell's Transcript logging with output directory as: %s", transcriptDirPath)
		if err = u.EnablePowerShellTranscription(transcriptDirPath, profile); err != nil {
			return "", fmt.Errorf("error enabling powershell transcription: %v", err)
		}
	}

	// Impersonate current thread as runAs user
	log.Debugf("Impersonating %s", user)
	if err = u.Impersonate(log, user, pass); err != nil {
		return "", fmt.Errorf("error impersonating: %v", err)
	}

	// Start Winpty under the user context thread.
	if pty, err = winpty.Start(winptyDllFilePath, shellCmd, defaultConsoleCol, defaultConsoleRow, winpty.WINPTY_FLAG_IMPERSONATE_THREAD); err != nil {
		log.Error(err)
		return
	}

	// Revert thread to original context
	if err = u.RevertToSelf(); err != nil {
		log.Error(err)
		return
	}
	log.Debug("Reverted to system profile.")

	return
}

// runShellProfile executes the shell profile config
func (p *ShellPlugin) runShellProfile(log log.T, config agentContracts.Configuration) error {
	if strings.TrimSpace(config.ShellProfile.Windows) == "" {
		return nil
	}
	if p.stdin == nil {
		return nil
	}
	commands := strings.Split(config.ShellProfile.Windows, "\n")

	for _, command := range commands {
		if _, err := p.stdin.Write([]byte(command + shellProfileNewLineCharacter)); err != nil {
			log.Errorf("Unable to write to stdin, err: %v.", err)
			return err
		}
	}
	return nil
}

// generateLogData generates a log file with the executed commands.
func (p *ShellPlugin) generateLogData(log log.T, config agentContracts.Configuration) error {
	platformVersion, _ := platform.PlatformVersion(log)

	osVersionSplit := strings.Split(platformVersion, ".")
	if osVersionSplit == nil || len(osVersionSplit) < 2 {
		return fmt.Errorf("error occurred while parsing OS version: %s", platformVersion)
	}

	// check if the OS version is 6.1 or higher
	osMajorVersion, err := strconv.Atoi(osVersionSplit[0])
	if err != nil {
		return err
	}

	osMinorVersion, err := strconv.Atoi(osVersionSplit[1])
	if err != nil {
		return err
	}

	// Generate logs based on the OS version number
	// https://docs.microsoft.com/en-us/windows/desktop/SysInfo/operating-system-version
	if osMajorVersion >= 10 {
		if err = p.generateTranscriptFile(log, p.logger.logFilePath, p.logger.ipcFilePath, true, config); err != nil {
			return err
		}
	} else if osMajorVersion >= 6 && osMinorVersion >= 3 {
		transcriptFile := filepath.Join(config.OrchestrationDirectory, "transcriptFile"+mgsConfig.LogFileExtension)
		if err = p.generateTranscriptFile(log, transcriptFile, p.logger.ipcFilePath, false, config); err != nil {
			return err
		}
		cleanControlCharacters(transcriptFile, p.logger.logFilePath)
	} else {
		cleanControlCharacters(p.logger.ipcFilePath, p.logger.logFilePath)
	}

	return nil
}

// generateTranscriptFile generates a transcript file using PowerShell
func (p *ShellPlugin) generateTranscriptFile(
	log log.T,
	transcriptFile string,
	loggerFile string,
	enableVirtualTerminalProcessingForWindows bool,
	config agentContracts.Configuration) error {
	err := StartCommandExecutor(log, mgsContracts.ShellProperties{}, true, config, p)
	if err != nil {
		return err
	}

	defer func() {
		if err := recover(); err != nil {
			if err = p.stop(log); err != nil {
				log.Errorf("Error occured while closing pty: %v", err)
			}
		}
	}()

	time.Sleep(5 * time.Second)

	// Increase buffer size
	screenBufferSizeCmdInput := fmt.Sprintf(screenBufferSizeCmd, mgsConfig.ScreenBufferSize, newLineCharacter)
	p.stdin.Write([]byte(screenBufferSizeCmdInput))

	time.Sleep(5 * time.Second)

	// Start shell recording
	recordCmdInput := fmt.Sprintf("%s %s%s", startRecordSessionCmd, transcriptFile, newLineCharacter)
	p.stdin.Write([]byte(recordCmdInput))

	time.Sleep(5 * time.Second)

	// Start shell logger
	loggerCmdInput := fmt.Sprintf("%s %s %t%s", appconfig.DefaultSessionLogger, loggerFile, enableVirtualTerminalProcessingForWindows, newLineCharacter)
	p.stdin.Write([]byte(loggerCmdInput))

	// Sleep till the logger completes execution
	time.Sleep(time.Minute)

	// Exit shell
	exitCmdInput := fmt.Sprintf("%s%s", mgsConfig.Exit, newLineCharacter)
	p.stdin.Write([]byte(exitCmdInput))

	// Sleep till the shell successfully exits before uploading
	time.Sleep(5 * time.Second)

	return nil
}

// cleanControlCharacters cleans up control characters from the log file
func cleanControlCharacters(sourceFileName, destinationFileName string) error {
	sourceFile, err := os.Open(sourceFileName)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destinationFile, err := os.Create(destinationFileName)
	if err != nil {
		return err
	}
	defer destinationFile.Close()

	escapeCharRegEx := regexp.MustCompile(`←`)
	specialChar1RegEx := regexp.MustCompile(`\[\?[0-9]+[a-zA-Z]`)
	specialChar2RegEx := regexp.MustCompile(`\[[0-9]+[A-Z]`)
	newLineCharRegEx := regexp.MustCompile(`\[0K`)

	emptyString := []byte("")
	scanner := bufio.NewScanner(sourceFile)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		var line []byte
		line = append(line, scanner.Bytes()...)
		line = escapeCharRegEx.ReplaceAll(line, emptyString)
		line = specialChar1RegEx.ReplaceAll(line, emptyString)
		line = specialChar2RegEx.ReplaceAll(line, emptyString)
		line = newLineCharRegEx.ReplaceAll(line, emptyString)

		// clean up pending escape characters if any
		var output []byte
		for _, v := range line {
			if v == 27 {
				output = append(output, emptyString...)
			} else {
				output = append(output, v)
			}
		}

		destinationFile.Write(append(output, []byte(newLineCharacter)...))
	}
	return nil
}

// checkForLoggingInterruption is used to detect if log streaming to CW has been interrupted
var checkForLoggingInterruption = func(log log.T, ipcFile *os.File, plugin *ShellPlugin) {
	// nothing to detect in case of windows
}

// isLogStreamingSupported checks if streaming of logs is supported since it depends on PowerShell's transcript logging
func (p *ShellPlugin) isLogStreamingSupported(log log.T) (bool, error) {
	if appconfig.PluginNameNonInteractiveCommands == p.name {
		return false, nil
	}
	if powerShellVersionSupportedForLogStreaming, err := isPowerShellVersionSupportedForLogStreaming(log); err != nil {
		return false, fmt.Errorf("PowerShell version can't be verified on the instance. No logs will be streamed to CloudWatch. The error is: %v", err)
	} else if !powerShellVersionSupportedForLogStreaming {
		return false, errors.New(mgsConfig.UnsupportedPowerShellVersionForStreamingErrorMsg)
	}

	if systemLevelPowerShellTranscriptLoggingConfigured := u.IsSystemLevelPowerShellTranscriptionConfigured(log); systemLevelPowerShellTranscriptLoggingConfigured {
		return false, errors.New(mgsConfig.PowerShellTranscriptLoggingEnabledErrorMsg)
	}

	log.Debug("Streaming of logs is supported.")
	return true, nil
}

// isPowerShellVersionSupportedForLogStreaming checks if PowerShell's version is 5.1 or higher in order to support streaming of logs.
// Streaming of logs depends on PowerShell's transcript logging feature which is supported from version 5.1 or higher.
func isPowerShellVersionSupportedForLogStreaming(log log.T) (bool, error) {
	powerShellVersion, err := u.GetInstalledVersionOfPowerShell()
	if err != nil {
		return false, fmt.Errorf("unable to get installed version of PowerShell, err: %v", err)
	}
	log.Debugf("Installed version of PowerShell is: %s", powerShellVersion)

	powerShellVersionSplit := strings.Split(powerShellVersion, ".")
	if powerShellVersionSplit == nil || len(powerShellVersionSplit) < 2 {
		return false, fmt.Errorf("error occurred while parsing PowerShell version")
	}

	powerShellMajorVersion, err := strconv.Atoi(powerShellVersionSplit[0])
	if err != nil {
		return false, fmt.Errorf("error occurred while parsing PowerShell version, err: %v", err)
	}

	powerShellMinorVersion, err := strconv.Atoi(powerShellVersionSplit[1])
	if err != nil {
		return false, fmt.Errorf("error occurred while parsing PowerShell version, err: %v", err)
	}

	// return true if the PowerShell version is 5.1 or higher for Transcript Logging to work
	if powerShellMajorVersion < powerShellTranscriptLoggingSupportedMajorVersion {
		return false, nil
	} else if powerShellMajorVersion == powerShellTranscriptLoggingSupportedMajorVersion && powerShellMinorVersion < powerShellTranscriptLoggingSupportedMinorVersion {
		return false, nil
	} else {
		return true, nil
	}
}

// getStreamingFilePath returns the file path of transcript log file created by PowerShell
func (p *ShellPlugin) getStreamingFilePath(log log.T) (streamingFilePath string, err error) {
	currentDate := fmt.Sprintf(time.Now().Format(dateformatyyyymmdd))
	dirPath := fmt.Sprintf(p.logger.transcriptDirPath + `/` + currentDate)

	// Check periodically for the presence of transcript file. Ideally file should be created as soon as shell starts.
	ticker := time.NewTicker(time.Second)
	for range ticker.C {
		files, err := ioutil.ReadDir(dirPath)
		if err != nil {
			return "", fmt.Errorf("error reading dir path %s, err: %v", p.logger.transcriptDirPath, err)
		}

		// Continue to check for the presence of the file, break out of the loop when found
		if files == nil {
			continue
		} else {
			streamingFilePath = dirPath + `/` + files[0].Name()
			log.Debugf("Transcript logging file path is: %s", streamingFilePath)
			break
		}
	}

	return
}

// isCleanupOfControlCharactersRequired returns true/false depending on whether log needs to be cleanup of control characters before streaming to destination
func (p *ShellPlugin) isCleanupOfControlCharactersRequired() bool {
	// Windows streaming of logs depends on PowerShell's transcript logging which takes care of control characters
	// and no additional cleanup is required.
	return false
}

//cleanupLogFile cleans up temporary log file on disk created by PowerShell's transcript logging
func (p *ShellPlugin) cleanupLogFile(log log.T, ipcFile *os.File) {
	if p.logger.transcriptDirPath != "" {
		log.Debugf("Deleting transcript directory: %s", p.logger.transcriptDirPath)
		if err := os.RemoveAll(p.logger.transcriptDirPath); err != nil {
			log.Debugf("Encountered error deleting transcript directory: %v", err)
		}
	}
}

// InputStreamMessageHandler passes payload byte stream to shell command executor
func (p *ShellPlugin) InputStreamMessageHandler(log log.T, streamDataMessage mgsContracts.AgentMessage) error {
	var isPluginNonInteractive = appconfig.PluginNameNonInteractiveCommands == p.name

	if !isPluginNonInteractive && (p.stdin == nil || p.stdout == nil) {
		// This is to handle scenario when cli/console starts sending size data but pty has not been started yet
		// Since packets are rejected, cli/console will resend these packets until pty starts successfully in separate thread
		log.Tracef("Pty unavailable. Reject incoming message packet")
		return mgsContracts.ErrHandlerNotReady
	}

	switch mgsContracts.PayloadType(streamDataMessage.PayloadType) {
	case mgsContracts.Output:
		log.Tracef("Output message received: %d", streamDataMessage.SequenceNumber)

		if isPluginNonInteractive {
			var signal os.Signal = nil
			for _, message := range streamDataMessage.Payload {
				if sig, exists := appconfig.ByteControlSignalsWindows[message]; exists {
					log.Debugf("Received control signal. message: %v, signal: %v", string(message), sig)
					signal = sig
					break
				}
			}
			if signal != nil {
				defer func() {
					if err := p.execCmd.Wait(); err != nil {
						log.Errorf("Error received after processing control signal: %s", err)
					}
				}()
				if err := p.execCmd.Signal(signal); err != nil {
					log.Errorf("Sending signal %v to command process %v failed with error %v", signal, p.execCmd.Pid(), err)
					return err
				}
			}
			return nil
		}

		// deal with powershell nextline issue https://github.com/lzybkr/PSReadLine/issues/579
		payloadString := string(streamDataMessage.Payload)
		if strings.Contains(payloadString, "\r\n") {
			// From windows machine, do nothing
		} else if strings.Contains(payloadString, "\n") {
			// From linux machine, replace \n with \r
			payloadString = strings.Replace(payloadString, "\n", "\r", -1)
		}

		if _, err := p.stdin.Write([]byte(payloadString)); err != nil {
			log.Errorf("Unable to write to stdin, err: %v.", err)
			return err
		}
	case mgsContracts.Size:
		// Do not handle terminal resize for non-interactive plugin as there is no pty
		if isPluginNonInteractive {
			log.Debug("Terminal resize message is ignored in NonInteractiveCommands plugin")
			return nil
		}
		var size mgsContracts.SizeData
		if err := json.Unmarshal(streamDataMessage.Payload, &size); err != nil {
			log.Errorf("Invalid size message: %s", err)
			return err
		}
		log.Tracef("Resize data received: cols: %d, rows: %d", size.Cols, size.Rows)
		if err := SetSize(log, size.Cols, size.Rows); err != nil {
			log.Errorf("Unable to set pty size: %s", err)
			return err
		}
	}
	return nil
}
