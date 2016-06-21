// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Amazon Software License (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/asl/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// +build darwin freebsd linux netbsd openbsd

// Package updateutil contains updater specific utilities.
package updateutil

import (
	"os/exec"
	"syscall"

	"github.com/aws/amazon-ssm-agent/agent/log"
)

const (
	// UpdateCmd represents the command argument for update
	UpdateCmd = "update"

	// SourceVersionCmd represents the command argument for source version
	SourceVersionCmd = "source.version"

	// SourceLocationCmd represents the command argument for source location
	SourceLocationCmd = "source.location"

	// SourceHashCmd represents the command argument for source hash value
	SourceHashCmd = "source.hash"

	// TargetVersionCmd represents the command argument for target version
	TargetVersionCmd = "target.version"

	// TargetLocationCmd represents the command argument for target location
	TargetLocationCmd = "target.location"

	// TargetHashCmd represents the command argument for target hash value
	TargetHashCmd = "target.hash"

	// PackageNameCmd represents the command argument for package name
	PackageNameCmd = "package.name"

	// MessageIDCmd represents the command argument for message id
	MessageIDCmd = "messageid"

	// StdoutFileName represents the command argument for standard output file
	StdoutFileName = "stdout"

	// StderrFileName represents the command argument for standard error file
	StderrFileName = "stderr"

	// OutputKeyPrefixCmd represents the command argument for output key prefix
	OutputKeyPrefixCmd = "output.key"

	// OutputBucketNameCmd represents the command argument for output bucket name
	OutputBucketNameCmd = "output.bucket"
)

const (
	// CompressFormat represents the compress format for linux platform
	CompressFormat = "tar.gz"
)

const (
	// Installer represents Install shell script
	Installer = "install.sh"

	// UnInstaller represents Uninstall shell script
	UnInstaller = "uninstall.sh"
)

func prepareProcess(command *exec.Cmd) {
	// make the process the leader of its process group
	// (otherwise we cannot kill it properly)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func agentStatusOutput() ([]byte, error) {
	return execCommand("status", "amazon-ssm-agent").Output()
}

func agentExpectedStatus() string {
	return "amazon-ssm-agent start/running"
}

func isUpdateSupported(log log.T) (bool, error) {
	return true, nil
}
