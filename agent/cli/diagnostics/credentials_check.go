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
	"context"
	"fmt"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/cli/cliutil"
	"github.com/aws/amazon-ssm-agent/agent/cli/diagnosticsutil"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/sts"
)

const (
	credentialsCheckStrName              = "AWS Credentials"
	credentialsCheckStrNoCreds           = "No credentials available"
	credentialsCheckStrSessionFailed     = "Failed to initialize aws session: %v"
	credentialsCheckStrSTSTimeout        = "STS call timed out"
	credentialsCheckStrEC2RoleError      = "EC2RoleRequestError: %s"
	credentialsCheckStrSTSFailure        = "Failed call sts endpoint: %v"
	credentialsCheckStrExpirationFailure = "Credentials are for %s but failed to get credentials expiration: %v"
	credentialsCheckStrSuccess           = "Credentials are for %s and will expire at %s"
)

type credentialsCheckQuery struct{}

func (q credentialsCheckQuery) GetName() string {
	return credentialsCheckStrName
}

func (credentialsCheckQuery) GetPriority() int {
	return 4
}

func (q credentialsCheckQuery) Execute() diagnosticsutil.DiagnosticOutput {
	agentIdentity, err := cliutil.GetAgentIdentity()

	if err != nil {
		return diagnosticsutil.DiagnosticOutput{
			Check:  q.GetName(),
			Status: diagnosticsutil.DiagnosticsStatusSkipped,
			Note:   credentialsCheckStrNoCreds,
		}
	}

	awsSession, err := diagnosticsutil.GetAwsSession(agentIdentity, "sts")
	if err != nil {
		return diagnosticsutil.DiagnosticOutput{
			Check:  q.GetName(),
			Status: diagnosticsutil.DiagnosticsStatusFailed,
			Note:   fmt.Sprintf(credentialsCheckStrSessionFailed, err),
		}
	}

	client := sts.New(awsSession)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	callerResp, err := client.GetCallerIdentityWithContext(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		awsErr, isAwsError := err.(awserr.Error)
		if isAwsError && awsErr.Code() == request.CanceledErrorCode {
			return diagnosticsutil.DiagnosticOutput{
				Check:  q.GetName(),
				Status: diagnosticsutil.DiagnosticsStatusFailed,
				Note:   credentialsCheckStrSTSTimeout,
			}
		} else if isAwsError && awsErr.Code() == "EC2RoleRequestError" {
			return diagnosticsutil.DiagnosticOutput{
				Check:  q.GetName(),
				Status: diagnosticsutil.DiagnosticsStatusFailed,
				Note:   fmt.Sprintf(credentialsCheckStrEC2RoleError, awsErr.Message()),
			}
		}

		return diagnosticsutil.DiagnosticOutput{
			Check:  q.GetName(),
			Status: diagnosticsutil.DiagnosticsStatusFailed,
			Note:   fmt.Sprintf(credentialsCheckStrSTSFailure, err),
		}
	}

	expireDate, err := agentIdentity.Credentials().ExpiresAt()
	if err != nil {
		return diagnosticsutil.DiagnosticOutput{
			Check:  q.GetName(),
			Status: diagnosticsutil.DiagnosticsStatusFailed,
			Note:   fmt.Sprintf(credentialsCheckStrExpirationFailure, *callerResp.Arn, err),
		}
	}

	return diagnosticsutil.DiagnosticOutput{
		Check:  q.GetName(),
		Status: diagnosticsutil.DiagnosticsStatusSuccess,
		Note:   fmt.Sprintf(credentialsCheckStrSuccess, *callerResp.Arn, expireDate),
	}
}

func init() {
	diagnosticsutil.RegisterDiagnosticQuery(credentialsCheckQuery{})
}
