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
//
//go:build linux || windows
// +build linux windows

package serialport

import (
	"context"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
)

const imdsMaxAttempts = 10

// IsEC2Environment reports whether the agent is running in EC2, determined by
// whether the instance metadata service is reachable. The serial console is
// only consumed in EC2 (GetConsoleOutput); on non-EC2 hosts (on-prem, bare metal)
// writing to it has no reader and can interfere with other users of the port.
// It attempts to get metadata with retry up to 10 times to ignore arbitrary failures/errors.
func IsEC2Environment(log log.T) bool {
	// IMDS ignores RetryMaxAttempts and only honors a Retryer func, so bound
	// retries via WithRetryer instead of WithRetryMaxAttempts.
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRetryer(func() aws.Retryer {
		return retry.NewStandard(func(o *retry.StandardOptions) {
			o.MaxAttempts = imdsMaxAttempts
		})
	}))
	if err != nil {
		// LoadDefaultConfig can fail for reasons unrelated to EC2 detection
		// (e.g. unreadable shared config/credentials files). Rather than
		// treating this as "not in EC2", log the error and fall back to a
		// minimal config so we can still probe the metadata service.
		log.Warnf("failed to load default AWS config, falling back to minimal config: %v", err)
		cfg = aws.Config{
			RetryMaxAttempts: imdsMaxAttempts,
		}
	}

	client := imds.NewFromConfig(cfg)
	out, err := client.GetMetadata(context.TODO(), &imds.GetMetadataInput{Path: ""})
	if err != nil {
		// This is as designed to check if instance is in EC2, so it is not an error
		return false
	}
	// The GetMetadata output Content is an io.ReadCloser that must be closed to
	// avoid leaking the underlying HTTP response body.
	defer out.Content.Close()

	return true
}
