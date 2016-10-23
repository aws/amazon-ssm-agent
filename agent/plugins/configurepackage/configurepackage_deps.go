// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package configurepackage implements the ConfigurePackage plugin.
// configurepackage_deps contains platform dependencies that should be able to be stubbed out in tests
package configurepackage

import (
	"io/ioutil"
	"os"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/framework/runutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/s3util"
	"github.com/aws/amazon-ssm-agent/agent/statemanager/model"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
)

var filesysdep fileSysDep = &fileSysDepImp{}

// dependency on filesystem and os utility functions
type fileSysDep interface {
	MakeDirExecute(destinationDir string) (err error)
	GetDirectoryNames(srcPath string) (directories []string, err error)
	GetFileNames(srcPath string) (files []string, err error)
	Exists(filePath string) bool
	Uncompress(src, dest string) error
	RemoveAll(path string) error
	Rename(oldpath, newpath string) error
	ReadFile(filename string) ([]byte, error)
}

type fileSysDepImp struct{}

func (fileSysDepImp) MakeDirExecute(destinationDir string) (err error) {
	return fileutil.MakeDirsWithExecuteAccess(destinationDir)
}

func (fileSysDepImp) GetDirectoryNames(srcPath string) (directories []string, err error) {
	return fileutil.GetDirectoryNames(srcPath)
}

func (fileSysDepImp) GetFileNames(srcPath string) (files []string, err error) {
	return fileutil.GetFileNames(srcPath)
}

func (fileSysDepImp) Exists(filePath string) bool {
	return fileutil.Exists(filePath)
}

func (fileSysDepImp) Uncompress(src, dest string) error {
	return fileutil.Uncompress(src, dest)
}

func (fileSysDepImp) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

func (fileSysDepImp) Rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

func (fileSysDepImp) ReadFile(filename string) ([]byte, error) {
	return ioutil.ReadFile(filename)
}

var networkdep networkDep = &networkDepImp{}

// dependency on S3 and downloaded artifacts
type networkDep interface {
	ListS3Folders(log log.T, amazonS3URL s3util.AmazonS3URL) (folderNames []string, err error)
	Download(log log.T, input artifact.DownloadInput) (output artifact.DownloadOutput, err error)
}

type networkDepImp struct{}

func (networkDepImp) ListS3Folders(log log.T, amazonS3URL s3util.AmazonS3URL) (folderNames []string, err error) {
	return artifact.ListS3Folders(log, amazonS3URL)
}

func (networkDepImp) Download(log log.T, input artifact.DownloadInput) (output artifact.DownloadOutput, err error) {
	return artifact.Download(log, input)
}

var execdep execDep = &execDepImp{util: new(updateutil.Utility)}

// dependency on action execution
type execDep interface {
	ExeCommand(log log.T, cmd string, workingDir string, updaterRoot string, stdOut string, stdErr string, isAsync bool) (err error)
	ParseDocument(plugin *Plugin, documentRaw []byte, orchestrationDir string, s3Bucket string, s3KeyPrefix string, messageID string, documentID string, defaultWorkingDirectory string) (pluginsInfo map[string]model.PluginState, err error)
	ExecuteDocument(plugin *Plugin, pluginInput map[string]model.PluginState, documentID string) (pluginOutputs map[string]*contracts.PluginResult)
}

type execDepImp struct {
	util   *updateutil.Utility
	runner runutil.Runner
}

func (m *execDepImp) ExeCommand(log log.T, cmd string, workingDir string, updaterRoot string, stdOut string, stdErr string, isAsync bool) (err error) {
	return m.util.ExeCommand(log, cmd, workingDir, updaterRoot, stdOut, stdErr, isAsync)
}

func (m *execDepImp) ParseDocument(plugin *Plugin, documentRaw []byte, orchestrationDir string, s3Bucket string, s3KeyPrefix string, messageID string, documentID string, defaultWorkingDirectory string) (pluginsInfo map[string]model.PluginState, err error) {
	return plugin.runner.ParseDocument(plugin.context, documentRaw, orchestrationDir, s3Bucket, s3KeyPrefix, messageID, documentID, defaultWorkingDirectory)
}

func (m *execDepImp) ExecuteDocument(plugin *Plugin, pluginInput map[string]model.PluginState, documentID string) (pluginOutputs map[string]*contracts.PluginResult) {
	log := plugin.context.Log()
	log.Debugf("Running subcommand")
	return plugin.runner.ExecuteDocument(plugin.context, pluginInput, documentID)
}
