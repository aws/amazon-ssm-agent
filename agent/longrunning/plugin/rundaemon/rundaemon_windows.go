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
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

// Created Executor function interfaces to allow for better testability
var DaemonCmdExecutor = RunDaemon
var BlockWhileDaemonRunningExecutor = BlockWhileDaemonRunning
var StopDaemonExecutor = StopDaemon
var IsDaemonRunningExecutor = IsDaemonRunning

//  RequestedDaemonStateType represents whether the user has explicitly requested to start/stop the daemon
type RequestedDaemonStateType uint

const (
	RequestedDisabled RequestedDaemonStateType = iota
	RequestedEnabled
)

//  CurrentDaemonStateType represents whether the daemon is currently running or not.
type CurrentDaemonStateType uint

const (
	CurrentStopped CurrentDaemonStateType = iota
	CurrentRunning
)

// Plugin is the type for the configureDaemon plugin.
type Plugin struct {
	pluginutil.DefaultPlugin
	// ExeLocation is the directory for a particular daemon package
	ExeLocation string
	// Name is name of the daemon
	Name string
	// CommandLine is command line to launch the daemon (On Windows, ame of executable or a powershell script)
	CommandLine string
	Process     *os.Process
	//ProcessStateLock lock is used to Protect access to daemon state updates
	ProcessStateLock sync.Mutex
	// RequestedDaemonState represents whether the user has explicitly requested to start/stop the daemon
	RequestedDaemonState RequestedDaemonStateType // 1 = Start. 0 = Stop
	// CurrentDaemonState represents whether the daemon is currently running or not.
	CurrentDaemonState CurrentDaemonStateType //  1 = Running, 0 = Stopped
}

// MinWaitBetweenRetries 60seconds
// Successive Daemon Restarts should be atleast 60sec apart
const MinWaitBetweenRetries = 60 * time.Second

// BlockWhileDaemonRunning checks if the process with the given process id is still running
// The function will block and the context swapped out while the underlying process is still running.
func BlockWhileDaemonRunning(context context.T, pid int) error {
	log := context.Log()
	process, err := os.FindProcess(pid)
	if err != nil {
		log.Infof("Daemon Not Running. Pid %v : %s", pid, err.Error())
		return err
	}
	log.Infof("Waiting for the process to die")
	// Control blocks here untill this process stops running (gets killed for example)
	_, err = process.Wait()
	return err
}

// IsRunning returns if the said plugin is running or not, to the long running plugin manager.
// We always return false here since the lifecycle of the underlying daemon is anyways being controlled here

func (p *Plugin) IsRunning(context context.T) bool {
	return false
}

func (p *Plugin) stopRequested() bool {
	p.ProcessStateLock.Lock()
	defer p.ProcessStateLock.Unlock()
	return p.RequestedDaemonState == RequestedDisabled
}

func IsDaemonRunning(p *Plugin) bool {
	p.ProcessStateLock.Lock()
	defer p.ProcessStateLock.Unlock()
	return p.CurrentDaemonState == CurrentRunning
}

func RunDaemon(daemonInvoke *exec.Cmd) (err error) {
	err = daemonInvoke.Start()
	return err
}

