// +build windows

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
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
	"golang.org/x/sys/windows/registry"
)

func init() {
	dep = &DepWindows{}
}

type DepWindows struct{}

func (DepWindows) PlatformVersion(log log.T) (version string, err error) {
	return platform.PlatformVersion(log)
}

func (DepWindows) IsPlatformNanoServer(log log.T) (bool, error) {
	return platform.IsPlatformNanoServer(log)
}

func (DepWindows) SetDaemonConfig(daemonConfigPath string, daemonConfigContent string) (err error) {
	if _, err := os.Stat(daemonConfigPath); os.IsNotExist(err) {
		os.MkdirAll(filepath.Dir(daemonConfigPath), 744)
		err := ioutil.WriteFile(daemonConfigPath, []byte(daemonConfigContent), 0644)
		if err != nil {
			return err
		}
	}
	return nil
}

func (DepWindows) MakeDirs(destinationDir string) (err error) {
	return fileutil.MakeDirs(destinationDir)
}
func (DepWindows) TempDir(dir, prefix string) (name string, err error) {
	return ioutil.TempDir(dir, prefix)
}

func (DepWindows) UpdateUtilExeCommandOutput(
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

func (DepWindows) ArtifactDownload(log log.T, input artifact.DownloadInput) (output artifact.DownloadOutput, err error) {
	return artifact.Download(log, input)
}

func openLocalRegistryKey(path string) (registry.Key, error) {
	return registry.OpenKey(registry.LOCAL_MACHINE, path, registry.ALL_ACCESS)
}

func (DepWindows) LocalRegistryKeySetDWordValue(path string, name string, value uint32) error {
	key, err := openLocalRegistryKey(path)
	if err != nil {
		return err
	}
	defer key.Close()
	return key.SetDWordValue(name, value)
}

func (DepWindows) LocalRegistryKeyGetStringValue(path string, name string) (val string, valtype uint32, err error) {
	key, err := openLocalRegistryKey(path)
	if err != nil {
		return "", 0, err
	}
	defer key.Close()
	return key.GetStringValue(name)
}

func (DepWindows) FileutilUncompress(log log.T, src, dest string) error {
	return fileutil.Uncompress(log, src, dest)
}
