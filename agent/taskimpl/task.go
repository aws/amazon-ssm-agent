// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Amazon Software License (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/asl/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package taskimpl

import (
	"fmt"
	"sync"
)

// Task is a collection of jobs.
type Task struct {
	jobs map[string]*JobToken
	m    sync.RWMutex
}

// NewTask creates a new task with no jobs.
func NewTask() *Task {
	return &Task{
		jobs: make(map[string]*JobToken),
	}
}

// AddJob adds a job to this task.
// Returns error if the job already exists.
func (t *Task) AddJob(jobID string, token *JobToken) error {
	t.m.Lock()
	defer t.m.Unlock()

	_, found := t.jobs[jobID]
	if found {
		return fmt.Errorf("Job with id %v already exists", jobID)
	}

	t.jobs[jobID] = token
	return nil
}

// GetJob retrieves a job from a task.
func (t *Task) GetJob(jobID string) (token *JobToken, found bool) {
	t.m.RLock()
	defer t.m.RUnlock()
	s, ok := t.jobs[jobID]
	return s, ok
}

// DeleteJob deletes the job with the given jobID.
func (t *Task) DeleteJob(jobID string) {
	t.m.Lock()
	defer t.m.Unlock()
	delete(t.jobs, jobID)
}

// DeleteAllJobs deletes all the jobs of this task.
// Returns the deleted jobs.
func (t *Task) DeleteAllJobs() map[string]*JobToken {
	t.m.Lock()
	defer t.m.Unlock()
	jobs := t.jobs
	t.jobs = map[string]*JobToken{}
	return jobs
}
