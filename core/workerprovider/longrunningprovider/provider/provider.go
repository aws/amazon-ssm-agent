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

// Package provider implements logic for allowing interaction with worker processes
package provider

import (
	"encoding/json"
	"runtime/debug"
	"strconv"
	"sync"

	"github.com/aws/amazon-ssm-agent/common/message"
	"github.com/aws/amazon-ssm-agent/core/app/context"
	"github.com/aws/amazon-ssm-agent/core/executor"
	"github.com/aws/amazon-ssm-agent/core/workerprovider/longrunningprovider/model"
)

// IProvider is the interface for check process status and starts the process if it's not running
type IProvider interface {
	Start(map[string]*model.WorkerConfig, []*message.Message)
	Monitor(map[string]*model.WorkerConfig, []*message.Message)
	KillAllWorkerProcesses()
}

// WorkerProvider owns workerPool, it auto discovers the worker config and the running processes
type WorkerProvider struct {
	sync.Mutex
	workerPool map[string]*model.Worker
	context    context.ICoreAgentContext
	exec       executor.IExecutor
}

// NewWorkerProvider creates worker provider instance
func NewWorkerProvider(context context.ICoreAgentContext, exec executor.IExecutor) *WorkerProvider {

	ctx := context.With("[WorkerProvider]")
	return &WorkerProvider{
		exec:       exec,
		workerPool: make(map[string]*model.Worker),
		context:    ctx,
	}
}

// Start discovers the worker config and the running processes
// It starts a new process for the worker if the worker has no running process
func (w *WorkerProvider) Start(configs map[string]*model.WorkerConfig, pingResults []*message.Message) {

	w.discoverWorkers(configs, pingResults)
	w.terminateOrphanSsmAgentWorker()
	w.startWorkersIfNotRunning()
}

func (w *WorkerProvider) Monitor(configs map[string]*model.WorkerConfig, pingResults []*message.Message) {

	w.discoverWorkers(configs, pingResults)
	w.startWorkersIfNotRunning()
}

func (w *WorkerProvider) discoverWorkers(configs map[string]*model.WorkerConfig, pingResults []*message.Message) {
	logger := w.context.Log()
	defer func() {
		if msg := recover(); msg != nil {
			logger.Errorf("worker provider run panic: %v", msg)
			logger.Errorf("%s: %s", msg, debug.Stack())
		}
	}()

	// get all running processes from the process tree
	allProcesses, err := w.exec.Processes()
	if err != nil {
		logger.Errorf("failed to retrieve processes tree, %s", err.Error())
	}

	// delete worker from worker pool if the config is no longer available (removed)
	for _, worker := range w.workerPool {
		if _, ok := configs[worker.Name]; !ok {
			delete(w.workerPool, worker.Name)
		}
	}

	// update worker pool with the new worker config (added)
	for _, config := range configs {
		if _, ok := w.workerPool[config.Name]; !ok {
			w.workerPool[config.Name] = &model.Worker{
				Name:      config.Name,
				Config:    config,
				Processes: make(map[int]*model.Process),
			}
		}
	}

	// set worker pool status to unknown, since it's been 60 seconds without updates
	for _, worker := range w.workerPool {
		for _, process := range worker.Processes {
			process.Status = model.Unknown
		}
	}

	// update worker pool status bases on the health ping results
	logger.Debugf("Update worker pool base on the health ping results")
	for _, pingResult := range pingResults {
		var payload *message.HealthResultPayload

		if pingResult.Topic != message.GetWorkerHealthResult {
			logger.Warnf("unsupported message topic: %s, %s", pingResult.Topic, pingResult)
			continue
		}

		if err := json.Unmarshal(pingResult.Payload, &payload); err != nil {
			logger.Warnf("unable to unmarshal payload: %s", pingResult)
			continue
		}
		logger.Debugf("unmarshal payload content: %v", payload)

		if _, ok := w.workerPool[payload.Name]; ok {
			if _, ok := w.workerPool[payload.Name].Processes[payload.Pid]; ok {
				logger.Tracef("%s process (pid:%v) exists the worker pool, update status", payload.Name, payload.Pid)
				w.workerPool[payload.Name].Processes[payload.Pid].Status = model.Active
			} else {
				logger.Tracef("Found running %s process (pid:%v), add to the worker pool", payload.Name, payload.Pid)
				w.workerPool[payload.Name].Processes[payload.Pid] = &model.Process{
					Pid:    payload.Pid,
					Status: model.Active,
				}
			}
		}
	}

	// update worker pool status bases on the process tree
	// health ping is less reliable when multiple instances of core agent are running
	logger.Debugf("Update worker pool base on the process tree")
	for _, worker := range w.workerPool {

		for _, process := range allProcesses {
			logger.Tracef("process id %v, executable %s", process.Pid, process.Executable)
			if process.Executable == worker.Config.BinaryName {
				if _, ok := w.workerPool[worker.Name].Processes[process.Pid]; ok {
					logger.Tracef("%s process (pid:%v) exists the worker pool, update status", process.Executable, process.Pid)
					w.workerPool[worker.Name].Processes[process.Pid].Status = model.Active
				} else {
					logger.Tracef("Found running %s process (pid:%v), add to the worker pool", process.Executable, process.Pid)
					w.workerPool[worker.Name].Processes[process.Pid] = &model.Process{
						Pid:    process.Pid,
						Status: model.Active,
					}
				}
			}
		}
	}

	// remove the unknown process since it's most likely terminated
	for _, worker := range w.workerPool {
		for _, process := range worker.Processes {
			if process.Status == model.Unknown {
				logger.Infof(
					"Process %s (pid:%v) has been terminated, remove from worker pool",
					worker.Name,
					strconv.Itoa(process.Pid))

				// Clean up process in case it was not terminated correctly
				if err := w.exec.Kill(process.Pid); err != nil {
					logger.Debugf("Failed to clean up process %v for worker %s, %s", process.Pid, worker.Name, err)
				}
				delete(worker.Processes, process.Pid)
			}
		}
	}

	return
}

