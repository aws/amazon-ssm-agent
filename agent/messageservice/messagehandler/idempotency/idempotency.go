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

// Package idempotency implements methods to maintain idempotency with the commands received
package idempotency

import (
	"os"
	"path/filepath"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/runcommand/contracts"
)

const (

	// number of documents to delete at a time
	deletionLimit = 200

	// Name represents idempotency name
	Name = "Idempotency"
)

var (
	// persistence timeout
	persistenceTimeoutMinutes = 30

	getDirectoryUnsortedOlderThan = fileutil.GetDirectoryNamesUnsortedOlderThan
	deleteDirectory               = fileutil.DeleteDirectory
	makeDirs                      = fileutil.MakeDirs
	stat                          = os.Stat
	isNotExist                    = os.IsNotExist
	getIdempotencyDir             = getIdempotencyDirectory
)

// CleanupOldIdempotencyEntries deletes the commands in idempotency folder after persistenceTimeout minutes
func CleanupOldIdempotencyEntries(idemCtx context.T) {
	context := idemCtx.With("[" + Name + "]")
	log := context.Log()
	directoryPath := getIdempotencyDir(context)
	documentTypeDir, err := getDirectoryUnsortedOlderThan(directoryPath, nil)
	if err != nil {
		log.Warnf("encountered error %v while listing directories in %v", err, directoryPath)
	}
	for _, docTypeDir := range documentTypeDir {
		olderTime := time.Now().Add(time.Duration(-persistenceTimeoutMinutes) * time.Minute)
		tempDocTypeDir := filepath.Join(directoryPath, docTypeDir)
		commandDirs, err := getDirectoryUnsortedOlderThan(tempDocTypeDir, &olderTime)
		if err != nil {
			log.Warnf("encountered error %v while listing entries in %v", err, directoryPath)
		}
		decrementCount := deletionLimit
		for _, commandDir := range commandDirs {
			decrementCount--
			tempCommandDir := filepath.Join(tempDocTypeDir, commandDir)
			deleteErr := deleteDirectory(tempCommandDir)
			if deleteErr != nil {
				log.Warnf("encountered error %v while deleting entry %v", deleteErr, tempCommandDir)
			} else {
				log.Debugf("successfully deleted entry %v", tempCommandDir)
			}
			if decrementCount == 0 {
				return // break from the deletion thread
			}
		}
	}
}

// CreateIdempotencyEntry writes command id to the idempotency directory
func CreateIdempotencyEntry(idemCtx context.T, message *contracts.DocumentState) error {
	var err error
	context := idemCtx.With("[" + Name + "]")
	log := context.Log()
	directoryPath := getIdempotencyDir(context)
	commandID, _ := messageContracts.GetCommandID(message.DocumentInformation.MessageID)
	commandDirPath := filepath.Join(directoryPath, string(message.DocumentType), commandID)
	log.Infof("writing command in the idempotency directory for command %v", commandID)
	if err = makeDirs(commandDirPath); err != nil {
		log.Warnf("could not create command directory in %v for the command %v: err: %v", commandDirPath, commandID, err)
		return err
	}
	return nil
}

// IsDocumentAlreadyReceived checks whether the document was received already
func IsDocumentAlreadyReceived(idemCtx context.T, message *contracts.DocumentState) bool {
	context := idemCtx.With("[" + Name + "]")
	log := context.Log()
	directoryPath := getIdempotencyDir(context)
	commandID, _ := messageContracts.GetCommandID(message.DocumentInformation.MessageID)
	commandDirPath := filepath.Join(directoryPath, string(message.DocumentType), commandID)

	if _, err := stat(commandDirPath); isNotExist(err) {
		log.Debugf("command not found in the idempotency directory %v", commandID)
		return false
	}
	log.Infof("command found in the idempotency directory, skipping the document %v", commandID)
	return true
}

// getIdempotencyDirectory returns the absolute path of idempotency directory
func getIdempotencyDirectory(context context.T) string {
	shortInstanceID, _ := context.Identity().ShortInstanceID()
	context.Identity().ShortInstanceID()
	return filepath.Join(appconfig.DefaultDataStorePath,
		shortInstanceID,
		appconfig.IdempotencyDirName)
}
