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

// Package service is a wrapper for the SSM Message Delivery Service and Offline Command Service
package service

import (
	"path/filepath"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
)

var logger = log.NewMockLog()

const (
	newCommands       = "testdata/new"
	submittedCommands = "testdata/new/submitted"
	invalidCommands   = "testdata/new/invalid"
	completeDir       = "testdata/new/completed"
)

func TestValid(t *testing.T) {
	service := GetTestService()

	defer CleanTestDirs()
	err := SubmitTestDoc("validcommand20.json")
	assert.Nil(t, err)

	messages, err := service.GetMessages(logger, "i-bar")

	assert.Nil(t, err)
	assert.Equal(t, 1, len(messages.Messages))
	assert.Equal(t, 0, FileCount(newCommands))
	assert.Equal(t, 1, FileCount(submittedCommands))
}

func TestInvalid(t *testing.T) {
	service := GetTestService()

	defer CleanTestDirs()
	err := SubmitTestDoc("invalidcommand.json")
	assert.Nil(t, err)

	messages, err := service.GetMessages(logger, "i-bar")

	assert.Nil(t, err)
	assert.Equal(t, 0, len(messages.Messages))
	assert.Equal(t, 0, FileCount(newCommands))
	assert.Equal(t, 1, FileCount(invalidCommands))
}

func TestBothVersions(t *testing.T) {
	service := GetTestService()

	defer CleanTestDirs()
	var err error
	err = SubmitTestDoc("validcommand20.json")
	assert.Nil(t, err)
	err = SubmitTestDoc("validcommand12.json")
	assert.Nil(t, err)

	messages, err := service.GetMessages(logger, "i-bar")

	assert.Nil(t, err)
	assert.Equal(t, 2, len(messages.Messages))
	assert.Equal(t, 0, FileCount(newCommands))
	assert.Equal(t, 2, FileCount(submittedCommands))
}

func TestOfflineService_SendReply(t *testing.T) {
	service := GetTestService()
	defer CleanTestDirs()
	service.SendReply(logger, "aws.ssm.testCommandID.testInstanceID", "payload")
	assert.Equal(t, 1, FileCount(completeDir))
}

func GetTestService() Service {
	CleanTestDirs()
	return &offlineService{
		TopicPrefix:         "foo",
		newCommandDir:       newCommands,
		submittedCommandDir: submittedCommands,
		invalidCommandDir:   invalidCommands,
		commandResultDir:    completeDir,
	}
}

func SubmitTestDoc(name string) error {
	if doc, err := fileutil.ReadAllText(filepath.Join("testdata", name)); err != nil {
		return err
	} else {
		return fileutil.WriteAllText(filepath.Join(newCommands, name), doc)
	}
}

func CleanTestDirs() {
	var files []string
	files, _ = fileutil.GetFileNames(submittedCommands)
	for _, file := range files {
		fileutil.DeleteFile(filepath.Join(submittedCommands, file))
	}
	files, _ = fileutil.GetFileNames(invalidCommands)
	for _, file := range files {
		fileutil.DeleteFile(filepath.Join(invalidCommands, file))
	}
	files, _ = fileutil.GetFileNames(newCommands)
	for _, file := range files {
		fileutil.DeleteFile(filepath.Join(newCommands, file))
	}
	files, _ = fileutil.GetFileNames(completeDir)
	for _, file := range files {
		fileutil.DeleteFile(filepath.Join(completeDir, file))
	}
}

func FileCount(path string) int {
	var files []string
	files, _ = fileutil.GetFileNames(path)
	return len(files)
}
