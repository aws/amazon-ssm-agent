// Copyright 2021 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package log

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExeLogFileName_CoreAgent(t *testing.T) {
	paths := []string{
		"C:\\Program Data\\Amazon\\SSM\\amazon-ssm-agent",
		"C:\\Program Data\\Amazon\\SSM\\amazon-ssm-agent.exe",
	}

	for _, path := range paths {
		getExePath = func() string {
			return path
		}
		assert.Equal(t, "amazon-ssm-agent", exeLogFileName())
	}
}

func TestExeLogFileName_AgentWorker(t *testing.T) {
	paths := []string{
		"C:\\Program Data\\Amazon\\SSM\\ssm-agent-worker",
		"C:\\Program Data\\Amazon\\SSM\\ssm-agent-worker.exe",
	}

	for _, path := range paths {
		getExePath = func() string {
			return path
		}
		assert.Equal(t, "ssm-agent-worker", exeLogFileName())
	}
}

func TestExeLogFileName_DocumentWorker(t *testing.T) {
	paths := []string{
		"C:\\Program Data\\Amazon\\SSM\\ssm-document-worker",
		"C:\\Program Data\\Amazon\\SSM\\ssm-document-worker.exe",
	}

	for _, path := range paths {
		getExePath = func() string {
			return path
		}
		assert.Equal(t, "ssm-document-worker", exeLogFileName())
	}
}
