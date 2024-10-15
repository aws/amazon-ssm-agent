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

package diagnostics

import (
	"fmt"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/cli/diagnosticsutil"
	"github.com/aws/amazon-ssm-agent/agent/proxyconfig"
)

const (
	proxyCheckStrName     = "Proxy configuration"
	proxyCheckStrHasProxy = "Detected proxy configuration: %s"
	proxyCheckStrNoProxy  = "No proxy configuration detected"
)

type proxyCheckQuery struct{}

func (q proxyCheckQuery) GetName() string {
	return proxyCheckStrName
}

func (proxyCheckQuery) GetPriority() int {
	return 6
}

func (q proxyCheckQuery) Execute() diagnosticsutil.DiagnosticOutput {
	proxyMap := proxyconfig.GetProxyConfig()

	isProxyConfigured := false
	var builder strings.Builder
	for proxyVar, proxyVal := range proxyMap {
		builder.WriteString(fmt.Sprintf("%s: ", proxyVar))
		if len(proxyVal) > 0 {
			isProxyConfigured = true
			builder.WriteString(fmt.Sprintf("%s ", proxyVal))
		} else {
			builder.WriteString("NULL ")
		}
	}

	if isProxyConfigured {
		return diagnosticsutil.DiagnosticOutput{
			Check:  q.GetName(),
			Status: diagnosticsutil.DiagnosticsStatusSuccess,
			Note:   fmt.Sprintf(proxyCheckStrHasProxy, builder.String()),
		}
	} else {
		return diagnosticsutil.DiagnosticOutput{
			Check:  q.GetName(),
			Status: diagnosticsutil.DiagnosticsStatusSkipped,
			Note:   proxyCheckStrNoProxy,
		}
	}
}

func init() {
	diagnosticsutil.RegisterDiagnosticQuery(proxyCheckQuery{})
}
