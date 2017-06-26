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

// Package cloudwatchlogspublisher is responsible for pulling logs from the log queue and publishing them to cloudwatch
package cloudwatchlogspublisher

import (
	"time"

	"github.com/aws/amazon-ssm-agent/agent/cloudwatchlogspublisher/cloudwatchlogsinterface"
	"github.com/aws/amazon-ssm-agent/agent/cloudwatchlogsqueue"
	"github.com/aws/amazon-ssm-agent/agent/log"
)

const (
	dataAlreadyAcceptedException   = "DataAlreadyAcceptedException"
	invalidSequenceTokenException  = "InvalidSequenceTokenException"
	resourceAlreadyExistsException = "ResourceAlreadyExistsException"
	defaultPollingInterval         = time.Second
	defaultPollingWaitTime         = time.Second
)

// ICloudWatchPublisher interface for publishing logs to cloudwatchlogs
type ICloudWatchPublisher interface {
	Init(log log.T) (err error)
	Start()
	Stop()
}

// CloudWatchPublisher wrapper to publish logs to cloudwatchlogs
type CloudWatchPublisher struct {
	cloudWatchLogsService cloudwatchlogsinterface.ICloudWatchLogsService
	publisherTicker       *time.Ticker
	QueuePollingInterval  time.Duration // The interval after which the publisher polls the queue
	QueuePollingWaitTime  time.Duration // The duration for which the publisher blocks while polling. For negative value will wait until enqueue
	log                   log.T
}

// Init initializes the publisher
func (cloudwatchPublisher *CloudWatchPublisher) Init(log log.T) error {

	log.Infof("Init the cloudwatchlogs publisher")

	cloudwatchPublisher.cloudWatchLogsService = NewCloudWatchLogsService()

	cloudwatchPublisher.log = log

	// Create if the LogGroup and LogStream are not present
	if err := cloudwatchPublisher.createLogGroupAndStream(log); err != nil {
		// Aborting Init
		log.Errorf("Error in ensuring log group and stream are present:%v", err)
		return err
	}

	// Setting the ticker interval for polling if not set or negatve
	if cloudwatchPublisher.QueuePollingInterval <= 0 {
		cloudwatchPublisher.QueuePollingInterval = defaultPollingInterval
	}

	// Setting the polling wait time if not set or 0
	if cloudwatchPublisher.QueuePollingWaitTime == 0 {
		cloudwatchPublisher.QueuePollingInterval = defaultPollingWaitTime
	}

	// Either log group and stream are present or created successfully
	return nil
}

// createLogGroupAndStream checks if log group and log stream are present. If not, creates them
func (cloudwatchPublisher *CloudWatchPublisher) createLogGroupAndStream(log log.T) error {

	logGroup := cloudwatchlogsqueue.GetLogGroup()
	logStream := cloudwatchlogsqueue.GetLogStream()

	if !cloudwatchPublisher.cloudWatchLogsService.IsLogGroupPresent(log, logGroup) {
		//Create Log Group
		err := cloudwatchPublisher.cloudWatchLogsService.CreateLogGroup(log, logGroup)
		if err != nil {
			// Aborting Init
			log.Errorf("Error creating log group:%v", err)
			return err
		}
	}

	if !cloudwatchPublisher.cloudWatchLogsService.IsLogStreamPresent(log, logGroup, logStream) {
		//Create Log Stream
		err := cloudwatchPublisher.cloudWatchLogsService.CreateLogStream(log, logGroup, logStream)
		if err != nil {
			// Aborting Init
			log.Errorf("Error creating log stream:%v", err)
			return err
		}
	}
	return nil
}

// Start starts the publisher to consume messages from the queue
func (cloudwatchPublisher *CloudWatchPublisher) Start() {

	cloudwatchPublisher.log.Infof("Start the cloudwatchlogs publisher")

	logGroup := cloudwatchlogsqueue.GetLogGroup()
	logStream := cloudwatchlogsqueue.GetLogStream()

	cloudwatchPublisher.log.Debugf("Cloudwatchlogs Publishing Logs to LogGroup: %v", logGroup)
	cloudwatchPublisher.log.Debugf("Cloudwatchlogs Publishing Logs to LogStream: %v", logStream)

	// Get the sequence token required to publish events to stream
	sequenceToken := cloudwatchPublisher.cloudWatchLogsService.GetSequenceTokenForStream(cloudwatchPublisher.log, logGroup, logStream)

	// Create a ticker for every second
	cloudwatchPublisher.publisherTicker = time.NewTicker(cloudwatchPublisher.QueuePollingInterval)

	go func() {
		for range cloudwatchPublisher.publisherTicker.C {

			//Check If Messages are in the Queue. If Messages are there continue to Push them to CW until empty
			messages, err := cloudwatchlogsqueue.Dequeue(cloudwatchPublisher.QueuePollingWaitTime)
			if err != nil {
				cloudwatchPublisher.log.Debugf("Error Dequeueing Messages from Cloudwatchlogs Queue : %v", err)
			}

			if messages != nil {
				// There are some messages. Call the PUT Api
				if sequenceToken, err = cloudwatchPublisher.cloudWatchLogsService.PutLogEvents(cloudwatchPublisher.log, messages, logGroup, logStream, sequenceToken); err != nil {
					// Error pushing logs even after retries and fixing sequence token
					// Skipping the batch and continuing
					cloudwatchPublisher.log.Errorf("Error pushing logs, skipping the batch:%v", err)
				}
			}
		}

	}()
}

// Stop called to stop the publisher
func (cloudwatchPublisher *CloudWatchPublisher) Stop() {
	cloudwatchPublisher.log.Infof("Stop the cloudwatchlogs publisher")
	if cloudwatchPublisher.publisherTicker != nil {
		cloudwatchPublisher.publisherTicker.Stop()
	}
}
