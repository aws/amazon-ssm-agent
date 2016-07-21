// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the AWS Customer Agreement (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/agreement/

// package sharedCredentials provides access to the aws shared credentials file.
//
// It will look for "AWS_SHARED_CREDENTIALS_FILE" env variable.
// If the env value is empty will default to current user's home directory.
// Linux/OSX: "$HOME/.aws/credentials"
// Windows:   "%USERPROFILE%\.aws\credentials"
package sharedCredentials

import (
	"fmt"
	"os"
	"path/filepath"

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
	// Look for "AWS_SHARED_CREDENTIALS_FILE" env variable.
	// If the env value is empty will default to current user's home directory.
	// Linux/OSX: "$HOME/.aws/credentials"
	// Windows:   "%USERPROFILE%\.aws\credentials"
	if credPath := os.Getenv("AWS_SHARED_CREDENTIALS_FILE"); credPath != "" {
		return credPath, nil
	}

	homeDir := os.Getenv("HOME") // *nix
	if homeDir == "" {           // Windows
		homeDir = os.Getenv("USERPROFILE")
	}
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
func Store(accessKeyID, secretAccessKey, sessionToken, profile string) error {
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
		return awserr.New("SharedCredentialsStore", "failed to load shared credentials file", err)
	}

	iniProfile := config.Section(profile)
	if err != nil {
		return awserr.New("SharedCredentialsStore", "failed to get profile", err)
	}

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
