// Copyright 2026 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/smithy-go/middleware"
)

// expiredCredsCodes are the error codes on which SDK v1 expired cached credentials and retried.
var expiredCredsCodes = map[string]struct{}{
	"ExpiredToken":          {},
	"ExpiredTokenException": {},
	"RequestExpired":        {},
}

// invalidateCredsOnExpiredError invalidates provider after any attempt that fails with an expired credentials code.
func invalidateCredsOnExpiredError(provider aws.CredentialsProvider) func(*middleware.Stack) error {
	cache, ok := provider.(interface{ Invalidate() })
	if !ok {
		return func(*middleware.Stack) error { return nil }
	}

	invalidate := func(ctx context.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
		out, metadata, err := next.HandleFinalize(ctx, in)
		if _, expired := expiredCredsCodes[GetAwsErrorCode(err)]; expired {
			cache.Invalidate()
		}
		return out, metadata, err
	}

	return func(stack *middleware.Stack) error {
		return stack.Finalize.Add(middleware.FinalizeMiddlewareFunc("InvalidateExpiredCredentials", invalidate), middleware.After)
	}
}
