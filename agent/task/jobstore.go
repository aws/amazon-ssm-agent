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
)

// JobStore is a collection of jobs.
type JobStore struct {
	jobs map[string]*JobToken
	m    sync.RWMutex
}

// NewJobStore creates a new task with no jobs.
func NewJobStore() *JobStore {
	return &JobStore{
		jobs: make(map[string]*JobToken),
	}
}

// AddJob adds a job to this task.
// Returns error if the job already exists.
func (t *JobStore) AddJob(jobID string, token *JobToken) error {
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
func (t *JobStore) GetJob(jobID string) (token *JobToken, found bool) {
	t.m.RLock()
	defer t.m.RUnlock()
	s, ok := t.jobs[jobID]
	return s, ok
}

// DeleteJob deletes the job with the given jobID.
func (t *JobStore) DeleteJob(jobID string) {
	t.m.Lock()
	defer t.m.Unlock()
	delete(t.jobs, jobID)
}

// DeleteAllJobs deletes all the jobs of this task.
// Returns the deleted jobs.
func (t *JobStore) DeleteAllJobs() map[string]*JobToken {
	t.m.Lock()
	defer t.m.Unlock()
	jobs := t.jobs
	t.jobs = map[string]*JobToken{}
	return jobs
}
