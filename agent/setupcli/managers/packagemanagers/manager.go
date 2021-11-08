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

package packagemanagers

import (
	"sort"
)

// package manager selection priority is based on order in list below
type PackageManager int

const (
	Undefined PackageManager = iota
	Snap
	Dpkg
	Rpm
)

var packageManagers = map[PackageManager]IPackageManager{}

func registerPackageManager(managerType PackageManager, manager IPackageManager) {
	packageManagers[managerType] = manager
}

// GetAllPackageManagers returns all package managers in priority order
func GetAllPackageManagers() []IPackageManager {
	supportedPackageManagersTypes := make([]PackageManager, 0, len(packageManagers))
	for managerType, _ := range packageManagers {
		supportedPackageManagersTypes = append(supportedPackageManagersTypes, managerType)
	}

	sort.SliceStable(supportedPackageManagersTypes, func(i, j int) bool {
		return supportedPackageManagersTypes[i] < supportedPackageManagersTypes[j]
	})

	orderedPackageManagers := make([]IPackageManager, 0, len(packageManagers))
	for _, managerType := range supportedPackageManagersTypes {
		orderedPackageManagers = append(orderedPackageManagers, packageManagers[managerType])
	}

	return orderedPackageManagers
}

// GetPackageManager returns a specific package manager of a specific package manager type
func GetPackageManager(managerType PackageManager) (IPackageManager, bool) {
	manager, ok := packageManagers[managerType]
	return manager, ok
}
