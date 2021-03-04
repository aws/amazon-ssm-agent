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
	"regexp"
	"runtime/debug"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/session/utility"
)

const (
	maxOrchestrationDirectoryDeletions int = 100
)

type validString func(string) bool
type modifyString func(string) string

type DocumentMgr interface {
	MoveDocumentState(fileName, srcLocationFolder, dstLocationFolder string)
	PersistDocumentState(fileName, locationFolder string, state contracts.DocumentState)
	GetDocumentState(fileName, locationFolder string) contracts.DocumentState
	RemoveDocumentState(fileName, locationFolder string)
}

//TODO use class lock instead of global lock?
//TODO decouple the DocState model to better fit the service-processor-executer architecture
//DocumentFileMgr encapsulate the file access and perform bookkeeping operations at the specified file location
type DocumentFileMgr struct {
	context       context.T
	dataStorePath string
	rootDirName   string
	stateLocation string
}

func NewDocumentFileMgr(context context.T, dataStorePath, rootDirName, stateLocation string) *DocumentFileMgr {
	return &DocumentFileMgr{
		context:       context,
		dataStorePath: dataStorePath,
		rootDirName:   rootDirName,
		stateLocation: stateLocation,
	}
}

func (d *DocumentFileMgr) MoveDocumentState(fileName, srcLocationFolder, dstLocationFolder string) {
	log := d.context.Log()
	instanceID, err := d.context.Identity().ShortInstanceID()
	if err != nil {
		log.Errorf("Failed to get short instanceID for MoveDocumentState: %v", err)
	}

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

}

