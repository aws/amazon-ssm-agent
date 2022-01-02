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
	"strings"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/times"
)

type PoolErrorCode string

var (
	// DuplicateCommand represents duplicate command in the buffer
	DuplicateCommand PoolErrorCode = "DuplicateCommand"

	// InvalidJobId represents invalid job Id
	InvalidJobId PoolErrorCode = "InvalidJobId"

	// UninitializedBuffer represents that the buffer has not been initialized in the pool
	UninitializedBuffer PoolErrorCode = "UninitializedBuffer"

	// JobQueueFull represents that the job queue buffer is full
	JobQueueFull PoolErrorCode = "JobQueueFull"
)

const (
	// unusedTokenValidityInMinutes denotes the unused token validity in minutes
	unusedTokenValidityInMinutes = 10
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

	// BufferTokensIssued returns the current buffer token size
	BufferTokensIssued() int

	// AcquireBufferToken acquires the buffer token based on job id
	AcquireBufferToken(jobId string) PoolErrorCode

	// ReleaseBufferToken releases the acquired token
	ReleaseBufferToken(jobId string) PoolErrorCode
}

// pool implements a task pool where all jobs are managed by a root task
type pool struct {
	log                log.T
	jobQueue           chan JobToken
	maxWorkers         int
	doneWorker         chan struct{}
	jobHandlerDone     chan struct{}
	isShutdown         bool
	bufferLimit        int
	tokenHoldingJobIds map[string]*time.Time
	clock              times.Clock
	mut                sync.RWMutex
	jobStore           *JobStore
	cancelDuration     time.Duration
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
func NewPool(log log.T, maxParallel int, bufferLimit int, cancelWaitDuration time.Duration, clock times.Clock) Pool {
	p := &pool{
		log:                log,
		jobQueue:           make(chan JobToken, bufferLimit),
		maxWorkers:         maxParallel,
		doneWorker:         make(chan struct{}),
		jobHandlerDone:     make(chan struct{}),
		clock:              clock,
		bufferLimit:        bufferLimit,
		cancelDuration:     cancelWaitDuration,
		tokenHoldingJobIds: make(map[string]*time.Time),
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
			p.ReleaseBufferToken(job.id)
			p.log.Infof("Got job %s, starting worker", job.id)
			workerCount++
			go func() {
				defer p.workerDone()
				if !job.cancelFlag.Canceled() && !job.cancelFlag.ShutDown() {
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

// BufferTokensIssued returns the current buffer token size
func (p *pool) BufferTokensIssued() int {
	p.mut.RLock()
	defer p.mut.RUnlock()
	return len(p.tokenHoldingJobIds)
}

// AcquireBufferToken acquires the buffer token based on job id
func (p *pool) AcquireBufferToken(jobId string) PoolErrorCode {
	p.mut.Lock()
	defer p.mut.Unlock()
	// no buffer case
	if p.bufferLimit == 0 {
		return UninitializedBuffer
	}
	// job already in the job store
	if p.HasJob(jobId) {
		return DuplicateCommand
	}
	// token already acquired for the job
	if _, ok := p.tokenHoldingJobIds[jobId]; ok {
		return DuplicateCommand
	}
	// empty job id
	if strings.TrimSpace(jobId) == "" {
		return InvalidJobId
	}
	currentTime := time.Now()
	// buffer length validation
	if len(p.tokenHoldingJobIds) >= p.bufferLimit {
		// removing expired tokens only when the job queue is full
		expirationTime := currentTime.Add(time.Duration(-unusedTokenValidityInMinutes) * time.Minute)
		for tokenId, tokenTime := range p.tokenHoldingJobIds {
			// hasJob condition added to handle long-running commands
			if tokenTime.Before(expirationTime) && !p.HasJob(tokenId) {
				p.log.Warnf("removing expired token %v from the TokenBuffer", tokenId)
				delete(p.tokenHoldingJobIds, tokenId)
			}
		}
		// do the validation again. if fails, return JobQueueFull error
		if len(p.tokenHoldingJobIds) >= p.bufferLimit {
			return JobQueueFull
		}
	}
	p.tokenHoldingJobIds[jobId] = &currentTime
	return ""
}

// ReleaseBufferToken releases the acquired token
func (p *pool) ReleaseBufferToken(jobId string) PoolErrorCode {
	p.mut.Lock()
	defer p.mut.Unlock()
	// no buffer case
	if p.bufferLimit == 0 {
		return UninitializedBuffer
	}
	// empty job id
	if strings.TrimSpace(jobId) == "" {
		return InvalidJobId
	}
	delete(p.tokenHoldingJobIds, jobId)
	p.log.Debugf("buffer limit end values for command %v: tokenSize - %v", jobId, len(p.tokenHoldingJobIds))
	return ""
}

// workerDone signals that a worker has terminated.
func (p *pool) workerDone() {
	p.doneWorker <- struct{}{}
}

// Submit adds a job to the execution queue of this pool.
// NOTE: When adding new errors in this function, make sure that the token buffer deletion is fine in that case
// When this function throw error, we release token in submit/cancel in processor.go.
// This may lead to mismatch between issued buffer token and jobs in the buffer when done improperly
func (p *pool) Submit(log log.T, jobID string, job Job) (err error) {
	if p.checkIsShutDown() {
		p.log.Errorf("Attempting to add job %s to a closed queue", jobID)
		return nil // restart will pick this pending document
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

// ShutDownAll cancels all the running jobs.
func (p *pool) ShutDownAll() {
	// remove jobs from task and save them to a local variable
	jobs := p.jobStore.DeleteAllJobs()

	// cancel each job
	for _, token := range jobs {
		token.cancelFlag.Set(ShutDown)
	}
}
