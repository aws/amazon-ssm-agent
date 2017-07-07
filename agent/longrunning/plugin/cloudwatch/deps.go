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

// Package cloudwatch implements cloudwatch plugin and its configuration
package cloudwatch

import (
	"bufio"
	"io"
	"os"

	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
)

// fileUtil defines the operations that fileutil uses to interact with file system
type fileUtil interface {
	Exists(filePath string) bool
	MakeDirs(destinationDir string) error
	WriteIntoFileWithPermissions(absolutePath, content string, perm os.FileMode) (bool, error)
}

type fileUtilImpl struct{}

// Exists returns true if the given file exists, false otherwise, ignoring any underlying error
func (f fileUtilImpl) Exists(filePath string) bool {
	return fileutil.Exists(filePath)
}

// MakeDirs create the directories along the path if missing.
func (f fileUtilImpl) MakeDirs(destinationDir string) error {
	return fileutil.MakeDirs(destinationDir)
}

// WriteIntoFileWithPermissions writes into file with given file mode permissions
func (f fileUtilImpl) WriteIntoFileWithPermissions(absolutePath, content string, perm os.FileMode) (bool, error) {
	return fileutil.WriteIntoFileWithPermissions(absolutePath, content, perm)
}

var fileUtilWrapper fileUtil = fileUtilImpl{}

// readLastLine reads the last line of the file
func readLastLine(log log.T, filename string) string {
	file, err := os.Open(filename)
	if err != nil {
		panic(err)
	}

	defer file.Close()

	reader := bufio.NewReader(file)

	// we need to calculate the size of the last line for file.ReadAt(offset) to work
	// NOTE : not a very effective solution as we need to read the entire file at least for 1 pass :(
	// will change later
	lastLineSize := 0
	lastTwoLineSize := 0
	size := 0

	for {
		line, _, err := reader.ReadLine()

		if err == io.EOF {
			break
		}
		lastTwoLineSize = lastLineSize
		lastLineSize = len(line)
	}

	fileInfo, err := os.Stat(filename)

	size = lastTwoLineSize
	if lastLineSize > 1 {
		size = lastLineSize
	}

	// make a buffer size according to the lastLineSize
	buffer := make([]byte, size)

	// +1 to compensate for the initial 0 byte of the line
	// otherwise, the initial character of the line will be missing

	// instead of reading the whole file into memory, we just read from certain offset

	offset := fileInfo.Size() - int64(size+2)
	numRead, err := file.ReadAt(buffer, offset)
	buffer = buffer[:numRead]
	return string(buffer)
}
