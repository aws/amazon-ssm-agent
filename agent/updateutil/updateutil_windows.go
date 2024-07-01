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

//go:build windows
// +build windows

// Package updateutil contains updater specific utilities.
package updateutil

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/backoffconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	model "github.com/aws/amazon-ssm-agent/agent/runcommand/contracts"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
	"github.com/cenkalti/backoff/v4"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

var (
	getPlatformSku                  = platform.PlatformSku
	readDir                         = fileutil.ReadDir
	unmarshallFile                  = jsonutil.UnmarshalFile
	sleep                           = time.Sleep
	fileExists                      = fileutil.Exists
	backoffConfigExponential        = backoffconfig.GetExponentialBackoff
	backOffRetry                    = backoff.Retry
	deleteFile                      = fileutil.DeleteFile
	fileWrite                       = fileutil.WriteIntoFileWithPermissions
	wmicCommand                     = filepath.Join(appconfig.EnvWinDir, "System32", "wbem", "wmic.exe")
	getVersionThroughRegistryKeyRef = getVersionThroughRegistryKey
	getVersionThroughWMIRef         = getVersionThroughWMI
)

type UpdatePluginRunState struct {
	CommandId string
	RunCount  int
}

func prepareProcess(command *exec.Cmd) {
}

// UpdateInstallDelayer delays the agent install when domain join reboot doc found
func (util *Utility) UpdateInstallDelayer(ctx context.T, updateRoot string) error {
	// adding panic handler to be on the safer side
	defer func() {
		if r := recover(); r != nil {
			ctx.Log().Errorf("panic while executing install delayer: %v", r)
		}
	}()

	instanceID, err := ctx.Identity().InstanceID()
	if err != nil {
		return fmt.Errorf("could not fetch instance id: %v", err)
	}
	if util.UpdateDocState.DocumentInformation.CommandID == "" {
		return fmt.Errorf("docstate is not loaded")
	}
	// command id blank also says that the document state is not loaded in util

	exponentialBackOff, err := backoffConfigExponential(200*time.Millisecond, 2) // only 2 retries
	if err != nil {
		return fmt.Errorf("could initialize exponential backoff: %v", err)
	}

	tempState, err := getPluginState(updateRoot, exponentialBackOff)
	if err != nil && tempState.CommandId == "" {
		removePluginState(ctx.Log(), updateRoot, exponentialBackOff)
		return fmt.Errorf("update state json could not be retrieved: %v", err)
	}

	if tempState.CommandId != util.UpdateDocState.DocumentInformation.CommandID {
		tempState.RunCount = 0
		tempState.CommandId = util.UpdateDocState.DocumentInformation.CommandID
	}
	tempState.RunCount = tempState.RunCount + 1

	if tempState.RunCount == 2 {
		removePluginState(ctx.Log(), updateRoot, exponentialBackOff)
		ctx.Log().Debugf("update run count exceeded the limit - command: /%v/", util.UpdateDocState.DocumentInformation.CommandID)
		return nil
	}

	// * Check whether document containing domain join plugin is in reboot state
	// * If in reboot state, the following steps will happen
	//      * In Iteration 1, the update doc will be saved to Agent queue to be processed again
	//		* In Iteration 1, the update state will also be saved to make sure that Update document does not go into a loop after we push into Agent queue
	//		* wait for 3 mins(30 secs * 6) for the document to change state. If changed, the update doc will be removed from Agent queue and the update state json will be removed
	// * If not in reboot state, proceed to install agent
	// NOTE: The update doc will be loaded immediately when the updater is started as it will be removed by document worker after few seconds
	for i := 0; i < 7; i++ {
		if !isDomainJoinPluginInReboot(ctx, instanceID) || i == 7 {
			removePluginState(ctx.Log(), updateRoot, exponentialBackOff)
			removeUpdateDocFromAgentCurrentQueue(ctx.Log(), util.UpdateDocState.DocumentInformation.CommandID, instanceID, exponentialBackOff)
			break
		}
		if i == 0 {
			err = savePluginState(updateRoot, tempState, exponentialBackOff)
			if err != nil {
				return fmt.Errorf("error while saving plugin state: %v", err)
			}

			err = pushUpdateDocToAgentCurrentQueue(util.UpdateDocState.DocumentInformation.CommandID, instanceID, util.UpdateDocState, exponentialBackOff)
			if err != nil {
				return fmt.Errorf("error while pushing to agent queue: %v", err)
			}
		}
		time.Sleep(30 * time.Second)
	}
	ctx.Log().Debugf("done with install delay check")
	return nil
}

