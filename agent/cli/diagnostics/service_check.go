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

//go:build freebsd || linux || netbsd || openbsd || darwin || windows
// +build freebsd linux netbsd openbsd darwin windows

package diagnostics

import (
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/cli/diagnosticsutil"
)

const (
	serviceCheckStrName           = "Agent service"
	serviceCheckStrFailedGetUser  = "Agent service is running but failed to get user: %v"
	serviceCheckStrUnexpectedUser = "Agent service is running but agent is running as %s instead of expected user %s"
	serviceCheckStrSuccess        = "Agent service is running and is running as expected user"
)

type serviceCheckQuery struct{}

func (q serviceCheckQuery) GetName() string {
	return serviceCheckStrName
}

func (serviceCheckQuery) GetPriority() int {
	return 5
}

func (q serviceCheckQuery) Execute() diagnosticsutil.DiagnosticOutput {
	err := isServiceRunning()

	if err != nil {
		return diagnosticsutil.DiagnosticOutput{
			Check:  q.GetName(),
			Status: diagnosticsutil.DiagnosticsStatusFailed,
			Note:   err.Error(),
		}
	}

	user, err := diagnosticsutil.GetUserRunningAgentProcess()
	if err != nil {
		return diagnosticsutil.DiagnosticOutput{
			Check:  q.GetName(),
			Status: diagnosticsutil.DiagnosticsStatusFailed,
			Note:   fmt.Sprintf(serviceCheckStrFailedGetUser, err),
		}
	}

	if user != diagnosticsutil.ExpectedServiceRunningUser {
		return diagnosticsutil.DiagnosticOutput{
			Check:  q.GetName(),
			Status: diagnosticsutil.DiagnosticsStatusFailed,
			Note:   fmt.Sprintf(serviceCheckStrUnexpectedUser, user, diagnosticsutil.ExpectedServiceRunningUser),
		}
	}

	return diagnosticsutil.DiagnosticOutput{
		Check:  q.GetName(),
		Status: diagnosticsutil.DiagnosticsStatusSuccess,
		Note:   serviceCheckStrSuccess,
	}
}

func init() {
	diagnosticsutil.RegisterDiagnosticQuery(serviceCheckQuery{})
}
