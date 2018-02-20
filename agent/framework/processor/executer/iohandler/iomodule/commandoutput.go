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
	"bytes"
	"io"

	"path/filepath"

	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
)

// CommandOutput handles writing output to a string.
type CommandOutput struct {
	// limit to the number of bytes to be written to the output string
	OutputLimit            int
	OutputString           *string
	FileName               string
	OrchestrationDirectory string
}

func (c CommandOutput) Read(log log.T, reader *io.PipeReader) {
	defer func() { reader.Close() }()
	filePath := filepath.Join(c.OrchestrationDirectory, c.FileName)
	var buf string
	var err error
	var buffer bytes.Buffer
	buf, err = fileutil.ReadAllText(filePath)

	if buf != "" {
		if len(buf) > c.OutputLimit {
			buffer.WriteString(buf[:c.OutputLimit])
		} else {
			buffer.WriteString(buf)
		}
	}
	if err != nil {
		log.Errorf("Error reading %v at path %v", c.FileName, filePath)
	}
	c.ReadPipeAndFile(log, reader, buffer)
}

func (c CommandOutput) ReadPipeAndFile(log log.T, reader *io.PipeReader, buffer bytes.Buffer) {
	// Read byte by byte
	scanner := bufio.NewScanner(reader)
	scanner.Split(bufio.ScanBytes)

	outputLimit := buffer.Len()
	for scanner.Scan() {
		// Check if size of output is greater than the output limit
		outputLimit++
		if outputLimit > c.OutputLimit {
			break
		}
		buffer.WriteString(scanner.Text())
	}
	// Clear contents of string to avoid duplicate output
	*c.OutputString = ""
	*c.OutputString = fmt.Sprintf("%v%v", *c.OutputString, buffer.String())
	log.Debugf("Number of bytes written to console output: %v", outputLimit-1)

	if err := scanner.Err(); err != nil {
		log.Error("Error with the scanner while reading the stream")
	}
}