func isDomainJoinPluginInReboot(ctx context.T, instanceID string) bool {
	log := ctx.Log()

	filepath := path.Join(appconfig.DefaultDataStorePath,
		instanceID,
		appconfig.DefaultDocumentRootDirName,
		appconfig.DefaultLocationOfState,
		appconfig.DefaultLocationOfCurrent)

	fileNames := make([]string, 0)
	tmpFileNames, _ := readDir(filepath)
	for _, tmpFileName := range tmpFileNames {
		fileNameWithFullPath := path.Join(filepath, tmpFileName.Name())
		fileNames = append(fileNames, fileNameWithFullPath)
	}

	// Read documents and check for domain join plugin
	// Return true when domain join plugin found
	var docState contracts.DocumentState
	for _, fileName := range fileNames {
		count, retryLimit := 0, 2
		for count < retryLimit {
			ctx.Log().Debugf("checking doc state file: %v", fileName)
			err := unmarshallFile(fileName, &docState)
			if err != nil {
				log.Warnf("encountered error while reading file /%v/: %v", fileName, err)
				count += 1
				sleep(100 * time.Millisecond)
				continue
			}
			if docState.IsRebootRequired() {
				for _, plugin := range docState.InstancePluginsInformation {
					if strings.ToLower(plugin.Name) == strings.ToLower(appconfig.PluginNameDomainJoin) {
						log.Infof("domain join doc with reboot state found: /%v/", docState.DocumentInformation.CommandID)
						return true
					}
				}
			}
			break
		}
	}
	return false
}

func (util *Utility) LoadUpdateDocumentState(ctx context.T, messageId string) error {
	instanceID, err := ctx.Identity().InstanceID()
	if err != nil {
		return fmt.Errorf("count not fetch instance id %v", err)
	}
	commandId, err := model.GetCommandID(messageId)
	if err != nil || commandId == "" {
		return fmt.Errorf("could not parse command id %v: %v", commandId, err)
	}

	updateCommandDocStatePath := path.Join(appconfig.DefaultDataStorePath,
		instanceID,
		appconfig.DefaultDocumentRootDirName,
		appconfig.DefaultLocationOfState,
		appconfig.DefaultLocationOfCurrent,
		commandId)

	if !fileExists(updateCommandDocStatePath) {
		return fmt.Errorf("update command document state file path not found: %v", updateCommandDocStatePath)
	}

	if err = unmarshallFile(updateCommandDocStatePath, &util.UpdateDocState); err != nil {
		return fmt.Errorf("failed to unmarshall update document state: %v", err)
	}
	ctx.Log().Debugf("document successfully loaded %v", util.UpdateDocState)
	return nil
}

func pushUpdateDocToAgentCurrentQueue(commandId, instanceID string, updateState contracts.DocumentState, expBackOff *backoff.ExponentialBackOff) error {
	defer expBackOff.Reset()
	absoluteFileName := path.Join(appconfig.DefaultDataStorePath,
		instanceID,
		appconfig.DefaultDocumentRootDirName,
		appconfig.DefaultLocationOfState,
		appconfig.DefaultLocationOfCurrent,
		commandId)

	content, err := jsonutil.Marshal(updateState) // Uses the loaded
	if err != nil {
		return fmt.Errorf("encountered error with message %v while marshalling %v to string", err, updateState)
	}
	return backOffRetry(func() error {
		success, err := fileWrite(absoluteFileName, jsonutil.Indent(content), os.FileMode(int(appconfig.ReadWriteAccess)))
		if !success || err != nil {
			return fmt.Errorf("could not persisted interim state in %v: %v", absoluteFileName, err)
		}
		return nil
	}, expBackOff)

	return nil
}

func getPluginState(updateRoot string, expBackOff *backoff.ExponentialBackOff) (UpdatePluginRunState, error) {
	defer expBackOff.Reset()
	var pluginState UpdatePluginRunState
	absoluteFileName := path.Join(updateRoot, stateJson)
	if !fileExists(absoluteFileName) {
		return pluginState, nil
	}
	callErr := backOffRetry(func() error {
		if err := unmarshallFile(absoluteFileName, &pluginState); err != nil {
			return err
		}
		return nil
	}, expBackOff)
	return pluginState, callErr
}

