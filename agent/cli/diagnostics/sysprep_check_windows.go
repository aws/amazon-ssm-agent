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

//go:build windows
// +build windows

package diagnostics

import (
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/cli/diagnosticsutil"
	"golang.org/x/sys/windows/registry"
)

const (
	imageStateComplete = "IMAGE_STATE_COMPLETE"
	sysPrepPath        = `SOFTWARE\Microsoft\Windows\CurrentVersion\Setup\State`

	sysprepCheckStrName                = "Windows sysprep image state"
	sysprepCheckStrRegistryKeyNotFound = "Couldn't find registry key for ImageState"
	sysprepCheckStrRegistryError       = "Error while trying to obtain setupKey from registry: %s"
	sysprepCheckStrSuccess             = "Windows image state value is at desired value %s"
	sysprepCheckStrWrongState          = "Windows image state should be %s but it is %s"
)

type sysPrepQuery struct{}

func (q sysPrepQuery) GetName() string {
	return sysprepCheckStrName
}

func (sysPrepQuery) GetPriority() int {
	return 7
}

func (q sysPrepQuery) Execute() diagnosticsutil.DiagnosticOutput {
	setupKey, err := registry.OpenKey(registry.LOCAL_MACHINE, sysPrepPath, registry.QUERY_VALUE)

	// In Windows 2003 and below, the path for the setupKey does not exist, hence skipping it.
	if err == registry.ErrNotExist {
		return diagnosticsutil.DiagnosticOutput{
			Check:  q.GetName(),
			Status: diagnosticsutil.DiagnosticsStatusSkipped,
			Note:   fmt.Sprintf(sysprepCheckStrRegistryKeyNotFound),
		}
	} else if err != nil {
		return diagnosticsutil.DiagnosticOutput{
			Check:  q.GetName(),
			Status: diagnosticsutil.DiagnosticsStatusFailed,
			Note:   fmt.Sprintf(sysprepCheckStrRegistryError, err),
		}
	}

	defer setupKey.Close()

	windowsImageState, _, err := setupKey.GetStringValue("ImageState")

	if windowsImageState == imageStateComplete {
		return diagnosticsutil.DiagnosticOutput{
			Check:  q.GetName(),
			Status: diagnosticsutil.DiagnosticsStatusSuccess,
			Note:   fmt.Sprintf(sysprepCheckStrSuccess, imageStateComplete),
		}
	} else {
		return diagnosticsutil.DiagnosticOutput{
			Check:  q.GetName(),
			Status: diagnosticsutil.DiagnosticsStatusFailed,
			Note:   fmt.Sprintf(sysprepCheckStrWrongState, imageStateComplete, windowsImageState),
		}
	}
}

func init() {
	diagnosticsutil.RegisterDiagnosticQuery(sysPrepQuery{})
}