// Starts a given executable or a specified powershell script and enables daemon functionality
func StartDaemon(p *Plugin, context context.T, configuration string) (err error) {
	// Bail out if an explicit Stop daemon is requested by the user
	if p.stopRequested() {
		return
	}

	// Below loop initiates daemon startup and then goes to sleep once the daemon
	// starts running. Once the daemon exits, it will attempt to retry launching
	// daemon unless the user has explicitly requested a stop.

	for {
		start := time.Now()
		log := context.Log()
		if p.Process != nil {
			err := BlockWhileDaemonRunningExecutor(context, p.Process.Pid)
			if err != nil {
				log.Infof("Encountered error: process may not have exited cleanly. Pid %v : %s", p.Process.Pid, err.Error())
			}
		}

		// Bail out if an explicit Stop daemon is requested by the user
		if p.stopRequested() {
			return
		}

		log.Infof("Attempting to Start Daemon")

		//TODO Currently pathnames with spaces do not seem to work correctly with the below
		// usage of exec.command. Given that ConfigurePackage defaults to a directory name which
		// doesnt have spaces (C:/ProgramData/Amazon/SSM/....), the issue is not currently exposed.
		// Needs to be fixed regardless.

		commandArguments := append(strings.Split(configuration, " "), pluginutil.ExitCodeTrap)
		log.Infof("Running command: %v.", commandArguments)

		daemonInvoke := exec.Command(commandArguments[0], commandArguments[1:]...)
		daemonInvoke.Dir = p.ExeLocation
		err := DaemonCmdExecutor(daemonInvoke)

		if err != nil {
			log.Errorf("Error starting Daemon: %s", err.Error())
			return err
		}
		p.ProcessStateLock.Lock()
		p.Process = daemonInvoke.Process
		p.CurrentDaemonState = CurrentRunning
		p.ProcessStateLock.Unlock()

		log.Infof("Started Daemon")

		// Setting the time between successive daemon restarts to be atleast 60 seconds
		end := time.Now()
		elapsedMilliSecs := end.Sub(start)
		MinWaitMilliSecs := MinWaitBetweenRetries
		if elapsedMilliSecs < MinWaitMilliSecs {
			log.Infof("Waiting %v milliseconds to start %s again", MinWaitMilliSecs-elapsedMilliSecs, p.Name)
			time.Sleep(MinWaitMilliSecs - elapsedMilliSecs)
		}
	}
}

func (p *Plugin) Start(context context.T, configuration string, orchestrationDir string, cancelFlag task.CancelFlag) error {
	log := context.Log()
	log.Infof(" Package location - %s", p.ExeLocation)
	log.Infof(" Command/Script/Executable to be run - %s", configuration)

	// Start a new daemon handling goroutine if the goroutine is currently not running
	p.ProcessStateLock.Lock()
	//TODO Explore replacing the need to call Unlock in multiple places with a defer statement.
	// Need to test before committing that change.
	if p.CurrentDaemonState == CurrentStopped {
		log.Infof(" Invoking goroutine to manage daemon lifecycle")
		// Set the User Requested state to ENABLED
		p.RequestedDaemonState = RequestedEnabled
		p.ProcessStateLock.Unlock()
		go StartDaemon(p, context, configuration)
	} else {
		p.ProcessStateLock.Unlock()
	}
	return nil
}

func StopDaemon(p *Plugin, context context.T) {
	log := context.Log()
	p.ProcessStateLock.Lock()
	//TODO Explore replacing the need to call Unlock below with a defer statement here.
	if p.Process != nil {
		log.Infof("Process id of daemon -> %v", p.Process.Pid)
		if p.CurrentDaemonState == CurrentRunning {
			err := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(p.Process.Pid)).Run()
			if err != nil {
				log.Infof("Encountered error while trying to stop the child processes %v : %s", p.Process.Pid, err.Error())
			} else {
				log.Infof("Successfully stopped the children of process %v", p.Process.Pid)
			}
			if err = p.Process.Kill(); err != nil {
				log.Infof("Encountered error while trying to kill the process %v : %s", p.Process.Pid, err.Error())
			} else {
				log.Infof("Successfully stopped the process %v", p.Process.Pid)
			}
			p.RequestedDaemonState = RequestedDisabled
			p.CurrentDaemonState = CurrentStopped
			p.Process = nil
		}
	}
	p.ProcessStateLock.Unlock()
}

func (p *Plugin) Stop(context context.T, cancelFlag task.CancelFlag) error {
	log := context.Log()
	log.Infof("Stopping Daemon")
	StopDaemonExecutor(p, context)
	return nil
}
