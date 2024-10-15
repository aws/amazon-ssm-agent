// Copyright 2024 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package processor contains the methods for update ssm agent.
// It also provides methods for sendReply and updateInstanceInfo

//go:build darwin || freebsd || linux || netbsd || openbsd
// +build darwin freebsd linux netbsd openbsd

package processor

import (
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
)

func updateExecutionTimeoutIfNeeded(retryCount int, defaultTimeOut int, updateUtil updateutil.T) {
	return
}

func getCommandEnv(log log.T, rootDir string) []string {
	return nil
}

// moveCleanInstallationDir is a dummy function for unix
func moveCleanInstallationDir(log log.T, updateDetail *UpdateDetail) {
	log.Info("Skipping move of files from update installation directory")
}
