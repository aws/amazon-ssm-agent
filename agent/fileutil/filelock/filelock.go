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
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/fileutil"
)

const (
	writeVerifyMilliseconds = 1000
)

type FileLocker interface {
	Lock(lockPath string, ownerId string, timeoutSeconds int) (locked bool, err error)
	Unlock(lockPath string, ownerId string) (hadLock bool, err error)
}

// TODO: Write platform-specific implementations instead.
type fileLocker struct {
}

func NewFileLocker() FileLocker {
	return &fileLocker{}
}

func GetOwnerIdForProcess() string {
	pid := os.Getpid()
	gid := os.Getgid()
	return fmt.Sprintf("pid-%d-gid-%d", pid, gid)
}

func (fl *fileLocker) Lock(lockPath string, ownerId string, timeoutSeconds int) (locked bool, err error) {
	locked, err = LockFile(lockPath, ownerId, timeoutSeconds)
	return
}

func (fl *fileLocker) Unlock(lockPath string, ownerId string) (hadLock bool, err error) {
	hadLock, err = UnlockFile(lockPath, ownerId)
	return
}

func statLockFile(lockPath string) (exists bool, modificationTime time.Time, err error) {
	var fi os.FileInfo
	if fi, err = os.Stat(lockPath); err != nil {
		if os.IsNotExist(err) {
			return false, time.Time{}, nil
		}
		return true, time.Time{}, err
	}
	modificationTime = fi.ModTime()
	return true, modificationTime, nil
}

func writeLockFile(lockPath string, contents string) (created bool, err error) {
	var f *os.File

	// Using os.O_EXCL because on most systems it will be atomic operation.
	if f, err = os.OpenFile(lockPath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600); err != nil {
		if os.IsExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("Unable to open lock file for writing. %v", err)
	}
	defer f.Close()

	if _, err = f.WriteString(contents); err != nil {
		return false, fmt.Errorf("Error writing to lock file. %v", err)
	}

	return true, nil
}

func readLockFile(lockPath string) (contents string, err error) {
	if contents, err = fileutil.ReadAllText(lockPath); err != nil {
		return contents, fmt.Errorf("Error reading from lock file %v. %v", lockPath, err)
	}
	return strings.TrimSpace(contents), err
}

func isLockFileExist(lockPath string) (exist bool, err error) {
	var exists bool
	if exists, _, err = statLockFile(lockPath); err != nil {
		return false, err
	}
	return exists, nil
}

func expireIfLockFileTimeout(lockPath string, timeoutSeconds int) (err error) {
	var modificationTime time.Time
	var exists bool
	if exists, modificationTime, err = statLockFile(lockPath); err != nil {
		return err
	}

	if !exists {
		return nil
	}

	startTime := modificationTime.Add(writeVerifyMilliseconds * time.Millisecond)
	timeoutTime := startTime.Add(time.Second * time.Duration(timeoutSeconds))

	timeout := timeoutTime.Before(time.Now())
	// Existing lock timed out, just quickly delete lockfile.
	if timeout {
		os.Remove(lockPath)
	}

	return nil
}

func LockFile(lockPath string, ownerId string, timeoutSeconds int) (locked bool, err error) {
	// Sleep a little to desynchronize processes
	time.Sleep(time.Duration(rand.Int63n(1000000)) * time.Microsecond)

	// Check if lock can be expired.
	if err = expireIfLockFileTimeout(lockPath, timeoutSeconds); err != nil {
		err := fmt.Errorf("Error checking lock file for timeout: %v. %v", lockPath, err)
		return false, err
	}

	var exist bool
	if exist, err = isLockFileExist(lockPath); err != nil {
		err := fmt.Errorf("Error checking whether lock file exists: %v. %v", lockPath, err)
		return false, err
	}

	if exist {
		// Lock file exists, another process must have the lock.
		return false, nil
	}

	// Try to create lock file
	var created bool
	if created, err = writeLockFile(lockPath, ownerId); err != nil {
		err := fmt.Errorf("Error locking file: %v. %v", lockPath, err)
		return false, err
	}

	if !created {
		// Another process must have gotten the lock.
		return false, nil
	}

	// Wait a little
	time.Sleep(writeVerifyMilliseconds * time.Millisecond)

	var contentsRead string
	if contentsRead, err = readLockFile(lockPath); err != nil && contentsRead != ownerId {
		// Content doesn't match, another process must have gotten the lock.
		return false, nil
	}

	return true, nil
}

func UnlockFile(lockPath string, ownerId string) (hadLock bool, err error) {
	var contentsRead string
	if contentsRead, _ = readLockFile(lockPath); contentsRead != ownerId {
		// Content doesn't match, another process must have gotten the lock.
		return false, nil
	}

	if err = os.Remove(lockPath); err != nil {
		return false, err
	}

	return true, nil
}