func (d *DocumentFileMgr) PersistDocumentState(fileName, locationFolder string, state contracts.DocumentState) {
	log := d.context.Log()
	instanceID, err := d.context.Identity().ShortInstanceID()
	if err != nil {
		log.Errorf("Failed to get short instanceID for PersistDocumentState: %v", err)
	}

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

func (d *DocumentFileMgr) GetDocumentState(fileName, locationFolder string) contracts.DocumentState {
	log := d.context.Log()
	instanceID, err := d.context.Identity().ShortInstanceID()
	if err != nil {
		log.Errorf("Failed to get short instanceID for GetDocumentState: %v", err)
	}

	filepath := path.Join(d.dataStorePath,
		instanceID,
		d.rootDirName,
		d.stateLocation,
		locationFolder)

	absoluteFileName := path.Join(filepath, fileName)

	var commandState contracts.DocumentState
	var count, retryLimit int = 0, 3

	// retry to avoid sync problem, which arises when OfflineService and MessageDeliveryService try to access the file at the same time
	for count < retryLimit {
		err := jsonutil.UnmarshalFile(absoluteFileName, &commandState)
		if err != nil {
			log.Errorf("encountered error with message %v while reading Interim state of command from file - %v", err, fileName)
			count += 1
			time.Sleep(500 * time.Millisecond)
			continue
		} else {
			//logging interim state as read from the file
			jsonString, err := jsonutil.Marshal(commandState)
			if err != nil {
				log.Errorf("encountered error with message %v while marshalling %v to string", err, commandState)
			} else {
				log.Tracef("interim CommandState read from file-system - %v", jsonutil.Indent(jsonString))
			}
			break
		}
	}

	if count >= retryLimit {
		if fileExists, _ := fileutil.LocalFileExist(absoluteFileName); fileExists {
			if documentContents, err := fileutil.ReadAllText(absoluteFileName); err == nil {
				log.Infof("Document contents: %v", documentContents)
			}

			d.MoveDocumentState(fileName, locationFolder, appconfig.DefaultLocationOfCorrupt)
		}

	}

	return commandState
}

// RemoveData deletes the fileName from locationFolder under defaultLogDir/instanceID
func (d *DocumentFileMgr) RemoveDocumentState(commandID, locationFolder string) {
	log := d.context.Log()
	instanceID, err := d.context.Identity().ShortInstanceID()
	if err != nil {
		log.Errorf("Failed to get short instanceID for GetDocumentState: %v", err)
	}

	absoluteFileName := docStateFileName(commandID, instanceID, locationFolder)

	err = fileutil.DeleteFile(absoluteFileName)
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
func orchestrationDir(instanceID, orchestrationRootDirName string, folderType string) string {
	switch folderType {
	case appconfig.DefaultSessionRootDirName:
		return path.Join(appconfig.DefaultDataStorePath,
			instanceID,
			appconfig.DefaultSessionRootDirName,
			orchestrationRootDirName)
	default:
		return path.Join(appconfig.DefaultDataStorePath,
			instanceID,
			appconfig.DefaultDocumentRootDirName,
			orchestrationRootDirName)

	}
}

// getOrchestrationDirectoryNames returns list of orchestration directories.
func getOrchestrationDirectoryNames(log log.T, instanceID, orchestrationRootDirName string, folderType string) (orchestrationRootDir string, dirNames []string, err error) {
	// Form the path for orchestration logs dir
	orchestrationRootDir = orchestrationDir(instanceID, orchestrationRootDirName, folderType)

	if !fileutil.Exists(orchestrationRootDir) {
		log.Debugf("Orchestration root directory doesn't exist: %v", orchestrationRootDir)
		return orchestrationRootDir, []string{}, nil
	}

	dirNames, err = fileutil.GetDirectoryNames(orchestrationRootDir)
	return orchestrationRootDir, dirNames, err
}

// isRunCommandDirName checks whether the file name format satisfies the format for RunCommand generated log files
func isRunCommandDirName(dirName string) (matched bool) {
	matched, _ = regexp.MatchString("^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$", dirName)
	return
}

// isAssociationLogFile checks whether the file name passed is of the format of Association Files
func isAssociationRunDirName(dirName string) (matched bool) {
	matched, _ = regexp.MatchString("^[0-9]{4}-[0-9]{2}-[0-9]{2}.*$", dirName)
	return
}

// cleanupAssociationDirectory cleans up association directory by deleting expired association run directories from it.
func cleanupAssociationDirectory(log log.T, deletedCount int, commandOrchestrationPath string, retentionDurationHours int) (canDeleteDirectory bool, deletedCountAfter int) {
	subdirNames, err := fileutil.GetDirectoryNames(commandOrchestrationPath)
	if err != nil {
		log.Debugf("Error reading association orchestration directory %v: %v", commandOrchestrationPath, err)
		return false, deletedCount
	}

	canDeleteDirectory = true

	for _, subdirName := range subdirNames {
		if deletedCount >= maxOrchestrationDirectoryDeletions {
			log.Infof("Reached max number of deletions for orchestration directories: %v", deletedCount)
			canDeleteDirectory = false
			break
		}

		if !isAssociationRunDirName(subdirName) {
			continue
		}

		subdirpath := filepath.Join(commandOrchestrationPath, subdirName)
		log.Debugf("Checking association-run orchestration directory: %v", subdirpath)
		if expiredDir := isOlderThan(log, subdirpath, retentionDurationHours); !expiredDir {
			canDeleteDirectory = false
			continue
		}

		log.Debugf("Attempting deletion of association-run orchestration directory %v", subdirpath)
		if err := fileutil.DeleteDirectory(subdirpath); err != nil {
			log.Debugf("Error deleting directory %v: %v", subdirpath, err)
			canDeleteDirectory = false
			continue
		}

		deletedCount += 1
	}

	return canDeleteDirectory, deletedCount
}

// isLegacyAssociationDirectory checks whether orchestration directory is a legacy association directory.
func isLegacyAssociationDirectory(log log.T, commandOrchestrationPath string) (bool, error) {
	subdirNames, err := fileutil.GetDirectoryNames(commandOrchestrationPath)
	if err != nil {
		log.Debugf("Error reading orchestration directory %v: %v", commandOrchestrationPath, err)
		return false, err
	}

	// If run sub-directory exists, then it is legacy association orchestration directory
	for _, subdirName := range subdirNames {
		if isAssociationRunDirName(subdirName) {
			return true, nil
		}
	}
	return false, nil
}

// DeleteOldOrchestrationDirectories deletes expired orchestration directories based on retentionDurationHours and associationRetentionDurationHours.
func DeleteOldOrchestrationDirectories(log log.T, instanceID, orchestrationRootDirName string, retentionDurationHours int, associationRetentionDurationHours int) {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Delete orchestration directories panic: %v", r)
			log.Errorf("Stacktrace:\n%s", debug.Stack())
		}
	}()
	orchestrationRootDir, dirNames, err := getOrchestrationDirectoryNames(log, instanceID, orchestrationRootDirName, appconfig.DefaultDocumentRootDirName)
	if err != nil {
		log.Debugf("Failed to get orchestration directories under %v", err)
		return
	}

	log.Debugf("Cleaning up orchestration directories: %v", orchestrationRootDir)

	deletedCount := 0
	for _, dirName := range dirNames {
		if deletedCount >= maxOrchestrationDirectoryDeletions {
			log.Infof("Reached max number of deletions for orchestration directories: %v", deletedCount)
			break
		}

		commandOrchestrationPath := filepath.Join(orchestrationRootDir, dirName)

		if isAssoc, err := isLegacyAssociationDirectory(log, commandOrchestrationPath); isAssoc && err == nil {
			var canDeleteDirectory bool
			canDeleteDirectory, deletedCount = cleanupAssociationDirectory(log, deletedCount, commandOrchestrationPath, associationRetentionDurationHours)
			if !canDeleteDirectory {
				continue
			}
		}

		log.Debugf("Checking command orchestration directory: %v", commandOrchestrationPath)
		if isOlderThan(log, commandOrchestrationPath, retentionDurationHours) {
			log.Debugf("Attempting deletion of command orchestration directory: %v", commandOrchestrationPath)

			err := fileutil.DeleteDirectory(commandOrchestrationPath)
			if err != nil {
				log.Debugf("Error deleting directory %v: %v", commandOrchestrationPath, err)
				continue
			}

			// Deletion of both document state and orchestration file was successful
			deletedCount += 1
		}

	}

	log.Debugf("Completed orchestration directory clean up")
}

