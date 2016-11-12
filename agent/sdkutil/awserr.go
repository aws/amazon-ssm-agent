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
	"runtime"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/aws-sdk-go/aws/awserr"
)

// HandleAwsError logs an AWS error.
func HandleAwsError(log log.T, err error, stopPolicy *StopPolicy) {
	if err != nil {
		// notice that we're using 1, so it will actually log the where
		// the error happened, 0 = this function, we don't want that.
		pc, fn, line, _ := runtime.Caller(1)
		log.Debugf("error in %s[%s:%d] %v", runtime.FuncForPC(pc).Name(), fn, line, err)

		// In case this is aws error, update the stop policy as well.
		if aErr, ok := err.(awserr.Error); ok {
			// Generic AWS Error with Code, Message, and original error (if any)
			log.Debugf("AWS error. Code: %v, Message: %v, origerror: %v ", aErr.Code(), aErr.Message(), aErr.OrigErr())

			// special treatment for Timeout exception - as this is expected
			if aErr.Code() == "RequestError" && aErr.OrigErr() != nil && strings.Contains(aErr.OrigErr().Error(), "Client.Timeout") {
				// resetting the error count to 0 - as these exceptions are all expected
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
	if aErr, ok := err.(awserr.Error); ok {
		return aErr.Code()
	}

	return errorCode
}

// resetStopPolicy will reset the stoppolicy error count
func resetStopPolicy(stopPolicy *StopPolicy) {
	if stopPolicy != nil {
		stopPolicy.ResetErrorCount()
	}
}
