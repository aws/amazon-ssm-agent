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
	"fmt"
	"io"

	"github.com/aws/amazon-ssm-agent/agent/log"
)

// CommandOutput handles writing output to a string.
type CommandOutput struct {
	// limit to the number of bytes to be written to the output string
	OutputLimit  int
	OutputString *string
}

// Read reads from the stream and writes to the output string
func (c CommandOutput) Read(log log.T, reader *io.PipeReader) {
	defer func() { reader.Close() }()

	// Read byte by byte
	scanner := bufio.NewScanner(reader)
	scanner.Split(bufio.ScanBytes)

	outputLimit := 0
	for scanner.Scan() {
		// Check if size of output is greater than the output limit
		outputLimit++
		if outputLimit > c.OutputLimit {
			break
		}
		*c.OutputString = fmt.Sprintf("%v%v", *c.OutputString, scanner.Text())
	}
	log.Debugf("Number of bytes written to console output: %v", outputLimit)

	if err := scanner.Err(); err != nil {
		log.Error("Error with the scanner while reading the stream")
	}
}
