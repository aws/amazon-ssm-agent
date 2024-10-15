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

//go:build windows
// +build windows

// Package process wraps up the os.Process interface and also provides os-specific process lookup functions
package proc

import (
	"os/exec"
	"time"
)

var (
	//https://msdn.microsoft.com/en-us/library/windows/desktop/ms724290(v=vs.85).aspx
	windowsBaseTime = time.Date(1601, 1, 1, 0, 0, 0, 0, time.UTC)
)

func prepareProcess(command *exec.Cmd) {
	// nothing to do on windows
}
