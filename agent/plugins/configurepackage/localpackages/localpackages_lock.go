// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package localpackages

import (
	"errors"
	"fmt"
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/fileutil/filelock"
)

const (
	lockTimeoutInSeconds = 30 * 60 // 30 minutes
)

// Prevent multiple actions for the same package at the same time
var lockPackageAction = &sync.Mutex{}
var mapPackageAction = make(map[string]string)

// lockPackage adds the package name to the list of packages currently being acted on in a threadsafe way
func lockPackage(filelocker filelock.FileLocker, lockPath string, packageArn string, action string) error {
	lockPackageAction.Lock()
	defer lockPackageAction.Unlock()

	if val, ok := mapPackageAction[packageArn]; ok {
		return errors.New(fmt.Sprintf(`Package "%v" is already in the process of action "%v"`, packageArn, val))
	}

	ownerId := filelock.GetOwnerIdForProcess()
	locked, err := filelocker.Lock(lockPath, ownerId, lockTimeoutInSeconds)
	if err != nil {
		return errors.New(fmt.Sprintf(`Error locking package "%v": "%v"`, packageArn, err))
	}

	if !locked {
		return errors.New(fmt.Sprintf(`Package "%v" is already in the process of other action`, packageArn))
	}

	mapPackageAction[packageArn] = action
	return nil
}

// unlockPackage removes the package name from the list of packages currently being acted on in a threadsafe way
func unlockPackage(filelocker filelock.FileLocker, lockPath string, packageArn string) error {
	lockPackageAction.Lock()
	defer lockPackageAction.Unlock()

	if _, ok := mapPackageAction[packageArn]; ok {
		delete(mapPackageAction, packageArn)
	}

	ownerId := filelock.GetOwnerIdForProcess()
	_, err := filelocker.Unlock(lockPath, ownerId)
	return err
}
