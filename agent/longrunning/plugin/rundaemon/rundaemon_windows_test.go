//go:build windows
// +build windows

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
//
// Package rundaemon implements rundaemon plugin and its configuration
package rundaemon

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	iohandlermocks "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler/mock"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
)

var pluginConfig = iohandler.PluginConfig{
	StdoutFileName:        "stdout",
	StderrFileName:        "stderr",
	MaxStdoutLength:       2500,
	MaxStderrLength:       2500,
	OutputTruncatedSuffix: "cw",
}

func NewPlugin(context context.T, pluginConfig iohandler.PluginConfig) (*Plugin, error) {
	var plugin Plugin
	plugin.MaxStdoutLength = pluginConfig.MaxStdoutLength
	plugin.MaxStderrLength = pluginConfig.MaxStderrLength
	plugin.StdoutFileName = pluginConfig.StdoutFileName
	plugin.StderrFileName = pluginConfig.StderrFileName
	plugin.OutputTruncatedSuffix = pluginConfig.OutputTruncatedSuffix
	plugin.Context = context
	return &plugin, nil
}

func MockRunDaemonExecutorWithNoError(daemonInvoke *exec.Cmd) (err error) {
	return nil
}

func MockStopDaemonExecutorWithNoError(p *Plugin) {
	return
}

func MockStartDaemonHelperExecutor(p *Plugin, configuration string) error {
	return nil
}

func MockBlockWhileDaemonRunning(context context.T, pid int) error {
	time.Sleep(2 * time.Second)
	return nil
}

func MockIsDaemonRunningExecutor(p *Plugin) bool {
	return true
}

// Test to perform a Start followed by a Stop operation
func TestSingleStartStop(t *testing.T) {
	context := context.NewMockDefault()
	cancelFlag := task.NewMockDefault()
	ioHandler := &iohandlermocks.MockIOHandler{}
	p, _ := NewPlugin(context, pluginConfig)
	p.Name = "TestSingleStartStop"
	DaemonCmdExecutor = MockRunDaemonExecutorWithNoError
	BlockWhileDaemonRunningExecutor = MockBlockWhileDaemonRunning
	StopDaemonExecutor = MockStopDaemonExecutorWithNoError
	StartDaemonHelperExecutor = MockStartDaemonHelperExecutor
	IsDaemonRunningExecutor = MockIsDaemonRunningExecutor
	t.Logf("Daemon starting")
	err := p.Start("powershell Sleep 5", "", cancelFlag, ioHandler)
	assert.NoError(t, err, fmt.Sprintf("Expected no error but got %v", err))
	time.Sleep(2 * time.Second)
	t.Logf("Daemon is running")
	if IsDaemonRunningExecutor(p) {
	} else {
		t.Fatalf("Daemon is not running. Bail out")
	}
	time.Sleep(2 * time.Second)
	err = p.Stop(cancelFlag)
	assert.NoError(t, err, fmt.Sprintf("Expected no error but got %v", err))

	if p.Process != nil {
		err = BlockWhileDaemonRunningExecutor(context, p.Process.Pid)
		assert.NoError(t, err, fmt.Sprintf("Expected no error but got %v", err))
	}
	t.Logf("Daemon stopped")
}

// Test to perform Successive Starts
func TestSuccessiveStarts(t *testing.T) {
	context := context.NewMockDefault()
	cancelFlag := task.NewMockDefault()
	ioHandler := &iohandlermocks.MockIOHandler{}

	p, _ := NewPlugin(context, pluginConfig)
	var pid int
	p.Name = "TestSuccessiveStarts"
	DaemonCmdExecutor = MockRunDaemonExecutorWithNoError
	BlockWhileDaemonRunningExecutor = MockBlockWhileDaemonRunning
	StopDaemonExecutor = MockStopDaemonExecutorWithNoError
	StartDaemonHelperExecutor = MockStartDaemonHelperExecutor
	IsDaemonRunningExecutor = MockIsDaemonRunningExecutor
	t.Logf("Daemon starting")
	p.Start("powershell Sleep 5", "", cancelFlag, ioHandler)
	time.Sleep(1 * time.Second)
	t.Logf("Daemon is running")
	if IsDaemonRunningExecutor(p) {
	} else {
		t.Fatalf("Daemon is not running. Bail out")
	}
	time.Sleep(1 * time.Second)
	if p.Process != nil {
		pid = p.Process.Pid
	}
	p.Start("", "", cancelFlag, ioHandler)
	time.Sleep(2 * time.Second)
	if p.Process != nil {
		if p.Process.Pid == pid {
			t.Logf("Daemon was already running")
		} else {
			t.Fatalf("Another instance of daemon started while one running")
		}
	}
	p.Stop(cancelFlag)
	BlockWhileDaemonRunning(context, pid)
	t.Logf("Daemon stopped")
}

// Test to perform Multiple Start-Stops
func TestMultipleStartStop(t *testing.T) {
	context := context.NewMockDefault()
	cancelFlag := task.NewMockDefault()
	ioHandler := &iohandlermocks.MockIOHandler{}
	p, _ := NewPlugin(context, pluginConfig)
	p.Name = "TestMultipleStartStop"
	DaemonCmdExecutor = RunDaemon
	BlockWhileDaemonRunningExecutor = BlockWhileDaemonRunning
	StopDaemonExecutor = StopDaemon
	StartDaemonHelperExecutor = StartDaemonHelper
	IsDaemonRunningExecutor = IsDaemonRunning

	for i := 0; i < 50; i++ {
		t.Logf("Daemon starting")
		p.Start("powershell Sleep 5", "", cancelFlag, ioHandler)
		time.Sleep(5 * time.Second)
		if p.Process != nil {
			proc, err := os.FindProcess(p.Process.Pid)
			if err != nil {
				t.Fatalf("Daemon is not running. Bail out")
				return
			} else {
				t.Logf("Process pid %v", proc.Pid)
			}
		} else {
			t.Fatalf("Daemon is not running. Bail out")
			return
		}
		pid := p.Process.Pid
		t.Logf("Daemon stopping")
		p.Stop(cancelFlag)
		BlockWhileDaemonRunningExecutor(context, pid)
	}
}

// Test to perform stop without an associated start
func TestStopWithoutStart(t *testing.T) {
	context := context.NewMockDefault()
	cancelFlag := task.NewMockDefault()
	p, _ := NewPlugin(context, pluginConfig)
	DaemonCmdExecutor = MockRunDaemonExecutorWithNoError
	BlockWhileDaemonRunningExecutor = MockBlockWhileDaemonRunning
	StopDaemonExecutor = MockStopDaemonExecutorWithNoError
	StartDaemonHelperExecutor = MockStartDaemonHelperExecutor
	IsDaemonRunningExecutor = MockIsDaemonRunningExecutor
	p.Name = "TestStopWithoutStart"
	t.Logf("Attempting to Stopping a Daemon without starting")
	err := p.Stop(cancelFlag)
	if err != nil {
		t.Fatalf("Stop returned errors")
	}
	return
}
