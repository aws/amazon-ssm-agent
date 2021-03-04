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
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/longrunning/jobobject"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

// Created Executor function interfaces to allow for better testability
var DaemonCmdExecutor = RunDaemon
var BlockWhileDaemonRunningExecutor = BlockWhileDaemonRunning
var StopDaemonExecutor = StopDaemon
var IsDaemonRunningExecutor = IsDaemonRunning
var StartDaemonHelperExecutor = StartDaemonHelper

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
	iohandler.PluginConfig
	// Context is the agent context for config, identity and logger
	Context context.T
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

// MaxThresholdToResetRetryCounter 5 hours
// If the daemon process exits happens after this specified threshold, then
// the Retrycount is set back to 0.
const MaxTimeThresholdToResetRetryCounter = 18000 * time.Second

// MaxRetryCountDuringFailures
const MaxRetryCountDuringFailures = 10

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
	// Control blocks here until this process stops running (gets killed for example)
	_, err = process.Wait()
	return err
}

// IsRunning returns if the said plugin is running or not, to the long running plugin manager.
// We always return false here since the lifecycle of the underlying daemon is anyways being controlled here
func (p *Plugin) IsRunning() bool {
	return false
}

// This function sets the flag to indicate that daemon stop has been requested via the StopPlugin call.
func (p *Plugin) stopRequested() bool {
	p.ProcessStateLock.Lock()
	defer p.ProcessStateLock.Unlock()
	return p.RequestedDaemonState == RequestedDisabled
}

// This function sets the daemon current state to being stopped.
func (p *Plugin) SetDaemonStateStopped() {
	p.ProcessStateLock.Lock()
	defer p.ProcessStateLock.Unlock()
	p.CurrentDaemonState = CurrentStopped
}

// Function RunDaemon invokes exec.Cmd.Start with appropriate arguments.
func RunDaemon(daemonInvoke *exec.Cmd) (err error) {
	err = daemonInvoke.Start()
	return err
}

// Function checks if the daemon is currently running or not.
func IsDaemonRunning(p *Plugin) bool {
	p.ProcessStateLock.Lock()
	defer p.ProcessStateLock.Unlock()
	return p.CurrentDaemonState == CurrentRunning
}

// Starts a given executable or a specified powershell script and enables daemon functionality
func StartDaemonHelper(p *Plugin, configuration string) (err error) {
	log := p.Context.Log()
	if IsDaemonRunningExecutor(p) {
		log.Infof("Daemon already running: %v", configuration)
		return
	}

	p.ProcessStateLock.Lock()
	defer p.ProcessStateLock.Unlock()
	log.Infof("Attempting to Start Daemon")

	//TODO Currently pathnames with spaces do not seem to work correctly with the below
	// usage of exec.command. Given that ConfigurePackage defaults to a directory name which
	// doesnt have spaces (C:/ProgramData/Amazon/SSM/....), the issue is not currently exposed.
	// Needs to be fixed regardless.

	commandArguments := append(strings.Split(configuration, " "))
	log.Infof("Running command: %v.", commandArguments)

	daemonInvoke := exec.Command(commandArguments[0], commandArguments[1:]...)
	daemonInvoke.Dir = p.ExeLocation
	err = DaemonCmdExecutor(daemonInvoke)

	if err != nil {
		log.Errorf("Error starting Daemon: %s", err.Error())
		return err
	}
	p.Process = daemonInvoke.Process

	// Attach daemon process to the SSM agent job object
	err = jobobject.AttachProcessToJobObject(uint32(daemonInvoke.Process.Pid))
	if err != nil {
		log.Errorf("Error attaching job object to Daemon: %s", err.Error())
	} else {
		log.Debugf("Successfully attached job object to Daemon")
	}
	p.CurrentDaemonState = CurrentRunning
	return
}

// Starts a given executable or a specified powershell script and enables daemon functionality
func StartDaemon(p *Plugin, configuration string) (err error) {
	log := p.Context.Log()
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Start daemon panic: \n%v", r)
			log.Errorf("Stacktrace:\n%s", debug.Stack())
		}
	}()
	RetryCount := 0
	// Bail out if an explicit Stop daemon is requested by the user
	if p.stopRequested() {
		log.Infof("Daemon requested to be stopped: %v", configuration)
		return
	}

	// Below loop initiates daemon startup and then goes to sleep once the daemon
	// starts running. Once the daemon exits, it will attempt to retry launching
	// daemon unless the user has explicitly requested a stop.

	for {
		start := time.Now()
		log := p.Context.Log()
		if p.Process != nil {
			err = BlockWhileDaemonRunningExecutor(p.Context, p.Process.Pid)
			p.SetDaemonStateStopped()
			if err != nil {
				log.Infof("Encountered error: process may not have exited cleanly. Pid %v : %s", p.Process.Pid, err.Error())
			}
		}

		// Bail out if an explicit Stop daemon is requested by the user
		if p.stopRequested() {
			log.Infof("Daemon requested to be stopped: %v", configuration)
			return
		}
		// Invoke the helper function to start daermo
		err = StartDaemonHelperExecutor(p, configuration)
		if err == nil {
			log.Infof("Started Daemon...")
		}

		// Setting the time between successive daemon restarts to be atleast 60 seconds
		end := time.Now()
		elapsedSecs := end.Sub(start)
		MinWaitSecs := MinWaitBetweenRetries
		MaxTimeToResetRetryCount := MaxTimeThresholdToResetRetryCounter
		if elapsedSecs < MinWaitSecs {
			RetryCount++
			if RetryCount > MaxRetryCountDuringFailures {
				log.Infof("Daemon %v process exited for %v times within the minimum threshold time window from its startup. Bailing out.", p.Name, RetryCount)
				return
			}
			log.Infof("Waiting %v seconds to start %s again", MinWaitSecs-elapsedSecs, p.Name)
			time.Sleep(MinWaitSecs - elapsedSecs)
		} else if elapsedSecs > MaxTimeToResetRetryCount {
			log.Infof("Setting retrycount for %s daemon back to 0", p.Name)
			RetryCount = 0
		}
	}
}

func (p *Plugin) Start(configuration string, orchestrationDir string, cancelFlag task.CancelFlag, out iohandler.IOHandler) error {
	log := p.Context.Log()
	log.Infof(" Package location - %s", p.ExeLocation)
	log.Infof(" Command/Script/Executable to be run - %s", configuration)

	// Start a new daemon handling goroutine if the goroutine is currently not running
	p.ProcessStateLock.Lock()
	if p.CurrentDaemonState == CurrentStopped {
		log.Infof(" Invoking goroutine to manage daemon lifecycle")
		// Set the User Requested state to ENABLED
		p.RequestedDaemonState = RequestedEnabled
		p.ProcessStateLock.Unlock()
		go StartDaemon(p, configuration)
	} else {
		p.ProcessStateLock.Unlock()
	}
	return nil
}

func StopDaemon(p *Plugin) {
	log := p.Context.Log()
	p.ProcessStateLock.Lock()
	defer p.ProcessStateLock.Unlock()
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
}

func (p *Plugin) Stop(cancelFlag task.CancelFlag) error {
	log := p.Context.Log()
	log.Infof("Stopping Daemon")
	StopDaemonExecutor(p)
	return nil
}
