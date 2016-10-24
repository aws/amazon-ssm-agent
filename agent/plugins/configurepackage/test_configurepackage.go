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
// test_configurepackage contains stub implementations
package configurepackage

import (
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/s3util"
	"github.com/aws/amazon-ssm-agent/agent/statemanager/model"
)

type ConfigurePackageStubs struct {
	// individual stub functions or interfaces go here with a temp variable for the original version
	fileSysDepStub fileSysDep
	fileSysDepOrig fileSysDep
	networkDepStub networkDep
	networkDepOrig networkDep
	execDepStub    execDep
	execDepOrig    execDep
}

// Set replaces dependencies with stub versions and saves the original version.
// it should always be followed by defer Clear()
func (m *ConfigurePackageStubs) Set() {
	if m.fileSysDepStub != nil {
		m.fileSysDepOrig = filesysdep
		filesysdep = m.fileSysDepStub
	}
	if m.networkDepStub != nil {
		m.networkDepOrig = networkdep
		networkdep = m.networkDepStub
	}
	if m.execDepStub != nil {
		m.execDepOrig = execdep
		execdep = m.execDepStub
	}
}

// Clear resets dependencies to their original values.
func (m *ConfigurePackageStubs) Clear() {
	if m.fileSysDepStub != nil {
		filesysdep = m.fileSysDepOrig
	}
	if m.networkDepStub != nil {
		networkdep = m.networkDepOrig
	}
	if m.execDepStub != nil {
		execdep = m.execDepOrig
	}
}

type FileSysDepStub struct {
	makeFileError        error
	directoriesResult    []string
	directoriesError     error
	filesResult          []string
	filesError           error
	existsResultDefault  bool
	existsResultSequence []bool
	uncompressError      error
	removeError          error
	renameError          error
	readResult           []byte
	readError            error
	writeError           error
}

func (m *FileSysDepStub) MakeDirExecute(destinationDir string) (err error) {
	return m.makeFileError
}

func (m *FileSysDepStub) GetDirectoryNames(srcPath string) (directories []string, err error) {
	return m.directoriesResult, m.directoriesError
}

func (m *FileSysDepStub) GetFileNames(srcPath string) (files []string, err error) {
	return m.filesResult, m.filesError
}

func (m *FileSysDepStub) Exists(filePath string) bool {
	if len(m.existsResultSequence) > 0 {
		result := m.existsResultSequence[0]
		if len(m.existsResultSequence) > 1 {
			m.existsResultSequence = append(m.existsResultSequence[:0], m.existsResultSequence[1:]...)
		} else {
			m.existsResultSequence = nil
		}
		return result
	}
	return m.existsResultDefault
}

func (m *FileSysDepStub) Uncompress(src, dest string) error {
	return m.uncompressError
}

func (m *FileSysDepStub) RemoveAll(path string) error {
	return m.removeError
}

func (m *FileSysDepStub) Rename(oldpath, newpath string) error {
	return m.renameError
}

func (m *FileSysDepStub) ReadFile(filename string) ([]byte, error) {
	return m.readResult, m.readError
}

func (m *FileSysDepStub) WriteFile(filename string, content string) error {
	return m.writeError
}

type NetworkDepStub struct {
	foldersResult          []string
	foldersError           error
	downloadResultDefault  artifact.DownloadOutput
	downloadErrorDefault   error
	downloadResultSequence []artifact.DownloadOutput
	downloadErrorSequence  []error
}

func (m *NetworkDepStub) ListS3Folders(log log.T, amazonS3URL s3util.AmazonS3URL) (folderNames []string, err error) {
	return m.foldersResult, m.foldersError
}

func (m *NetworkDepStub) Download(log log.T, input artifact.DownloadInput) (output artifact.DownloadOutput, err error) {
	if len(m.downloadResultSequence) > 0 {
		result := m.downloadResultSequence[0]
		error := m.downloadErrorSequence[0]
		if len(m.downloadResultSequence) > 1 {
			m.downloadResultSequence = append(m.downloadResultSequence[:0], m.downloadResultSequence[1:]...)
			m.downloadErrorSequence = append(m.downloadErrorSequence[:0], m.downloadErrorSequence[1:]...)
		} else {
			m.downloadResultSequence = nil
			m.downloadErrorSequence = nil
		}
		return result, error
	}
	return m.downloadResultDefault, m.downloadErrorDefault
}

type ExecDepStub struct {
	execError    error
	pluginInput  *model.PluginState
	parseError   error
	pluginOutput *contracts.PluginResult
}

func (m *ExecDepStub) ExeCommand(log log.T, cmd string, workingDir string, updaterRoot string, stdOut string, stdErr string, isAsync bool) (err error) {
	return m.execError
}

func (m *ExecDepStub) ParseDocument(plugin *Plugin, documentRaw []byte, orchestrationDir string, s3Bucket string, s3KeyPrefix string, messageID string, documentID string, defaultWorkingDirectory string) (pluginsInfo map[string]model.PluginState, err error) {
	pluginsInfo = make(map[string]model.PluginState)
	if m.pluginInput != nil {
		pluginsInfo["test"] = *m.pluginInput
	}
	return pluginsInfo, m.parseError
}

func (m *ExecDepStub) ExecuteDocument(plugin *Plugin, pluginInput map[string]model.PluginState, documentID string) (pluginOutputs map[string]*contracts.PluginResult) {
	pluginOutputs = make(map[string]*contracts.PluginResult)
	if m.pluginOutput != nil {
		pluginOutputs["test"] = m.pluginOutput
	}
	return
}
