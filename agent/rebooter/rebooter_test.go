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

package rebooter

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
)

func fakeWatchForReboot(log log.T) {
	ch = GetChannel()
	val := <-ch
	if val == RebootRequestTypeReboot {
		log.Info("start rebooting the machine...")
	} else {
		log.Error("reboot type not supported yet")
	}
}

func TestRequestPendingReboot(t *testing.T) {
	var successCount int = 0
	var wg sync.WaitGroup
	var logger = log.NewMockLog()
	go fakeWatchForReboot(logger)
	// Random number
	total := rand.Intn(10)

	// Spawn goroutines to Request Pending Reboot
	for i := 0; i < total; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if RequestPendingReboot(logger) {
				successCount++
			}
		}()
	}
	wg.Wait()

	// Wait a second to allow some ops to accumulate.
	time.Sleep(time.Second)
	assert.Equal(t, successCount, 1, "Request reboot should only return true once")
}
