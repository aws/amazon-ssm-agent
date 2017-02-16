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
//
// +build darwin freebsd linux netbsd openbsd

package linuxcontainerutil

import (
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
)

func init() {
	dep = &DepLinux{}
}

type DepLinux struct{}

func (DepLinux) GetInstanceContext(log log.T) (instanceContext *updateutil.InstanceContext, err error) {
	updateUtil := new(updateutil.Utility)
	return updateUtil.CreateInstanceContext(log)
}

func (DepLinux) UpdateUtilExeCommandOutput(
	customUpdateExecutionTimeoutInSeconds int,
	log log.T,
	cmd string,
	parameters []string,
	workingDir string,
	outputRoot string,
	stdOut string,
	stdErr string,
	usePlatformSpecificCommand bool) (output string, err error) {
	util := updateutil.Utility{CustomUpdateExecutionTimeoutInSeconds: customUpdateExecutionTimeoutInSeconds}
	return util.ExeCommandOutput(log, cmd, parameters, workingDir, outputRoot, stdOut, stdErr, usePlatformSpecificCommand)
}