func savePluginState(updateRoot string, tempState UpdatePluginRunState, expBackOff *backoff.ExponentialBackOff) error {
	defer expBackOff.Reset()
	absoluteFileName := path.Join(updateRoot, stateJson)
	content, err := jsonutil.Marshal(tempState)
	if err != nil {
		return fmt.Errorf("encountered error with message %v while marshalling %v to string", err, tempState)
	}

	return backOffRetry(func() error {
		s, err := fileWrite(absoluteFileName, jsonutil.Indent(content), os.FileMode(int(appconfig.ReadWriteAccess)))
		if s && err == nil {
			return nil
		}
		return fmt.Errorf("could not save file %v: %v", absoluteFileName, err)
	}, expBackOff)
}

func removePluginState(log log.T, updateRoot string, expBackOff *backoff.ExponentialBackOff) {
	defer expBackOff.Reset()
	absoluteFileName := path.Join(updateRoot, stateJson)
	if fileExists(absoluteFileName) {
		err := backOffRetry(func() error {
			return deleteFile(absoluteFileName)
		}, expBackOff)
		if err != nil {
			log.Warnf("Could not remove plugin state file %v", err)
			return
		}
		log.Infof("update plugin state file removed successfully")
	}
}

func removeUpdateDocFromAgentCurrentQueue(log log.T, commandId, instanceID string, expBackOff *backoff.ExponentialBackOff) {
	defer expBackOff.Reset()
	absoluteFileName := path.Join(appconfig.DefaultDataStorePath,
		instanceID,
		appconfig.DefaultDocumentRootDirName,
		appconfig.DefaultLocationOfState,
		appconfig.DefaultLocationOfCurrent,
		commandId)
	if fileExists(absoluteFileName) {
		err := backOffRetry(func() error {
			return deleteFile(absoluteFileName)
		}, expBackOff)
		if err != nil {
			log.Warnf("Could not remove plugin state file %v", err)
			return
		}
		log.Infof("update doc state file removed successfully")
	}
}

func isAgentServiceRunning(log log.T) (bool, error) {
	serviceName := "AmazonSSMAgent"
	expectedState := svc.Running

	manager, err := mgr.Connect()
	if err != nil {
		log.Warnf("Cannot connect to service manager: %v", err)
		return false, err
	}
	defer manager.Disconnect()

	service, err := manager.OpenService(serviceName)
	if err != nil {
		log.Warnf("Cannot open agent service: %v", err)
		return false, err
	}
	defer service.Close()

	serviceStatus, err := service.Query()
	if err != nil {
		log.Warnf("Cannot query agent service: %v", err)
		return false, err
	}

	return serviceStatus.State == expectedState, err
}

func setPlatformSpecificCommand(parts []string) []string {
	cmd := appconfig.PowerShellPluginCommandName + " -ExecutionPolicy unrestricted"
	return append(strings.Split(cmd, " "), parts...)
}

// ResolveUpdateRoot returns the platform specific path to update artifacts
func ResolveUpdateRoot(sourceVersion string) (string, error) {
	return appconfig.UpdaterArtifactsRoot, nil
}

func verifyVersion(log log.T, targetVersion string) updateconstants.ErrorCode {
	log.Infof("Verifying Agent version using Registry")
	registryCurrentAgentVersion := getVersionThroughRegistryKey(log)
	if targetVersion == registryCurrentAgentVersion {
		log.Infof("Verifying Agent version using WMI query")
		wmiCurrentAgentVersion := getVersionThroughWMI(log)
		if targetVersion == wmiCurrentAgentVersion {
			return "" // return blank when success
		}
		return updateconstants.ErrorInstTargetVersionNotFoundViaWMIC
	}
	return updateconstants.ErrorInstTargetVersionNotFoundViaReg
}

func getVersionThroughRegistryKey(log log.T) string {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Services\AmazonSSMAgent`, registry.QUERY_VALUE)
	if err != nil {
		log.Warnf("Error opening registry key: %v", err)
		return ""
	}
	defer key.Close()
	version, _, err := key.GetStringValue("Version")
	if err != nil {
		log.Warnf("Error getting Agent version value: %v", err)
	}
	return strings.TrimSpace(version)
}

func getVersionThroughWMI(log log.T) string {
	version := ""
	contentBytes, err := execCommand(wmicCommand, "product", "where", "name like 'Amazon SSM Agent%'", "get", "version", "/Value").Output()
	if err != nil {
		log.Warnf("Error getting version value from WMIC: %v %v", string(contentBytes), err)
		return version
	}
	contents := string(contentBytes)
	log.Debugf("Version info from WMIC: %v", contents)
	data := strings.Split(contents, "=")
	if len(data) > 1 {
		version = strings.TrimSpace(data[1])
	}
	return strings.TrimSpace(version)
}
