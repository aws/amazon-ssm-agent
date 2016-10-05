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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTask(t *testing.T) {
	nJobs := 5

	var jobs = make(map[string]*JobToken)
	for i := 0; i < nJobs; i++ {
		jobID := fmt.Sprintf("job%d", i)
		token := &JobToken{id: jobID}
		jobs[jobID] = token
	}

	tsk := testAddAndGet(t, jobs)

	for jobID := range jobs {
		// test delete job
		tsk.DeleteJob(jobID)
		j, found := tsk.GetJob(jobID)
		assert.False(t, found)
		assert.Nil(t, j)
	}

	tsk = testAddAndGet(t, jobs)

	// test delete all jobs
	assert.Equal(t, jobs, tsk.DeleteAllJobs())
	for jobID := range jobs {
		// test job is missing
		j, found := tsk.GetJob(jobID)
		assert.False(t, found)
		assert.Nil(t, j)
	}
}

func testAddAndGet(t *testing.T, jobs map[string]*JobToken) (tsk *JobStore) {
	tsk = NewJobStore()
	for jobID, token := range jobs {
		// test get missing job
		j, found := tsk.GetJob(jobID)
		assert.False(t, found)
		assert.Nil(t, j)

		// test add job
		err := tsk.AddJob(jobID, token)
		assert.Nil(t, err)

		// test get existing job
		j, found = tsk.GetJob(jobID)
		assert.True(t, found)
		assert.Equal(t, token, j)

		// test add existing job
		token2 := &JobToken{id: jobID}
		err = tsk.AddJob(jobID, token2)
		assert.NotNil(t, err)
	}
	return
}
