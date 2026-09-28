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

package sdkutil

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/smithy-go"
	"github.com/stretchr/testify/assert"
)

var (
	errSample = errors.New("some error")
)

func TestHandleAwsErrorCount(t *testing.T) {
	errorCount := 10
	stopPolicy := NewStopPolicy("test", errorCount)

	log := log.NewMockLog()

	for i := 0; i < errorCount-1; i++ {
		HandleAwsError(log, errSample, stopPolicy)
		if !stopPolicy.IsHealthy() {
			assert.Fail(t, "stoppolicy should be healthy")
		}
	}

	HandleAwsError(log, errSample, stopPolicy)
	if stopPolicy.IsHealthy() {
		assert.Fail(t, "stoppolicy should not be healthy")
	}

	HandleAwsError(log, nil, stopPolicy)
	if !stopPolicy.IsHealthy() {
		assert.Fail(t, "stoppolicy should have reset and be heallthy")
	}

}

func TestHandleAwsError_ContextDeadlineExceeded_ResetsStopPolicy(t *testing.T) {
	stopPolicy := NewStopPolicy("test", 10)
	stopPolicy.AddErrorCount(5) // simulate some prior errors
	log := log.NewMockLog()

	HandleAwsError(log, context.DeadlineExceeded, stopPolicy)

	assert.True(t, stopPolicy.IsHealthy(),
		"context.DeadlineExceeded should reset stop policy, not increment error count")
}

func TestHandleAwsError_NetTimeout_ResetsStopPolicy(t *testing.T) {
	stopPolicy := NewStopPolicy("test", 10)
	stopPolicy.AddErrorCount(5)
	log := log.NewMockLog()

	HandleAwsError(log, &netTimeoutError{}, stopPolicy)

	assert.True(t, stopPolicy.IsHealthy(),
		"net timeout errors should reset stop policy, not increment error count")
}

func TestHandleAwsError_ClientTimeout_ResetsStopPolicy(t *testing.T) {
	stopPolicy := NewStopPolicy("test", 10)
	stopPolicy.AddErrorCount(5)
	log := log.NewMockLog()

	err := fmt.Errorf("net/http: request canceled (Client.Timeout exceeded while awaiting headers)")
	HandleAwsError(log, err, stopPolicy)

	assert.True(t, stopPolicy.IsHealthy(),
		"Client.Timeout string errors should reset stop policy")
}

func TestHandleAwsError_Ec2messagesAccessDenied_PlainError_ResetsStopPolicy(t *testing.T) {
	stopPolicy := NewStopPolicy("test", 10)
	stopPolicy.AddErrorCount(5)
	log := log.NewMockLog()

	// MDS client returns plain fmt.Errorf, not wrapped in smithy.APIError
	err := fmt.Errorf("ec2messages: AccessDeniedException: User is not authorized")
	HandleAwsError(log, err, stopPolicy)

	assert.True(t, stopPolicy.IsHealthy(),
		"plain ec2messages AccessDenied errors should reset stop policy")
}

func TestHandleAwsError_SmithyExpiredToken_ResetsStopPolicy(t *testing.T) {
	stopPolicy := NewStopPolicy("test", 10)
	stopPolicy.AddErrorCount(5) // simulate prior errors
	log := log.NewMockLog()

	err := &smithy.OperationError{
		ServiceID:     "SSM",
		OperationName: "UpdateInstanceInformation",
		Err:           &smithy.GenericAPIError{Code: "ExpiredTokenException", Message: "token expired"},
	}
	HandleAwsError(log, err, stopPolicy)

	assert.True(t, stopPolicy.IsHealthy(),
		"ExpiredTokenException should reset stop policy to allow credential refresh, not max it out")
}

func TestGetAwsError_SmithyAPIError(t *testing.T) {
	apiErr := &smithy.GenericAPIError{Code: "AccessDeniedException", Message: "not authorized"}
	err := &smithy.OperationError{
		ServiceID:     "SSM",
		OperationName: "UpdateInstanceInformation",
		Err:           apiErr,
	}

	result := GetAwsError(err)
	assert.NotNil(t, result, "GetAwsError should extract from smithy.APIError")
	assert.Equal(t, "AccessDeniedException", result.Code())
	assert.Equal(t, "not authorized", result.Message())
}

func TestGetAwsError_NonSmithyError_ReturnsNil(t *testing.T) {
	err := fmt.Errorf("some random error")
	result := GetAwsError(err)
	assert.Nil(t, result, "GetAwsError should return nil for non-SDK errors")
}

// netTimeoutError implements net.Error with Timeout() = true
type netTimeoutError struct{}

func (e *netTimeoutError) Error() string   { return "i/o timeout" }
func (e *netTimeoutError) Timeout() bool   { return true }
func (e *netTimeoutError) Temporary() bool { return true }

// V2 compat: GetAwsErrorCode should map the bare "no EC2 IMDS role found"
// error surfaced by aws-sdk-go-v2/credentials/ec2rolecreds when the instance
// profile is detached, back to the v1 code EC2RoleRequestError. This is what
// EC2RoleProvider.RemoteRetrieve keys off of when deciding whether to fall
// through to the Default Host Management (SSM) credential path.
func TestGetAwsErrorCode_ImdsRoleNotFound_MapsToEC2RoleRequestError(t *testing.T) {
	// Shape produced by aws-sdk-go-v2 ec2rolecreds Provider.Retrieve when the
	// instance profile has been detached (IMDS returns 404). The error is a
	// bare fmt.Errorf, not a smithy.APIError, and it typically ends up wrapped
	// again by the SSM client's signer step ("get identity: get credentials:
	// failed to retrieve instance profile role credentials. Err: ..."). Both
	// the direct and wrapped forms must map to EC2RoleRequestError.
	direct := fmt.Errorf("no EC2 IMDS role found, operation error ec2imds: GetMetadata, http response error StatusCode: 404, request to EC2 IMDS failed")
	assert.Equal(t, "EC2RoleRequestError", GetAwsErrorCode(direct))

	wrapped := fmt.Errorf("retrieved credentials failed to report to ssm. Error: %w", direct)
	assert.Equal(t, "EC2RoleRequestError", GetAwsErrorCode(wrapped))

	// The full nested shape seen in the live agent log — SSM OperationError
	// wrapping a signer error wrapping the ec2rolecreds error.
	nested := fmt.Errorf("operation error SSM: UpdateInstanceInformation, get identity: get credentials: failed to retrieve instance profile role credentials. Err: %w", direct)
	assert.Equal(t, "EC2RoleRequestError", GetAwsErrorCode(nested))
}

// Unrelated errors must not be misclassified as EC2RoleRequestError.
func TestGetAwsErrorCode_UnrelatedError_ReturnsEmpty(t *testing.T) {
	assert.Equal(t, "", GetAwsErrorCode(fmt.Errorf("some random unrelated error")))
	assert.Equal(t, "", GetAwsErrorCode(nil))
}
