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

// Package statemanager helps persist documents state to disk
package statemanager

import (
	"os"
	"path"
	"sync"

	"path/filepath"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/statemanager/model"
)

//TODO:  Revisit this when making Persistence invasive - i.e failure in file-systems should resort to Agent crash instead of swallowing errors

var lock sync.RWMutex
var docLock = make(map[string]*sync.RWMutex)

// GetDocumentInterimState returns CommandState object after reading file <fileName> from locationFolder
// under defaultLogDir/instanceID
func GetDocumentInterimState(log log.T, fileName, instanceID, locationFolder string) model.DocumentState {

	rLockDocument(fileName)
	defer rUnlockDocument(fileName)

	absoluteFileName := docStateFileName(fileName, instanceID, locationFolder)

	docState := getDocState(log, absoluteFileName)

	return docState
}

// PersistData stores the given object in the file-system in pretty Json indented format
// This will override the contents of an already existing file
func PersistData(log log.T, fileName, instanceID, locationFolder string, object interface{}) {

	lockDocument(fileName)
	defer unlockDocument(fileName)

	absoluteFileName := docStateFileName(fileName, instanceID, locationFolder)

	content, err := jsonutil.Marshal(object)
	if err != nil {
		log.Errorf("encountered error with message %v while marshalling %v to string", err, object)
	} else {
		if fileutil.Exists(absoluteFileName) {
			log.Debugf("overwriting contents of %v", absoluteFileName)
		}
		log.Tracef("persisting interim state %v in file %v", jsonutil.Indent(content), absoluteFileName)
		if s, err := fileutil.WriteIntoFileWithPermissions(absoluteFileName, jsonutil.Indent(content), os.FileMode(int(appconfig.ReadWriteAccess))); s && err == nil {
			log.Debugf("successfully persisted interim state in %v", locationFolder)
		} else {
			log.Debugf("persisting interim state in %v failed with error %v", locationFolder, err)
		}
	}
}

// IsDocumentCurrentlyExecuting checks if document already present in Pending or Current folder
func IsDocumentCurrentlyExecuting(fileName, instanceID string) bool {

	if len(fileName) == 0 {
		return false
	}

	lockDocument(fileName)
	defer unlockDocument(fileName)

	absoluteFileName := docStateFileName(fileName, instanceID, appconfig.DefaultLocationOfPending)
	if fileutil.Exists(absoluteFileName) {
		return true
	}
	absoluteFileName = docStateFileName(fileName, instanceID, appconfig.DefaultLocationOfCurrent)
	return fileutil.Exists(absoluteFileName)
}

// RemoveData deletes the fileName from locationFolder under defaultLogDir/instanceID
func RemoveData(log log.T, commandID, instanceID, locationFolder string) {

	absoluteFileName := docStateFileName(commandID, instanceID, locationFolder)

	err := fileutil.DeleteFile(absoluteFileName)
	if err != nil {
		log.Errorf("encountered error %v while deleting file %v", err, absoluteFileName)
	} else {
		log.Debugf("successfully deleted file %v", absoluteFileName)
	}
}

// MoveDocumentState moves the document file to target location
func MoveDocumentState(log log.T, fileName, instanceID, srcLocationFolder, dstLocationFolder string) {

	//get a lock for documentID specific lock
	lockDocument(fileName)

	absoluteSource := path.Join(appconfig.DefaultDataStorePath,
		instanceID,
		appconfig.DefaultDocumentRootDirName,
		appconfig.DefaultLocationOfState,
		srcLocationFolder)

	absoluteDestination := path.Join(appconfig.DefaultDataStorePath,
		instanceID,
		appconfig.DefaultDocumentRootDirName,
		appconfig.DefaultLocationOfState,
		dstLocationFolder)

	if s, err := fileutil.MoveFile(fileName, absoluteSource, absoluteDestination); s && err == nil {
		log.Debugf("moved file %v from %v to %v successfully", fileName, srcLocationFolder, dstLocationFolder)
	} else {
		log.Debugf("moving file %v from %v to %v failed with error %v", fileName, srcLocationFolder, dstLocationFolder, err)
	}

	//release documentID specific lock - before deleting the entry from the map
	unlockDocument(fileName)

	//delete documentID specific lock if document has finished executing. This is to avoid documentLock growing too much in memory.
	//This is done by ensuring that as soon as document finishes executing it is removed from documentLock
	//Its safe to assume that document has finished executing if it is being moved to appconfig.DefaultLocationOfCompleted
	if dstLocationFolder == appconfig.DefaultLocationOfCompleted {
		deleteLock(fileName)
	}
}

// GetDocumentInfo returns the document info for the specified fileName
func GetDocumentInfo(log log.T, fileName, instanceID, locationFolder string) model.DocumentInfo {
	rLockDocument(fileName)
	defer rUnlockDocument(fileName)

	absoluteFileName := docStateFileName(fileName, instanceID, locationFolder)

	commandState := getDocState(log, absoluteFileName)

	return commandState.DocumentInformation
}

