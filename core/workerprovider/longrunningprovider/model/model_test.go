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

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWorkerConfig(t *testing.T) {
	config := WorkerConfig{
		Name:       "test",
		BinaryName: "test.exe",
		Path:       "filepath",
		Args:       []string{},
	}

	assert.Equal(t, config.Name, "test")
	assert.Equal(t, config.BinaryName, "test.exe")
	assert.Equal(t, config.Path, "filepath")
}

func TestWorker(t *testing.T) {
	worker := Worker{
		Name: "test",
		Processes: map[int]*Process{
			1: &Process{1, Active},
		},
	}

	assert.Equal(t, worker.Name, "test")
	assert.Equal(t, worker.Processes[1].Pid, 1)
	assert.Equal(t, worker.Processes[1].Status, Active)
}