func (w *WorkerProvider) KillAllWorkerProcesses() {
	logger := w.context.Log()

	for _, worker := range w.workerPool {
		for _, process := range worker.Processes {
			if err := w.exec.Kill(process.Pid); err != nil {
				logger.Warnf("Failed to terminate %s process (pid:%v), %s", worker.Name, process.Pid, err)
				continue
			}
			delete(worker.Processes, process.Pid)
			logger.Debugf("Worker %s process (pid:%v) terminated", worker.Name, process.Pid)
		}
	}
}

func (w *WorkerProvider) terminateOrphanSsmAgentWorker() {
	logger := w.context.Log()

	for _, worker := range w.workerPool {
		if worker.Name == model.SSMAgentWorkerName {
			if len(worker.Processes) > 0 {
				logger.Infof("Found orphan %s process from previous execution, terminating...", worker.Name)
			}

			for _, process := range worker.Processes {
				if err := w.exec.Kill(process.Pid); err != nil {
					// work process cannot be terminated, this should never happen
					logger.Errorf("failed to terminate orphan %s, %s", model.SSMAgentWorkerName, err)
					continue
				}
				delete(worker.Processes, process.Pid)
				logger.Infof("Orphan %s process (pid:%v) terminated", worker.Name, process.Pid)
			}
		}
	}
}

func (w *WorkerProvider) startWorkersIfNotRunning() {
	logger := w.context.Log()

	// start process if worker has no running process
	for _, worker := range w.workerPool {
		if len(worker.Processes) == 0 {
			logger.Infof("Worker %s is not running, starting worker process", worker.Name)
			if process, err := w.exec.Start(worker.Config); err != nil {
				logger.Errorf("failed to start worker %v, %v", worker.Name, err)
			} else {
				w.workerPool[worker.Name].Processes[process.Pid] = process
				logger.Infof("Worker %s (pid:%v) started", worker.Name, strconv.Itoa(process.Pid))
			}
		} else {
			for _, process := range worker.Processes {
				logger.Debugf("Worker %s (pid:%v) is running, skip", worker.Name, process.Pid)
			}
		}
	}

	return
}
