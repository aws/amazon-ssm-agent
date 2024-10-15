// Copyright 2023 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
//Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package verificationmanagers is used to verify the agent packages
package verificationmanagers

// VerificationManager denotes the type of verification manager
type VerificationManager int

const (
	// Undefined denotes the verification manager as undefined
	Undefined VerificationManager = iota
	// Linux denotes the verification manager for Linux
	Linux
	// Darwin denotes the verification manager for MacOS
	Darwin
	// Windows denotes the verification manager for Windows
	Windows
	// Skip denotes that the verification manager can be skipped
	Skip
)

var verificationManagers = map[VerificationManager]IVerificationManager{}

func registerVerificationManager(managerType VerificationManager, manager IVerificationManager) {
	verificationManagers[managerType] = manager
}

// GetVerificationManager returns the verification manager based on platform
func GetVerificationManager(managerType VerificationManager) (IVerificationManager, bool) {
	manager, ok := verificationManagers[managerType]
	return manager, ok
}
