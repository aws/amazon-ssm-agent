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

// package sharedCredentials provides access to the aws shared credentials file.
package sharedCredentials

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/aws/amazon-ssm-agent/agent/log"

	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/go-ini/ini"
)

const (
	defaultProfile     = "default"
	awsAccessKeyID     = "aws_access_key_id"
	awsSecretAccessKey = "aws_secret_access_key"
	awsSessionToken    = "aws_session_token"
)

// filename returns the filename to use to read AWS shared credentials.
//
// Will return an error if the user's home directory path cannot be found.
func filename() (string, error) {
	if credPath := os.Getenv("AWS_SHARED_CREDENTIALS_FILE"); credPath != "" {
		return credPath, nil
	}

	homeDir := getPlatformSpecificHomeLocation()
	if homeDir == "" {
		return "", awserr.New("UserHomeNotFound", "user home directory not found.", nil)
	}

	return filepath.Join(homeDir, ".aws", "credentials"), nil
}

func createFile(filePath string) error {
	dir, _ := filepath.Split(filePath)

	if err := fileutil.MakeDirs(dir); err != nil {
		return fmt.Errorf("error creating directories, %s. %v", dir, err)
	}

	if err := fileutil.HardenedWriteFile(filePath, []byte("")); err != nil {
		return fmt.Errorf("error creating file, %s. %v", filePath, err)
	}
	return nil
}

// Store function updates the shared credentials with the specified values:
// * If the shared credentials file does not exist, it will be created. Any parent directories will also be created.
// * If the section to update does not exist, it will be created.
func Store(log log.T, accessKeyID, secretAccessKey, sessionToken, profile string, force bool) error {
	if profile == "" {
		profile = defaultProfile
	}

	credPath, err := filename()
	if err != nil {
		return err
	}

	// check if file exists, if not create it
	if !fileutil.Exists(credPath) {
		err := createFile(credPath)
		if err != nil {
			return awserr.New("SharedCredentialsStore", "failed to create shared credentials file", err)
		}
	}

	config, err := ini.Load(credPath)
	if err != nil {
		if force {
			log.Warn("Failed to load shared credentials file. Force update is enabled, creating a new empty config.", err)
			config = ini.Empty()
		} else {
			return awserr.New("SharedCredentialsStore", "failed to load shared credentials file", err)
		}
	}

	iniProfile := config.Section(profile)

	// Default to empty string if not found
	iniProfile.Key(awsAccessKeyID).SetValue(accessKeyID)

	iniProfile.Key(awsSecretAccessKey).SetValue(secretAccessKey)

	iniProfile.Key(awsSessionToken).SetValue(sessionToken)

	err = config.SaveTo(credPath)
	if err != nil {
		return awserr.New("SharedCredentialsStore", "failed to save profile", err)
	}

	return nil
}
