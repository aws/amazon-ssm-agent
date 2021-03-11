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

package iomodule

import (
	"bufio"
	"io"
	"os"
	"path/filepath"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
)

// CommandOutput handles writing output to a string.
type CommandOutput struct {
	OutputString           *string
	FileName               string
	OrchestrationDirectory string
}

// CleanUp cleans up local files according to PluginLocalOutputCleanup app config
func (c CommandOutput) cleanUp(context context.T, exitCode int) {
	pluginLocalOutputCleanup := context.AppConfig().Ssm.PluginLocalOutputCleanup
	if pluginLocalOutputCleanup != appconfig.DefaultPluginOutputRetention && exitCode != appconfig.RebootExitCode {
		filePath := filepath.Join(c.OrchestrationDirectory, c.FileName)
		fileutil.DeleteFile(filePath)
	}
}

func (c CommandOutput) Read(context context.T, reader *io.PipeReader, exitCode int) {
	log := context.Log()
	defer func() { reader.Close() }()
	defer c.cleanUp(context, exitCode)

	if err := fileutil.MakeDirs(c.OrchestrationDirectory); err != nil {
		log.Errorf("failed to create orchestrationDir directory at %v: %v", c.OrchestrationDirectory, err)
		return
	}
	filePath := filepath.Join(c.OrchestrationDirectory, c.FileName)
	fileWriter, err := os.OpenFile(filePath, appconfig.FileFlagsCreateOrAppend, appconfig.ReadWriteAccess)

	if err != nil {
		log.Errorf("Failed to open the file at %v: %v", filePath, err)
		return
	}

	defer fileWriter.Close()

	// Read byte by byte and write to file
	scanner := bufio.NewScanner(reader)
	scanner.Split(bufio.ScanBytes)
	for scanner.Scan() {
		if _, err = fileWriter.Write([]byte(scanner.Text())); err != nil {
			log.Errorf("Failed to write the message to stdoutConsoleFile: %v", err)
		}
	}

	// Check if scanner exited because of an error
	if err := scanner.Err(); err != nil {
		log.Error("Error with the scanner while reading the stream")
	}

	fi, err := fileWriter.Stat()
	if err != nil {
		log.Errorf("Failed to get file stat: %v", err)
		return
	}

	// Write output to console
	if fi.Size() > 0 {
		*c.OutputString, err = fileutil.ReadAllText(filePath)
		if err != nil {
			log.Errorf("Error reading %v at path %v", c.FileName, filePath)
		}
	}
}
