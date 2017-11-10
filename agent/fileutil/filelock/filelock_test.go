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

package filelock

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSerialLocking(t *testing.T) {
	lockPath := "lock-file.TestSerialLocking.tmp"
	ownerId1 := "1001"
	ownerId2 := "1002"
	timeoutSeconds := 1

	var err error
	var locked bool
	var hadLock bool

	// First, clean up
	os.Remove(lockPath)

	// First lock
	locked, err = LockFile(lockPath, ownerId1, timeoutSeconds)
	assert.NoError(t, err)
	assert.Equal(t, true, locked)

	hadLock, err = UnlockFile(lockPath, ownerId1)
	assert.NoError(t, err)
	assert.Equal(t, true, hadLock)

	// Second lock, same owner
	locked, err = LockFile(lockPath, ownerId1, timeoutSeconds)
	assert.NoError(t, err)
	assert.Equal(t, true, locked)

	hadLock, err = UnlockFile(lockPath, ownerId1)
	assert.NoError(t, err)
	assert.Equal(t, true, hadLock)

	// Third lock, different owner
	locked, err = LockFile(lockPath, ownerId2, timeoutSeconds)
	assert.NoError(t, err)
	assert.Equal(t, true, locked)

	hadLock, err = UnlockFile(lockPath, ownerId2)
	assert.NoError(t, err)
	assert.Equal(t, true, hadLock)

	// Last, clean up again
	os.Remove(lockPath)
}

func TestInterlappingLocking(t *testing.T) {
	lockPath := "lock-file.TestInterlappingLocking.tmp"
	ownerId1 := "1001"
	ownerId2 := "1002"
	timeoutSeconds := 1

	var err error
	var locked bool
	var hadLock bool

	// First, clean up
	os.Remove(lockPath)

	// First lock
	locked, err = LockFile(lockPath, ownerId1, timeoutSeconds)
	assert.NoError(t, err)
	assert.Equal(t, true, locked)

	// Second lock, different owner - should fail
	locked, err = LockFile(lockPath, ownerId2, timeoutSeconds)
	assert.NoError(t, err)
	assert.Equal(t, false, locked)

	hadLock, err = UnlockFile(lockPath, ownerId1)
	assert.NoError(t, err)
	assert.Equal(t, true, hadLock)

	// Last, clean up again
	os.Remove(lockPath)
}

func TestLockingTimeout(t *testing.T) {
	lockPath := "lock-file.TestInterlappingLocking.tmp"
	ownerId1 := "1001"
	ownerId2 := "1002"
	timeoutSeconds := 1

	var err error
	var locked bool
	var hadLock bool

	// First, clean up
	os.Remove(lockPath)

	// First lock
	locked, err = LockFile(lockPath, ownerId1, timeoutSeconds)
	assert.NoError(t, err)
	assert.Equal(t, true, locked)

	time.Sleep(2000 * time.Millisecond)

	// Second lock, different owner - should succeed, since lock is timed out.
	locked, err = LockFile(lockPath, ownerId2, timeoutSeconds)
	assert.NoError(t, err)
	assert.Equal(t, true, locked)

	// First unlock, should fail, since lock timed out.
	hadLock, err = UnlockFile(lockPath, ownerId1)
	assert.NoError(t, err)
	assert.Equal(t, false, hadLock)

	hadLock, err = UnlockFile(lockPath, ownerId2)
	assert.NoError(t, err)
	assert.Equal(t, true, hadLock)

	// Last, clean up again
	os.Remove(lockPath)
}

func TestParallelLocking(t *testing.T) {
	lockPath := "lock-file.TestParallelLocking.tmp"
	timeoutSeconds := 100

	// First, clean up
	os.Remove(lockPath)

	N := 1000
	var wg sync.WaitGroup
	wg.Add(N)

	var lockCountAtomic int32 = 0

	for i := 0; i < N; i++ {
		go func(id int) {
			defer wg.Done()
			ownerId := fmt.Sprintf("ownerId-%d", id)

			// Only one thread should get the lock.
			locked, err := LockFile(lockPath, ownerId, timeoutSeconds)
			assert.NoError(t, err)

			if locked {
				atomic.AddInt32(&lockCountAtomic, 1)
			}

			fmt.Printf("gorouting completed %v\n", ownerId)
		}(i)
	}

	wg.Wait()
	lockCount := int(atomic.LoadInt32(&lockCountAtomic))
	assert.Equal(t, 1, lockCount)

	// Last, clean up again
	os.Remove(lockPath)
}