// DeleteSessionOrchestrationDirectories deletes expired orchestration directories based on session retentionDurationHours.
func DeleteSessionOrchestrationDirectories(log log.T, instanceID, orchestrationRootDirName string, retentionDurationHours int) {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Delete session orchestration directories panic: %v", r)
			log.Errorf("Stacktrace:\n%s", debug.Stack())
		}
	}()
	orchestrationRootDir, dirNames, err := getOrchestrationDirectoryNames(log, instanceID, orchestrationRootDirName, appconfig.DefaultSessionRootDirName)
	if err != nil {
		log.Debugf("Failed to get orchestration directories under %v", err)
		return
	}

	log.Debugf("Cleaning up orchestration directories: %v", orchestrationRootDir)

	deletedCount := 0
	for _, dirName := range dirNames {
		if deletedCount >= maxOrchestrationDirectoryDeletions {
			log.Infof("Reached max number of deletions for orchestration directories: %v", deletedCount)
			break
		}

		sessionOrchestrationPath := filepath.Join(orchestrationRootDir, dirName)

		log.Debugf("Checking session orchestration directory: %v", sessionOrchestrationPath)
		if isOlderThan(log, sessionOrchestrationPath, retentionDurationHours) {
			log.Debugf("Attempting deletion of session orchestration directory: %v", sessionOrchestrationPath)

			err := fileutil.DeleteDirectory(sessionOrchestrationPath)
			if err != nil {
				log.Debugf("Error deleting directory %v: %v", sessionOrchestrationPath, err)

				// With CloudWatch streaming of logs, a change was introduced to make ipcTempFile append only on linux.
				// This append only mode results into error while deletion of the file.
				// Below logic is to attempt to delete ipcTempFile in case of such errors.
				u := &utility.SessionUtil{}
				success, err := u.DeleteIpcTempFile(sessionOrchestrationPath)
				if err != nil || !success {
					log.Debugf("Retry attempt to delete session orchestration directory %s failed, %v", sessionOrchestrationPath, err)
					continue
				}
			}

			// Deletion of both document state and orchestration file was successful
			deletedCount += 1
		}

	}

	log.Debugf("Completed orchestration directory clean up")
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

// docStateFileName returns absolute filename where command states are persisted
func docStateFileName(fileName, instanceID, locationFolder string) string {
	return path.Join(DocumentStateDir(instanceID, locationFolder), fileName)
}
