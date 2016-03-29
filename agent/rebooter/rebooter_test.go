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

package rebooter

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRequestPendingReboot(t *testing.T) {
	var expected, actual int
	var wg sync.WaitGroup

	// Random number
	expected = rand.Intn(100)

	// Spawn goroutines to Request Pending Reboot
	for i := 0; i < expected; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			RequestPendingReboot()
		}()
	}
	wg.Wait()

	// Wait a second to allow some ops to accumulate.
	time.Sleep(time.Second)
	actual = int(RebootRequestCount())

	assert.Equal(t, expected, actual, "The RebootRequestedCount is not the same as the RequestPendingReboot count.")
}

func TestRebootRequested(t *testing.T) {
	var expected, actual bool

	expected = true

	RequestPendingReboot()

	actual = RebootRequested()

	assert.Equal(t, expected, actual)
}
