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
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
)

const (
	maxCloudWatchUploadRetry = 60
)

// File handles writing to an output file and upload to s3 and cloudWatch
type File struct {
	FileName               string
	OrchestrationDirectory string
	OutputS3BucketName     string
	OutputS3KeyPrefix      string
	LogGroupName           string
	LogStreamName          string
}

// CleanUp cleans up local files according to PluginLocalOutputCleanup app config
func (file File) cleanUp(context context.T, uploadComplete bool, exitCode int) {
	pluginLocalOutputCleanup := context.AppConfig().Ssm.PluginLocalOutputCleanup
	log := context.Log()

	if pluginLocalOutputCleanup == appconfig.DefaultPluginOutputRetention {
		return
	}

	// File is incomplete in the case of a reboot
	if exitCode != appconfig.RebootExitCode && (pluginLocalOutputCleanup == appconfig.PluginLocalOutputCleanupAfterExecution ||
		(pluginLocalOutputCleanup == appconfig.PluginLocalOutputCleanupAfterUpload && uploadComplete)) {
		filePath := filepath.Join(file.OrchestrationDirectory, file.FileName)
		log.Debugf("Deleting file at %s", filePath)
		if err := fileutil.DeleteFile(filePath); err != nil {
			log.Errorf("failed to delete orchestration file. Err: %s Filepath: %s", err, filePath)
		}
	}
}

// Read reads from the stream and writes to the output file, s3 and CloudWatchLogs.
func (file File) Read(context context.T, reader *io.PipeReader, exitCode int) {
	uploadComplete := false
	log := context.Log()
	defer func() { reader.Close() }()
	defer func() { file.cleanUp(context, uploadComplete, exitCode) }()

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

	cwl := cloudWatchServiceRetriever.NewCloudWatchLogsService(context)
	if file.LogGroupName != "" {
		log.Debugf("Received CloudWatch Configs: LogGroupName: %s\n, LogStreamName: %s\n", file.LogGroupName, file.LogStreamName)
		//Start CWL logging on different go routine
		go cwl.StreamData(
			file.LogGroupName,
			file.LogStreamName,
			filePath,
			false,
			false,
			make(chan bool),
			false,
			false)
	}

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
		if s3, err := s3ServiceRetriever.NewAmazonS3Util(context, file.OutputS3BucketName); err == nil {
			if err := s3.S3Upload(log, file.OutputS3BucketName, s3Key, filePath); err != nil {
				log.Errorf("Failed to upload the output to s3: %v", err)
			} else {
				uploadComplete = true
			}
		}
	}

	//Block main thread until CloudWatchLogs uploading is complete or until maxCloudWatchUploadRetry is reached
	//TODO Add unit test to test maxRetry logic
	if file.LogGroupName != "" {
		cwl.SetIsFileComplete(true)
		retry := 0
		for !cwl.GetIsUploadComplete() && retry < maxCloudWatchUploadRetry {
			retry++
			time.Sleep(cloudWatchUploadFrequency)
		}

		uploadComplete = uploadComplete || cwl.GetIsUploadComplete()
	}
}
