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

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/s3util"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
)

// TODO:MF: This should be able to go away when localpackages has encapsulated all filesystem access
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
	WriteFile(filename string, content string) error
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

func (fileSysDepImp) WriteFile(filename string, content string) error {
	return fileutil.WriteAllText(filename, content)
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
	ParseDocument(context context.T, documentRaw []byte, orchestrationDir string, s3Bucket string, s3KeyPrefix string, messageID string, documentID string, defaultWorkingDirectory string) (pluginsInfo []model.PluginState, err error)
	ExecuteDocument(runner runpluginutil.PluginRunner, context context.T, pluginInput []model.PluginState, documentID string, documentCreatedDate string) (pluginOutputs map[string]*contracts.PluginResult)
}

type execDepImp struct {
	util *updateutil.Utility
}

func (m *execDepImp) ParseDocument(context context.T, documentRaw []byte, orchestrationDir string, s3Bucket string, s3KeyPrefix string, messageID string, documentID string, defaultWorkingDirectory string) (pluginsInfo []model.PluginState, err error) {
	return runpluginutil.ParseDocument(context, documentRaw, orchestrationDir, s3Bucket, s3KeyPrefix, messageID, documentID, defaultWorkingDirectory)
}

func (m *execDepImp) ExecuteDocument(runner runpluginutil.PluginRunner, context context.T, pluginInput []model.PluginState, documentID string, documentCreatedDate string) (pluginOutputs map[string]*contracts.PluginResult) {
	log := context.Log()
	log.Debugf("Running subcommand")
	return runner.ExecuteDocument(context, pluginInput, documentID, documentCreatedDate)
}
