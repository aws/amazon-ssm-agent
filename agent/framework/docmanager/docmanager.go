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

// Package docmanager helps persist documents state to disk
package docmanager

import (
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
)

const (
	maxLogFileDeletions int = 100
)

type validString func(string) bool
type modifyString func(string) string

//TODO:  Revisit this when making Persistence invasive - i.e failure in file-systems should resort to Agent crash instead of swallowing errors

var lock sync.RWMutex
var docLock = make(map[string]*sync.RWMutex)

type DocumentMgr interface {
	MoveDocumentState(log log.T, fileName, instanceID, srcLocationFolder, dstLocationFolder string)
	PersistDocumentState(log log.T, fileName, instanceID, locationFolder string, state contracts.DocumentState)
	GetDocumentState(log log.T, fileName, instanceID, locationFolder string) contracts.DocumentState
	RemoveDocumentState(log log.T, fileName, instanceID, locationFolder string)
}

//TODO use class lock instead of global lock?
//TODO decouple the DocState model to better fit the service-processor-executer architecture
//DocumentFileMgr encapsulate the file access and perform bookkeeping operations at the specified file location
type DocumentFileMgr struct {
	dataStorePath string
	rootDirName   string
	stateLocation string
}

func NewDocumentFileMgr(dataStorePath, rootDirName, stateLocation string) *DocumentFileMgr {
	return &DocumentFileMgr{
		dataStorePath: dataStorePath,
		rootDirName:   rootDirName,
		stateLocation: stateLocation,
	}
}

func (d *DocumentFileMgr) MoveDocumentState(log log.T, fileName, instanceID, srcLocationFolder, dstLocationFolder string) {
	//get a lock for documentID specific lock
	lockDocument(fileName)

	absoluteSource := path.Join(d.dataStorePath,
		instanceID,
		d.rootDirName,
		d.stateLocation,
		srcLocationFolder)

	absoluteDestination := path.Join(d.dataStorePath,
		instanceID,
		d.rootDirName,
		d.stateLocation,
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

func (d *DocumentFileMgr) PersistDocumentState(log log.T, fileName, instanceID, locationFolder string, state contracts.DocumentState) {
	lockDocument(fileName)
	defer unlockDocument(fileName)

	absoluteFileName := path.Join(path.Join(d.dataStorePath,
		instanceID,
		d.rootDirName,
		d.stateLocation,
		locationFolder), fileName)

	content, err := jsonutil.Marshal(state)
	if err != nil {
		log.Errorf("encountered error with message %v while marshalling %v to string", err, state)
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

func (d *DocumentFileMgr) GetDocumentState(log log.T, fileName, instanceID, locationFolder string) contracts.DocumentState {

	rLockDocument(fileName)
	defer rUnlockDocument(fileName)

	absoluteFileName := path.Join(path.Join(d.dataStorePath,
		instanceID,
		d.rootDirName,
		d.stateLocation,
		locationFolder), fileName)

	var commandState contracts.DocumentState
	err := jsonutil.UnmarshalFile(absoluteFileName, &commandState)
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

// RemoveData deletes the fileName from locationFolder under defaultLogDir/instanceID
func (d *DocumentFileMgr) RemoveDocumentState(log log.T, commandID, instanceID, locationFolder string) {

	absoluteFileName := docStateFileName(commandID, instanceID, locationFolder)

	err := fileutil.DeleteFile(absoluteFileName)
	if err != nil {
		log.Errorf("encountered error %v while deleting file %v", err, absoluteFileName)
	} else {
		log.Debugf("successfully deleted file %v", absoluteFileName)
	}
}

//TODO rework this part
// DocumentStateDir returns absolute filename where command states are persisted
func DocumentStateDir(instanceID, locationFolder string) string {
	return filepath.Join(appconfig.DefaultDataStorePath,
		instanceID,
		appconfig.DefaultDocumentRootDirName,
		appconfig.DefaultLocationOfState,
		locationFolder)
}

// orchestrationDir returns the absolute path of the orchestration directory
func orchestrationDir(instanceID, orchestrationRootDirName string) string {
	return path.Join(appconfig.DefaultDataStorePath,
		instanceID,
		appconfig.DefaultDocumentRootDirName,
		orchestrationRootDirName)
}

// DeleteOldOrchestrationFolderLogs deletes the logs from document/state/completed and document/orchestration folders older than retention duration which satisfy the file name format
func DeleteOldOrchestrationFolderLogs(log log.T, instanceID, orchestrationRootDirName string, retentionDurationHours int, isIntendedFileNameFormat validString) {
	defer func() {
		// recover in case the function panics
		if msg := recover(); msg != nil {
			log.Errorf("DeleteOldOrchestrationFolderLogs failed with message %v", msg)
		}
	}()

	// Form the path for orchestration logs dir
	orchestrationRootDir := orchestrationDir(instanceID, orchestrationRootDirName)

	if !fileutil.Exists(orchestrationRootDir) {
		log.Debugf("Completed log directory doesn't exist: %v", orchestrationRootDir)
		return
	}

	outputFiles, err := fileutil.GetFileNames(orchestrationRootDir)
	if err != nil {
		log.Debugf("Failed to read files under %v", err)
		return
	}

	if outputFiles == nil || len(outputFiles) == 0 {
		log.Debugf("Completed log directory %v is invalid or empty", orchestrationRootDir)
		return
	}

	// Go through all log files in the completed logs dir, delete max maxLogFileDeletions files and the corresponding dirs from orchestration folder
	countOfDeletions := 0
	for _, completedFile := range outputFiles {

		outputLogFullPath := filepath.Join(orchestrationRootDir, completedFile)

		//Checking for the file name format so that the function only deletes the files it is called to do. Also checking whether the file is beyond retention time.
		if isIntendedFileNameFormat(completedFile) && isOlderThan(log, outputLogFullPath, retentionDurationHours) {
			log.Debugf("Attempting Deletion of folder : %v", outputLogFullPath)

			err := fileutil.DeleteDirectory(outputLogFullPath)
			if err != nil {
				log.Debugf("Error deleting dir %v: %v", outputLogFullPath, err)
				continue
			}

			// Deletion of both document state and orchestration file was successful
			countOfDeletions += 1
			if countOfDeletions > maxLogFileDeletions {
				break
			}
		}

	}

	log.Debugf("Completed DeleteOldOrchestrationFolderLogs")
}

// isOlderThan checks whether the file is older than the retention duration
func isOlderThan(log log.T, fileFullPath string, retentionDurationHours int) bool {
	modificationTime, err := fileutil.GetFileModificationTime(fileFullPath)

	if err != nil {
		log.Debugf("Failed to get modification time %v", err)
		return false
	}

	// Check whether the current time is after modification time plus the retention duration
	return modificationTime.Add(time.Hour * time.Duration(retentionDurationHours)).Before(time.Now())
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
