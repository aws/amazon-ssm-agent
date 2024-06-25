// Copyright 2024 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package processor contains the methods for update ssm agent.
// It also provides methods for sendReply and updateInstanceInfo

//go:build windows
// +build windows

package processor

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
)

var (
	osEnviron    = os.Environ
	fileMakeDirs = fileutil.MakeDirs
)

func updateExecutionTimeoutIfNeeded(retryCount int, defaultTimeOut int, updateUtil updateutil.T) {
	// use the default one
	if retryCount > 1 {
		updateUtil.UpdateExecutionTimeOut(defaultTimeOut) // use default timeout
		return
	}
	updateUtil.UpdateExecutionTimeOut(defaultTimeOut * 4) // 10 mins
}

func getCommandEnv(log log.T, rootDir string) []string {
	updateFilePath := filepath.Join(rootDir, installationDirectory)
	fileErr := fileMakeDirs(updateFilePath)
	if fileErr != nil {
		log.Warnf("could not create update installation directory: %v", fileErr)
		return nil
	}
	env := osEnviron()
	// Modify the Temp environment variables
	newEnv := make([]string, 0, len(env))
	tmpEnvName := "TMP="
	tempEnvName := "TEMP="
	for _, e := range env {
		if strings.HasPrefix(e, tmpEnvName) || strings.HasPrefix(e, tempEnvName) {
			// These skipped Env variables will be updated below
			continue
		} else {
			newEnv = append(newEnv, e)
		}
	}
	// below if condition is used to fill in TMP env variable
	newEnv = append(newEnv, tmpEnvName+updateFilePath) // TMP
	// below if condition is used to fill in TEMP env variable
	newEnv = append(newEnv, tempEnvName+updateFilePath) // TEMP

	return newEnv
}

// moveCleanInstallationDir moves files from update installation directory to temp folder
// and alter delete the installation directory
func moveCleanInstallationDir(log log.T, updateDetail *UpdateDetail) {
	log.Info("Initiating move of files from update installation directory")
	updateInstallationDir := filepath.Join(updateDetail.UpdateRoot, installationDirectory)
	tempDir := os.TempDir()
	olderTime := time.Now().Add(time.Duration(-2) * time.Hour)
	artifactNames, dirErr := getFileNamesLaterThan(updateInstallationDir, &olderTime)
	if dirErr != nil {
		log.Warnf("error while getting the file names: %v", dirErr)
	}
	if dirErr == nil {
		for _, artifactName := range artifactNames {
			if s, err := moveFile(artifactName, updateInstallationDir, tempDir); s && err == nil {
				log.Debugf("moved file %v from %v to %v successfully", artifactName, updateInstallationDir, tempDir)
			}
		}
	}
	time.Sleep(200 * time.Millisecond) // Wait for short time(randomly chosen) for OS to finish moving files
	if err := deleteDirectory(updateInstallationDir); err != nil {
		log.Warnf("error while deleting the update installation folder: %v", err)
	}
}
