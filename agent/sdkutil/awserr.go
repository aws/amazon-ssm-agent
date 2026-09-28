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

// Package sdkutil provides utilities used to call awssdk.
package sdkutil

import (
	"context"
	"errors"
	"fmt"
	"net"
	"runtime"
	"strings"

	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	"github.com/aws/amazon-ssm-agent/agent/log"
)

// awserr represents an AWS error with code, message, and original error
type awserr struct {
	code    string
	message string
	origErr error
}

type Error interface {
	// Satisfy the generic error interface.
	error

	// Returns the short phrase depicting the classification of the error.
	Code() string

	// Returns the error details message.
	Message() string

	// Returns the original error if one was set.  Nil is returned if not set.
	OrigErr() error
}

// New creates a new awserr with the given code, message, and original error
func New(code, message string, origErr error) *awserr {
	return &awserr{
		code:    code,
		message: message,
		origErr: origErr,
	}
}

// Error returns the string representation of the error
func (e *awserr) Error() string {
	if e.origErr != nil {
		return fmt.Sprintf("%s: %s\ncaused by: %s", e.code, e.message, e.origErr.Error())
	}
	return fmt.Sprintf("%s: %s", e.code, e.message)
}

// Code returns the error code
func (e *awserr) Code() string {
	return e.code
}

// Message returns the error message
func (e *awserr) Message() string {
	return e.message
}

// OrigErr returns the original error
func (e *awserr) OrigErr() error {
	return e.origErr
}

type RequestFailure interface {
	Error

	// The status code of the HTTP response.
	StatusCode() int

	// The request ID returned by the service for a request failure. This will
	// be empty if no request ID is available such as the request failed due
	// to a connection error.
	RequestID() string
}

// HandleAwsError logs an AWS error.
func HandleAwsError(log log.T, err error, stopPolicy *StopPolicy) {
	if err != nil {
		// notice that we're using 1, so it will actually log the where
		// the error happened, 0 = this function, we don't want that.
		pc, fn, line, _ := runtime.Caller(1)
		log.Debugf("error in %s[%s:%d] %v", runtime.FuncForPC(pc).Name(), fn, line, err)

		// Detect transport-level timeouts (context.DeadlineExceeded, net timeouts)
		// before checking smithy.APIError — these never get wrapped as API errors.
		if errors.Is(err, context.DeadlineExceeded) || isNetTimeout(err) || strings.Contains(err.Error(), "Client.Timeout") {
			log.Debugf("Timeout error detected, resetting stop policy: %v", err)
			if stopPolicy != nil {
				resetStopPolicy(stopPolicy)
			}
			return
		}

		// Detect ec2messages AccessDenied from plain error strings (MDS client uses
		// fmt.Errorf, not smithy errors) — the ec2messages endpoint is deprecated.
		if strings.Contains(err.Error(), "ec2messages") && strings.Contains(err.Error(), "AccessDenied") {
			log.Debugf("ec2messages endpoint access denied. This is expected if ec2messages permissions are not configured. The ec2messages endpoint is deprecated; ssmmessages is the recommended endpoint.")
			if stopPolicy != nil {
				resetStopPolicy(stopPolicy)
			}
			return
		}

		// Detect smithy transport-level send errors (*smithyhttp.RequestSendError),
		// which wrap network/transport failures that occur before an API response
		// is received — e.g. connection failures during credential refresh. These
		// are never surfaced as smithy.APIError, so check for them first and treat
		// them as transient by resetting the stop policy.
		var opErr *smithy.OperationError
		var sendErr *smithyhttp.RequestSendError
		if errors.As(err, &sendErr) || (errors.As(err, &opErr) && isTransportError(opErr.Unwrap())) {
			log.Warnf("transport-level request send error detected, resetting stop policy: %v", err)
			if stopPolicy != nil {
				resetStopPolicy(stopPolicy)
			}
			return
		}

		// In case this is aws error, update the stop policy as well.
		// Use errors.As to unwrap *smithy.OperationError which wraps smithy.APIError.
		var aErr smithy.APIError
		if errors.As(err, &aErr) {
			// Generic AWS Error with Code, Message, and original error (if any)
			log.Debugf("AWS error. Code: %v, Message: %v, origerror: %v ", aErr.ErrorCode(), aErr.ErrorMessage(), aErr.Error())
			if aErr.ErrorCode() == "ExpiredTokenException" {
				// In v1, expired tokens were handled by the SDK's retry middleware
				// (auto-invalidate creds → retry with fresh). Treat as transient —
				// reset the stop policy so the next API call attempt can use refreshed
				// credentials. Do not max out the policy as that instantly trips health.
				log.Warnf("ExpiredTokenException encountered, resetting stop policy for credential refresh: %v", err)
				if stopPolicy != nil {
					resetStopPolicy(stopPolicy)
				}
				return
			}

			// special treatment for Timeout exception from SDK-level RequestError
			if aErr.ErrorCode() == "RequestError" && strings.Contains(aErr.ErrorMessage(), "Client.Timeout") {
				if stopPolicy != nil {
					resetStopPolicy(stopPolicy)
				}
				return
			}

			// special treatment for ec2messages AccessDeniedException via smithy error
			if aErr.ErrorCode() == "AccessDeniedException" && strings.Contains(err.Error(), "ec2messages") {
				log.Debugf("ec2messages endpoint access denied (smithy). The ec2messages endpoint is deprecated; ssmmessages is the recommended endpoint.")
				if stopPolicy != nil {
					resetStopPolicy(stopPolicy)
				}
				return
			}
		}

		log.Errorf("error when calling AWS APIs. error details - %v", err)
		if stopPolicy != nil {
			log.Infof("increasing error count by 1")
			stopPolicy.AddErrorCount(1)
		}

	} else {
		// there is no error,
		resetStopPolicy(stopPolicy)
	}
}

