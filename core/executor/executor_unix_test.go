// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

//go:build freebsd || linux || netbsd || openbsd
// +build freebsd linux netbsd openbsd

// Package executor wraps up the os.Process interface and also provides os-specific process lookup functions
package executor

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/core/workerprovider/longrunningprovider/model"
	"github.com/stretchr/testify/assert"
)

var logger = log.NewMockLog()

// TODO add process start time
func TestIsProcessPsExists(t *testing.T) {
	cmdString := "sleep"
	cmd := exec.Command(cmdString, "5")
	err := cmd.Start()
	//do not call wait in case the process are recycled
	assert.NoError(t, err)
	pid := cmd.Process.Pid
	ppid := os.Getpid()
	logger.Infof("process pid: %v", pid)

	processes, err := getProcess()
	found := false
	for _, process := range processes {
		if process.Pid == pid && process.PPid == ppid && process.Executable == cmdString {
			found = true
		}
	}
	assert.True(t, found)
}

func TestIsProcessProcExists(t *testing.T) {
	oldListProcessPs := listProcessPs
	listProcessPs = func() ([]byte, error) {
		return nil, fmt.Errorf("SomeRandomError")
	}
	defer func() { listProcessPs = oldListProcessPs }()
	cmdString := "sleep"
	cmd := exec.Command(cmdString, "5")
	err := cmd.Start()
	//do not call wait in case the process are recycled
	assert.NoError(t, err)
	pid := cmd.Process.Pid
	ppid := os.Getpid()
	logger.Infof("process pid: %v", pid)

	processes, err := getProcess()
	found := false
	for _, process := range processes {
		if process.Pid == pid && process.PPid == ppid && process.Executable == cmdString {
			found = true
		}
	}
	assert.True(t, found)
}

func TestStartProcess(t *testing.T) {
	mockLog := log.NewMockLog()
	executor := ProcessExecutor{mockLog}

	ssmAgentWorker := &model.WorkerConfig{
		Name:       model.SSMAgentWorkerName,
		BinaryName: model.SSMAgentWorkerBinaryName,
		Path:       "sleep",
		Args:       []string{"2"},
	}

	process, err := executor.Start(ssmAgentWorker)

	assert.Nil(t, err)
	assert.NotNil(t, process)
}

func TestKillProcess(t *testing.T) {
	mockLog := log.NewMockLog()
	executor := ProcessExecutor{mockLog}

	ssmAgentWorker := &model.WorkerConfig{
		Name:       model.SSMAgentWorkerName,
		BinaryName: model.SSMAgentWorkerBinaryName,
		Path:       "sleep",
		Args:       []string{"5"},
	}

	process, err := executor.Start(ssmAgentWorker)

	assert.Nil(t, err)
	assert.NotNil(t, process)

	err = executor.Kill(process.Pid)
	assert.Nil(t, err)
}
