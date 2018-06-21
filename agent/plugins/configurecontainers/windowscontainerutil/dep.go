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

package windowscontainerutil

import (
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/log"
)

var dep dependencies

type dependencies interface {
	PlatformVersion(log log.T) (version string, err error)
	IsPlatformNanoServer(log log.T) (bool, error)
	SetDaemonConfig(daemonConfigPath string, daemonConfigContent string) (err error)
	MakeDirs(destinationDir string) (err error)
	TempDir(dir, prefix string) (name string, err error)
	UpdateUtilExeCommandOutput(
		customUpdateExecutionTimeoutInSeconds int,
		log log.T,
		cmd string,
		parameters []string,
		workingDir string,
		outputRoot string,
		stdOut string,
		stdErr string,
		usePlatformSpecificCommand bool) (output string, err error)
	ArtifactDownload(log log.T, input artifact.DownloadInput) (output artifact.DownloadOutput, err error)
	LocalRegistryKeySetDWordValue(path string, name string, value uint32) error
	LocalRegistryKeyGetStringValue(path string, name string) (val string, valtype uint32, err error)
	FileutilUncompress(log log.T, src, dest string) error
}