// GetAwsErrorCode tries to return AwsError code
func GetAwsErrorCode(err error) string {
	errorCode := ""
	var apiErr smithy.APIError
	if ok := errors.As(err, &apiErr); ok {
		return apiErr.ErrorCode()
	}

	// Fall back to the local awserr type. Some call sites (e.g.
	// EC2RoleProvider.updateEmptyInstanceInformation) convert SDK v2 smithy
	// errors into the local awserr via GetAwsError, which no longer implements
	// smithy.APIError. Without this fallback the underlying code (e.g.
	// AccessDeniedException) is lost, breaking downstream classification such
	// as the Default Host Management fallback gating in EC2RoleProvider.
	var localErr *awserr
	if ok := errors.As(err, &localErr); ok {
		return localErr.Code()
	}

	// V2 compat: when the instance profile is detached, IMDS returns 404 and
	// aws-sdk-go-v2/credentials/ec2rolecreds surfaces it as a bare fmt.Errorf
	// ("no EC2 IMDS role found, %w") that isn't a smithy.APIError. In v1 the
	// same failure came back as awserr.RequestFailure with code
	// EC2RoleRequestError, which was one of the sentinel codes that gated the
	// Default Host Management fallback in EC2RoleProvider.RemoteRetrieve.
	// Preserve that behavior by mapping the v2 error string back to the v1
	// code so the DHMC fallback still triggers when IPR is detached.
	if err != nil && strings.Contains(err.Error(), "no EC2 IMDS role found") {
		return "EC2RoleRequestError"
	}

	return errorCode
}

// GetAwsError tries to return AwsError from either the local awserr type or smithy.APIError
func GetAwsError(err error) *awserr {
	// First try the local awserr type
	var localErr *awserr
	if ok := errors.As(err, &localErr); ok {
		return localErr
	}
	// Fall back to extracting from smithy.APIError (SDK v2 errors)
	var apiErr smithy.APIError
	if ok := errors.As(err, &apiErr); ok {
		return New(apiErr.ErrorCode(), apiErr.ErrorMessage(), err)
	}
	return nil
}

// resetStopPolicy will reset the stoppolicy error count
func resetStopPolicy(stopPolicy *StopPolicy) {
	if stopPolicy != nil {
		stopPolicy.ResetErrorCount()
	}
}

// isNetTimeout checks if the error is a network timeout (net.Error with Timeout() == true)
func isNetTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// isTransportError checks if the error chain contains a smithy transport-level
// send error (*smithyhttp.RequestSendError) or a network error. These represent
// connection/transport failures rather than API-level errors.
func isTransportError(err error) bool {
	if err == nil {
		return false
	}
	var sendErr *smithyhttp.RequestSendError
	if errors.As(err, &sendErr) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}
