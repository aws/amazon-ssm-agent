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

//go:build integration || fastinteg
// +build integration fastinteg

// Package scheduler provides ability to create scheduled job
package scheduler

import (
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/mocks/context"
	"github.com/stretchr/testify/require"
)

func TestCreatingAndStoppingScheduler(t *testing.T) {
	context := context.NewMockDefault()
	job, err := CreateScheduler(context.Log(), func() {}, 10)
	require.NoError(t, err)
	t.Cleanup(func() { Stop(job) })
	Stop(job)
	select {
	case <-job.done:
	case <-time.After(time.Second):
		t.Fatal("scheduler did not stop during its delay")
	}
}
