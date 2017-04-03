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

// Package configurepackage implements the ConfigurePackage plugin.
package configurepackage

import (
	"errors"
	"fmt"
	"sync"
)

// Prevent multiple actions for the same package at the same time
var lockPackageAction = &sync.Mutex{}
var mapPackageAction = make(map[string]string)

// lockPackage adds the package name to the list of packages currently being acted on in a threadsafe way
func lockPackage(packageName string, action string) error {
	lockPackageAction.Lock()
	defer lockPackageAction.Unlock()
	if val, ok := mapPackageAction[packageName]; ok {
		return errors.New(fmt.Sprintf(`Package "%v" is already in the process of action "%v"`, packageName, val))
	}
	mapPackageAction[packageName] = action

	return nil
}

// unlockPackage removes the package name from the list of packages currently being acted on in a threadsafe way
func unlockPackage(packageName string) {
	lockPackageAction.Lock()
	defer lockPackageAction.Unlock()
	if _, ok := mapPackageAction[packageName]; ok {
		delete(mapPackageAction, packageName)
	}
}
