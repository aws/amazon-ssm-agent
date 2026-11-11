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

// Package scheduler schedules State Manager association discovery.
package scheduler

import (
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/polling"
)

// Job runs association polls serially until stopped.
type Job = polling.Job

// CreateScheduler preserves initial discovery within 30 seconds, then spreads
// recurring polls across the configured interval with fresh jitter on each wait.
func CreateScheduler(log log.T, task func(), config appconfig.SsmCfg) (*Job, error) {
	return polling.Start(log, task, config.AssociationFrequencyMinutes, 30*time.Second, polling.Jitter{
		Early: time.Duration(config.PollingJitterEarlySeconds) * time.Second,
		Late:  time.Duration(config.PollingJitterLateSeconds) * time.Second,
	})
}

// Stop cancels future polls while allowing an active poll to finish.
func Stop(job *Job) {
	job.Stop()
}