// PersistDocumentInfo stores the given PluginState in file-system in pretty Json indented format
// This will override the contents of an already existing file
func PersistDocumentInfo(log log.T, docInfo model.DocumentInfo, fileName, instanceID, locationFolder string) {

	absoluteFileName := docStateFileName(fileName, instanceID, locationFolder)

	//get documentID specific write lock
	lockDocument(fileName)
	defer unlockDocument(fileName)

	//Plugins should safely assume that there already
	//exists a persisted interim state file - if not then it should throw error

	//read command state from file-system first
	commandState := getDocState(log, absoluteFileName)

	commandState.DocumentInformation = docInfo

	setDocState(log, commandState, absoluteFileName, locationFolder)
}

// GetPluginState returns PluginState after reading fileName from given locationFolder under defaultLogDir/instanceID
func GetPluginState(log log.T, pluginID, commandID, instanceID, locationFolder string) *model.PluginState {

	rLockDocument(commandID)
	defer rUnlockDocument(commandID)

	absoluteFileName := docStateFileName(commandID, instanceID, locationFolder)

	commandState := getDocState(log, absoluteFileName)

	for _, pluginState := range commandState.InstancePluginsInformation {
		if pluginState.Id == pluginID {
			return &pluginState
		}
	}

	return nil
}

// PersistPluginState stores the given PluginState in file-system in pretty Json indented format
// This will override the contents of an already existing file
func PersistPluginState(log log.T, pluginState model.PluginState, pluginID, commandID, instanceID, locationFolder string) {

	lockDocument(commandID)
	defer unlockDocument(commandID)

	absoluteFileName := docStateFileName(commandID, instanceID, locationFolder)

	//Plugins should safely assume that there already
	//exists a persisted interim state file - if not then it should throw error
	commandState := getDocState(log, absoluteFileName)

	//TODO:  after adding unit-tests for persist data - this can be removed
	if commandState.InstancePluginsInformation == nil {
		pluginsInfo := []model.PluginState{}
		pluginsInfo = append(pluginsInfo, pluginState)
		commandState.InstancePluginsInformation = pluginsInfo
	} else {
		for index, plugin := range commandState.InstancePluginsInformation {
			if plugin.Id == pluginID {
				commandState.InstancePluginsInformation[index] = pluginState
				break
			}
		}
	}

	setDocState(log, commandState, absoluteFileName, locationFolder)
}

// DocumentStateDir returns absolute filename where command states are persisted
func DocumentStateDir(instanceID, locationFolder string) string {
	return filepath.Join(appconfig.DefaultDataStorePath,
		instanceID,
		appconfig.DefaultDocumentRootDirName,
		appconfig.DefaultLocationOfState,
		locationFolder)
}

// getDocState reads commandState from given file
func getDocState(log log.T, fileName string) model.DocumentState {

	var commandState model.DocumentState
	err := jsonutil.UnmarshalFile(fileName, &commandState)
	if err != nil {
		log.Errorf("encountered error with message %v while reading Interim state of command from file - %v", err, fileName)
	} else {
		//logging interim state as read from the file
		jsonString, err := jsonutil.Marshal(commandState)
		if err != nil {
			log.Errorf("encountered error with message %v while marshalling %v to string", err, commandState)
		} else {
			log.Tracef("interim CommandState read from file-system - %v", jsonutil.Indent(jsonString))
		}
	}

	return commandState
}

// setDocState persists given commandState
func setDocState(log log.T, commandState model.DocumentState, absoluteFileName, locationFolder string) {

	content, err := jsonutil.Marshal(commandState)
	if err != nil {
		log.Errorf("encountered error with message %v while marshalling %v to string", err, commandState)
	} else {
		if fileutil.Exists(absoluteFileName) {
			log.Debugf("overwriting contents of %v", absoluteFileName)
		}
		log.Tracef("persisting interim state %v in file %v", jsonutil.Indent(content), absoluteFileName)
		if s, err := fileutil.WriteIntoFileWithPermissions(absoluteFileName, jsonutil.Indent(content), os.FileMode(int(appconfig.ReadWriteAccess))); s && err == nil {
			log.Debugf("successfully persisted interim state in %v", locationFolder)
		} else {
			log.Debugf("persisting interim state in %v failed with error %v", locationFolder, err)
		}
	}
}

// rLockDocument locks id specific RWMutex for reading
func rLockDocument(id string) {
	//check if document lock even exists
	if !doesLockExist(id) {
		createLock(id)
	}

	docLock[id].RLock()
}

// rUnlockDocument releases id specific single RLock
func rUnlockDocument(id string) {
	docLock[id].RUnlock()
}

// lockDocument locks id specific RWMutex for writing
func lockDocument(id string) {
	//check if document lock even exists
	if !doesLockExist(id) {
		createLock(id)
	}

	docLock[id].Lock()
}

// unlockDocument releases id specific Lock for writing
func unlockDocument(id string) {
	docLock[id].Unlock()
}

// doesLockExist returns true if there exists documentLock for given id
func doesLockExist(id string) bool {
	lock.RLock()
	defer lock.RUnlock()
	_, ok := docLock[id]
	return ok
}

// createLock creates id specific lock (RWMutex)
func createLock(id string) {
	lock.Lock()
	defer lock.Unlock()
	docLock[id] = &sync.RWMutex{}
}

// deleteLock deletes id specific lock
func deleteLock(id string) {
	lock.Lock()
	defer lock.Unlock()
	delete(docLock, id)
}

// docStateFileName returns absolute filename where command states are persisted
func docStateFileName(fileName, instanceID, locationFolder string) string {
	return path.Join(DocumentStateDir(instanceID, locationFolder), fileName)
}
