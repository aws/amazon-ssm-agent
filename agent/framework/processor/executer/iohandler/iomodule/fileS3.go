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
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/s3util"
)

// File handles writing to an output file and upload to s3
type File struct {
	FileName               string
	OrchestrationDirectory string
	OutputS3BucketName     string
	OutputS3KeyPrefix      string
}

// Read reads from the stream and writes to the output file and s3.
func (file File) Read(log log.T, reader *io.PipeReader) {
	defer func() { reader.Close() }()

	log.Debugf("OrchestrationDir %v ", file.OrchestrationDirectory)

	// create orchestration dir if needed
	if err := fileutil.MakeDirs(file.OrchestrationDirectory); err != nil {
		log.Errorf("failed to create orchestrationDir directory at %v: %v", file.OrchestrationDirectory, err)
		return
	}

	filePath := filepath.Join(file.OrchestrationDirectory, file.FileName)
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
			log.Errorf("Failed to write the message to stdout: %v", err)
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

	// Upload output file to S3
	if file.OutputS3BucketName != "" && fi.Size() > 0 {
		s3Key := fileutil.BuildS3Path(file.OutputS3KeyPrefix, file.FileName)
		if err := s3util.NewAmazonS3Util(log, file.OutputS3BucketName).S3Upload(log, file.OutputS3BucketName, s3Key, filePath); err != nil {
			log.Errorf("Failed to upload the output to s3: %v", err)
		}
	}
}
