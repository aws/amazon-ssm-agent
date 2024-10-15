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

	"github.com/aws/amazon-ssm-agent/agent/cli/cliutil"
	"github.com/aws/amazon-ssm-agent/agent/cli/diagnosticsutil"
)

const (
	onpremCheckStrName           = "Hybrid instance registration"
	onpremCheckStrNoRegistration = "Instance does not have hybrid registration"
	onpremCheckStrSuccess        = "Instance has hybrid registration with instance id %s in region %s"
)

type onpremCheckQuery struct{}

func (q onpremCheckQuery) GetName() string {
	return onpremCheckStrName
}

func (onpremCheckQuery) GetPriority() int {
	return 2
}

func (q onpremCheckQuery) Execute() diagnosticsutil.DiagnosticOutput {
	if !diagnosticsutil.IsOnPremRegistration() {
		return diagnosticsutil.DiagnosticOutput{
			Check:  q.GetName(),
			Status: diagnosticsutil.DiagnosticsStatusSkipped,
			Note:   onpremCheckStrNoRegistration,
		}
	}

	agentIdentity, _ := cliutil.GetAgentIdentity()
	instanceId, _ := agentIdentity.InstanceID()
	region, _ := agentIdentity.Region()
	return diagnosticsutil.DiagnosticOutput{
		Check:  q.GetName(),
		Status: diagnosticsutil.DiagnosticsStatusSuccess,
		Note:   fmt.Sprintf(onpremCheckStrSuccess, instanceId, region),
	}
}

func init() {
	diagnosticsutil.RegisterDiagnosticQuery(onpremCheckQuery{})
}
