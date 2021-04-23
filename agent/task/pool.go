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

package task

import (
	"fmt"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/times"
)

// Pool is a pool of jobs.
type Pool interface {
	// Submit schedules a job to be executed in the associated worker pool.
	// Returns an error if a job with the same name already exists.
	Submit(log log.T, jobID string, job Job) error

	// Cancel cancels the given job. Jobs that have not started yet will never be started.
	// Jobs that are running will have their CancelFlag set to the Canceled state.
	// It is the responsibility of the job to terminate within a reasonable time.
	// If the job fails to terminate after a Cancel, the job may be abandoned.
	// Returns true if the job has been found and canceled, false if the job was not found.
	Cancel(jobID string) bool

	// Shutdown cancels all the jobs and shuts down the workers.
	Shutdown()

	// ShutdownAndWait calls Shutdown then waits until all the workers have exited
	// or until the timeout has elapsed, whichever comes first. Returns true if all
	// workers terminated before the timeout or false if the timeout expired.
	ShutdownAndWait(timeout time.Duration) (finished bool)

	// HasJob returns if jobStore has specified job
	HasJob(jobID string) bool
}

// pool implements a task pool where all jobs are managed by a root task
type pool struct {
	log            log.T
	jobQueue       chan JobToken
	maxWorkers     int
	doneWorker     chan struct{}
	jobHandlerDone chan struct{}
	isShutdown     bool
	clock          times.Clock
	mut            sync.Mutex
	jobStore       *JobStore
	cancelDuration time.Duration
}

// JobToken embeds a job and its associated info
type JobToken struct {
	id         string
	job        Job
	cancelFlag *ChanneledCancelFlag
	log        log.T
}

// NewPool creates a new task pool and launches maxParallel workers.
// The cancelWaitDuration parameter defines how long to wait for a job
// to complete a cancellation request.
func NewPool(log log.T, maxParallel int, cancelWaitDuration time.Duration, clock times.Clock) Pool {
	p := &pool{
		log:            log,
		jobQueue:       make(chan JobToken),
		maxWorkers:     maxParallel,
		doneWorker:     make(chan struct{}),
		jobHandlerDone: make(chan struct{}),
		clock:          clock,
		cancelDuration: cancelWaitDuration,
	}

	p.jobStore = NewJobStore()

	// defines the job processing function.
	processor := func(j JobToken) {
		defer p.jobStore.DeleteJob(j.id)
		process(j.log, j.job, j.cancelFlag, cancelWaitDuration, p.clock)
	}

	// start job handler
	go p.startJobHandler(processor)

	return p
}

// Shutdown cancels all the jobs in this pool and shuts down the workers.
func (p *pool) Shutdown() {
	// ShutDown and delete all jobs
	p.ShutDownAll()

	p.mut.Lock()
	defer p.mut.Unlock()
	if !p.isShutdown {
		// close the channel to makes all workers terminate once the pending
		// jobs have been consumed (the pending jobs are in the Canceled state
		// so they will simply be discarded)
		close(p.jobQueue)
		p.isShutdown = true
	}
}

// ShutdownAndWait calls Shutdown then waits until all the workers have exited
// or until the timeout has elapsed, whichever comes first. Returns true if all
// workers terminated before the timeout or false if the timeout expired.
func (p *pool) ShutdownAndWait(timeout time.Duration) (finished bool) {
	p.Shutdown()

	timeoutTimer := p.clock.After(timeout)
	exitTimer := p.clock.After(timeout + p.cancelDuration)

	for {
		select {
		case <-p.jobHandlerDone:
			p.log.Debug("Pool shutdown normally.")
			return true
		case <-timeoutTimer: // timeoutTimer will always trigger before exitTimer
			p.log.Debug("Pool shutdown timed, start cancelling jobs...")
			// wait for the worker pool to react to the cancel flag and fail the ongoing jobs
			p.CancelAll()
		case <-exitTimer:
			p.log.Debug("Pool eventual timeout with workers still running")
			return false
		}
	}
}

// startJobHandler starts the job handler
func (p *pool) startJobHandler(jobProcessor func(JobToken)) {
	workerCount := 0

exitLoopLabel:
	for {
		// If there are too many workers currently running, wait for worker before trying to start a new job
		if workerCount >= p.maxWorkers {
			p.log.Debug("Max workers are running, waiting for a worker to complete")
			<-p.doneWorker
			p.log.Debug("Worker completed, can start next job")
			workerCount--
		}

		// now there are workers available, wait for a job or a worker to finish
		select {
		case job, ok := <-p.jobQueue:
			if !ok {
				p.log.Info("JobQueue has been closed")
				break exitLoopLabel
			}

			p.log.Infof("Got job %s, starting worker", job.id)
			workerCount++
			go func() {
				defer p.workerDone()
				if !job.cancelFlag.Canceled() {
					jobProcessor(job)
				}
			}()
		case <-p.doneWorker:
			p.log.Debug("Worker completed")
			workerCount--
		}
	}

	// Wait for all workers
	for workerCount != 0 {
		<-p.doneWorker
		p.log.Debug("Worker completed after shutdown")
		workerCount--
	}

	p.log.Info("All workers have finished and pool has been put into shutdown")
	close(p.jobHandlerDone)
}

// workerDone signals that a worker has terminated.
func (p *pool) workerDone() {
	p.doneWorker <- struct{}{}
}

// Submit adds a job to the execution queue of this pool.
func (p *pool) Submit(log log.T, jobID string, job Job) (err error) {
	if p.checkIsShutDown() {
		p.log.Errorf("Attempting to add job %s to a closed queue", jobID)
		return fmt.Errorf("pool is closed")
	}

	token := JobToken{
		id:         jobID,
		job:        job,
		cancelFlag: NewChanneledCancelFlag(),
		log:        log,
	}
	err = p.jobStore.AddJob(jobID, &token)
	if err != nil {
		return
	}
	p.jobQueue <- token
	return
}

// HasJob returns if jobStore has specified job
func (p *pool) HasJob(jobID string) bool {
	_, found := p.jobStore.GetJob(jobID)
	return found
}

// Cancel cancels the job with the given id.
func (p *pool) Cancel(jobID string) (canceled bool) {
	jobToken, found := p.jobStore.GetJob(jobID)
	if !found {
		return false
	}

	// delete job to avoid multiple cancelations
	p.jobStore.DeleteJob(jobID)

	jobToken.cancelFlag.Set(Canceled)
	return true
}

// checkIsShutDown safely reads if the pool has been set into shutdown
func (p *pool) checkIsShutDown() bool {
	p.mut.Lock()
	defer p.mut.Unlock()
	return p.isShutdown
}

// CancelAll cancels all the running jobs.
func (p *pool) CancelAll() {
	// remove jobs from task and save them to a local variable
	jobs := p.jobStore.DeleteAllJobs()

	// cancel each job
	for _, token := range jobs {
		token.cancelFlag.Set(Canceled)
	}
}

// ShutdownAll cancels all the running jobs.
func (p *pool) ShutDownAll() {
	// remove jobs from task and save them to a local variable
	jobs := p.jobStore.DeleteAllJobs()

	// cancel each job
	for _, token := range jobs {
		token.cancelFlag.Set(ShutDown)
	}
}
