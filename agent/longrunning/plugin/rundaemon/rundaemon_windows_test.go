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
//
// Package rundaemon implements rundaemon plugin and its configuration
package rundaemon

import (
	"os"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

var pluginConfig = pluginutil.PluginConfig{
	StdoutFileName:        "stdout",
	StderrFileName:        "stderr",
	MaxStdoutLength:       2500,
	MaxStderrLength:       2500,
	OutputTruncatedSuffix: "cw",
}

func NewPlugin(pluginConfig pluginutil.PluginConfig) (*Plugin, error) {
	var plugin Plugin
	plugin.MaxStdoutLength = pluginConfig.MaxStdoutLength
	plugin.MaxStderrLength = pluginConfig.MaxStderrLength
	plugin.StdoutFileName = pluginConfig.StdoutFileName
	plugin.StderrFileName = pluginConfig.StderrFileName
	plugin.OutputTruncatedSuffix = pluginConfig.OutputTruncatedSuffix
	return &plugin, nil
}

// Test to perform a Start followed by a Stop operation
func TestSingleStartStop(t *testing.T) {
	context := context.NewMockDefault()
	cancelFlag := task.NewMockDefault()
	p, _ := NewPlugin(pluginConfig)
	p.ExeLocation = `C:\testing`
	p.Name = "TestStart"
	componentName := "http.exe"
	t.Logf("Daemon starting")
	p.Start(context, componentName, "", cancelFlag)
	time.Sleep(2 * time.Second)
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
	p.Stop(context, cancelFlag)
	BlockWhileDaemonRunning(context, pid)
	t.Logf("Daemon stopped")
}

// Test to perform Successive Starts
func TestSuccessiveStarts(t *testing.T) {
	context := context.NewMockDefault()
	cancelFlag := task.NewMockDefault()
	p, _ := NewPlugin(pluginConfig)
	p.ExeLocation = `C:\testing`
	p.Name = "TestStart"
	componentName := "http.exe"
	t.Logf("Daemon starting")
	p.Start(context, componentName, "", cancelFlag)
	time.Sleep(2 * time.Second)
	if p.Process != nil {
		proc, err := os.FindProcess(p.Process.Pid)
		if err != nil {
			t.Fatalf("Daemon is not running. Bail out")
		} else {
			t.Logf("Process pid %v", proc.Pid)
		}
	} else {
		t.Fatalf("Daemon is not running. Bail out")
		return
	}
	pid := p.Process.Pid
	p.Start(context, componentName, "", cancelFlag)
	time.Sleep(10 * time.Second)
	if p.Process.Pid == pid {
		t.Logf("Daemon was already running")
	}
	p.Stop(context, cancelFlag)
	BlockWhileDaemonRunning(context, pid)
	t.Logf("Daemon stopped")

}

// Test to perform Multiple Start-Stops
func TestMultipleStartStop(t *testing.T) {
	context := context.NewMockDefault()
	cancelFlag := task.NewMockDefault()
	p, _ := NewPlugin(pluginConfig)
	p.ExeLocation = `C:\\testing`
	p.Name = "TestStart"
	componentName := "http.exe"
	time.Sleep(10 * time.Second)
	for i := 0; i < 50; i++ {
		t.Logf("Daemon starting")
		p.Start(context, componentName, "", cancelFlag)
		time.Sleep(2 * time.Second)
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
		p.Stop(context, cancelFlag)
		BlockWhileDaemonRunning(context, pid)
	}
}

// Test to perform stop without an associated start
func TestStopWithoutStart(t *testing.T) {
	context := context.NewMockDefault()
	cancelFlag := task.NewMockDefault()
	p, _ := NewPlugin(pluginConfig)
	p.ExeLocation = "C:\\testing"
	p.Name = "TestStart"
	t.Logf("Attempting to Stopping a Daemon without starting")
	err := p.Stop(context, cancelFlag)
	if err != nil {
		t.Fatalf("Stop returned errors")
	}
	return
}
