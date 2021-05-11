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
	"runtime/debug"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/agentlogstocloudwatch/cloudwatchlogspublisher/cloudwatchlogsinterface"
	"github.com/aws/amazon-ssm-agent/agent/agentlogstocloudwatch/cloudwatchlogsqueue"
	"github.com/aws/amazon-ssm-agent/agent/context"
)

const (
	dataAlreadyAcceptedException   = "DataAlreadyAcceptedException"
	invalidSequenceTokenException  = "InvalidSequenceTokenException"
	resourceAlreadyExistsException = "ResourceAlreadyExistsException"
	resourceNotFoundException      = "ResourceNotFoundException"
	defaultPollingInterval         = time.Second
	defaultPollingWaitTime         = 200 * time.Millisecond
	pollingBackoffMultiplier       = 2
	maxPollingInterval             = 30 * time.Second
)

// ICloudWatchPublisher interface for publishing logs to cloudwatchlogs
type ICloudWatchPublisher interface {
	Init() (err error)
	Start()
	Stop()
}

// CloudWatchPublisher wrapper to publish logs to cloudwatchlogs
type CloudWatchPublisher struct {
	context                      context.T
	cloudWatchLogsService        cloudwatchlogsinterface.ICloudWatchLogsService
	cloudWatchLogsServiceSharing cloudwatchlogsinterface.ICloudWatchLogsService
	selfDestination              *destinationConfigurations
	sharingDestination           *destinationConfigurations
	isSharingEnabled             bool
	publisherTicker              *time.Ticker
	stopPollingChannel           chan bool
	QueuePollingInterval         time.Duration // The interval after which the publisher polls the queue
	QueuePollingWaitTime         time.Duration // The duration for which the publisher blocks while polling. For negative value will wait until enqueue
}

// destinationConfigurations captures the cloudwatchlogs destination configurations required for pushing logs
type destinationConfigurations struct {
	logGroup        string
	logStream       string
	accessKeyId     string
	secretAccessKey string
}

func NewCloudWatchPublisher(context context.T) *CloudWatchPublisher {
	return &CloudWatchPublisher{
		context: context,
	}
}

// Init initializes the publisher
func (cloudwatchPublisher *CloudWatchPublisher) Init() {
	log := cloudwatchPublisher.context.Log()
	defer func() {
		// recover in case the init panics
		if msg := recover(); msg != nil {
			log.Errorf("Cloudwatchlogs publisher init failed:%v", msg)
		}
	}()

	log.Infof("Init the cloudwatchlogs publisher")

	// Setting the ticker interval for polling if not set or negatve
	if cloudwatchPublisher.QueuePollingInterval <= 0 {
		cloudwatchPublisher.QueuePollingInterval = defaultPollingInterval
	}

	// Setting the polling wait time if not set or 0
	if cloudwatchPublisher.QueuePollingWaitTime <= 0 {
		cloudwatchPublisher.QueuePollingWaitTime = defaultPollingWaitTime
	}

	if cloudwatchlogsqueue.IsActive() {
		cloudwatchPublisher.Start()
	}

	go cloudwatchPublisher.CloudWatchLogsEventsListener()
}

// CloudWatchLogsEventsListener listens to cloudwatchlogs events channel
func (cloudwatchPublisher *CloudWatchPublisher) CloudWatchLogsEventsListener() {
	log := cloudwatchPublisher.context.Log()
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Cloudwatch listener panic: %v", r)
			log.Errorf("Stacktrace:\n%s", debug.Stack())
		}
	}()
	for event := range cloudwatchlogsqueue.CloudWatchLogsEventsChannel {

		switch event {
		case cloudwatchlogsqueue.QueueActivated:
			cloudwatchPublisher.Start()
		case cloudwatchlogsqueue.QueueDeactivated:
			cloudwatchPublisher.Stop()
		case cloudwatchlogsqueue.LoggingDestinationChanged:
			cloudwatchPublisher.Stop()
			cloudwatchPublisher.Start()
		}
	}
}

// createLogGroupAndStream checks if log group and log stream are present. If not, creates them
func (cloudwatchPublisher *CloudWatchPublisher) createLogGroupAndStream(logGroup, logStream string) error {
	log := cloudwatchPublisher.context.Log()
	//Create Log Group
	if err := cloudwatchPublisher.cloudWatchLogsService.CreateLogGroup(logGroup); err != nil {
		// Aborting Init
		log.Errorf("Error creating log group: %v", err)
		return err
	}

	if err := cloudwatchPublisher.cloudWatchLogsService.CreateLogStream(logGroup, logStream); err != nil {
		// Aborting Init
		log.Errorf("Error creating log stream: %v", err)
		return err
	}
	return nil
}

// Start starts the publisher to consume messages from the queue
func (cloudwatchPublisher *CloudWatchPublisher) Start() {
	log := cloudwatchPublisher.context.Log()
	log.Infof("Start the cloudwatchlogs publisher")

	var err error
	// If service nil, create a new service, else use the existing one
	if cloudwatchPublisher.cloudWatchLogsService == nil {
		cloudwatchPublisher.cloudWatchLogsService = NewCloudWatchLogsService(cloudwatchPublisher.context)
	}

	logGroup := cloudwatchlogsqueue.GetLogGroup()
	logStream, err := cloudwatchPublisher.context.Identity().ShortInstanceID()
	if err != nil {
		log.Errorf("Error in getting instance Id :%v. Aborting CloudWatchlogs publisher start", err)
		return
	}

	log.Debugf("Cloudwatchlogs Publishing Logs to LogGroup: %v", logGroup)
	log.Debugf("Cloudwatchlogs Publishing Logs to LogStream: %v", logStream)

	cloudwatchPublisher.selfDestination = &destinationConfigurations{
		logGroup:  logGroup,
		logStream: logStream,
	}

	// Create if the LogGroup and LogStream are not present
	if err = cloudwatchPublisher.createLogGroupAndStream(logGroup, logStream); err != nil {
		// Aborting Start
		log.Errorf("Error in ensuring log group and stream are present:%v", err)
		return
	}

	// Get the sequence token required to publish events to stream
	sequenceToken := cloudwatchPublisher.cloudWatchLogsService.GetSequenceTokenForStream(logGroup, logStream)

	// Setup sharing if enabled
	cloudwatchPublisher.isSharingEnabled = cloudwatchlogsqueue.IsLogSharingEnabled()
	var sequenceTokenSharing *string
	log.Debugf("Cloudwatchlogs Sharing Enabled: %v", cloudwatchPublisher.isSharingEnabled)

	if cloudwatchPublisher.isSharingEnabled {
		if cloudwatchPublisher.sharingDestination = getSharingConfigurations(); cloudwatchPublisher.sharingDestination == nil {
			log.Error("Sharing Configurations Incorrect. Abort Sharing.")
			cloudwatchPublisher.isSharingEnabled = false
		} else {
			sequenceTokenSharing = cloudwatchPublisher.setupSharing()
		}
	}

	cloudwatchPublisher.stopPollingChannel = make(chan bool)
	cloudwatchPublisher.startPolling(sequenceToken, sequenceTokenSharing)
}

// startPolling creates a ticker and starts polling the queue
func (cloudwatchPublisher *CloudWatchPublisher) startPolling(sequenceToken, sequenceTokenSharing *string) {
	log := cloudwatchPublisher.context.Log()
	// Create a ticker for every second
	currentPollingInterval := cloudwatchPublisher.QueuePollingInterval
	cloudwatchPublisher.publisherTicker = time.NewTicker(currentPollingInterval)
	pollingShouldBackoff := false

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("Cloudwatch publisher poll panic: %v", r)
				log.Errorf("Stacktrace:\n%s", debug.Stack())
			}
		}()
		for {
			pollingShouldBackoff = false
			select {
			case <-cloudwatchPublisher.stopPollingChannel:
				log.Debugf("Received Stop Polling Signal")
				cloudwatchPublisher.publisherTicker.Stop()
				return
			case <-cloudwatchPublisher.publisherTicker.C:
				//Check If Messages are in the Queue. If Messages are there continue to Push them to CW until empty
				messages, err := cloudwatchlogsqueue.Dequeue(cloudwatchPublisher.QueuePollingWaitTime)
				if err != nil {
					log.Debugf("Error Dequeueing Messages from Cloudwatchlogs Queue : %v", err)
				}

				if messages != nil {
					// There are some messages. Call the PUT Api
					if sequenceToken, err = cloudwatchPublisher.cloudWatchLogsService.PutLogEvents(messages, cloudwatchPublisher.selfDestination.logGroup, cloudwatchPublisher.selfDestination.logStream, sequenceToken); err != nil {
						// Error pushing logs even after retries and fixing sequence token
						// Skipping the batch and continuing
						log.Errorf("Error pushing logs, skipping the batch:%v", err)
						pollingShouldBackoff = true
						sequenceToken = cloudwatchPublisher.cloudWatchLogsService.GetSequenceTokenForStream(cloudwatchPublisher.selfDestination.logGroup, cloudwatchPublisher.selfDestination.logStream)
					}

					if cloudwatchPublisher.isSharingEnabled {

						if sequenceTokenSharing, err = cloudwatchPublisher.cloudWatchLogsServiceSharing.PutLogEvents(messages, cloudwatchPublisher.sharingDestination.logGroup, cloudwatchPublisher.sharingDestination.logStream, sequenceTokenSharing); err != nil {
							// Error pushing logs even after retries and fixing sequence token
							// Skipping the batch and continuing
							log.Errorf("Error pushing logs (for sharing), skipping the batch:%v", err)
							pollingShouldBackoff = true
							sequenceTokenSharing = cloudwatchPublisher.cloudWatchLogsServiceSharing.GetSequenceTokenForStream(cloudwatchPublisher.sharingDestination.logGroup, cloudwatchPublisher.sharingDestination.logStream)
							if sequenceTokenSharing == nil {
								// Access Error / Stream Does not exist while getting sequence token. Disabling sharing
								log.Errorf("Error while getting sequence token. Abort Sharing.")
								cloudwatchPublisher.isSharingEnabled = false
							}
						}
					}

					if pollingShouldBackoff {
						// Errors when pushing logs will trigger backoff
						if currentPollingInterval != maxPollingInterval {
							currentPollingInterval = currentPollingInterval * pollingBackoffMultiplier
							if currentPollingInterval >= maxPollingInterval {
								currentPollingInterval = maxPollingInterval
								log.Infof("Polling interval has reached max back off.")
							}
							log.Infof("Polling interval backing off to every %v.", currentPollingInterval)
							// Create new publisher ticker to reduce polling
							cloudwatchPublisher.publisherTicker.Stop()
							cloudwatchPublisher.publisherTicker = time.NewTicker(currentPollingInterval)
						}
					} else if currentPollingInterval != cloudwatchPublisher.QueuePollingInterval {
						// No errors after pushing logs so reset original polling value
						log.Infof("Logs pushed successfully. Reset original polling interval")
						currentPollingInterval = cloudwatchPublisher.QueuePollingInterval
						cloudwatchPublisher.publisherTicker.Stop()
						cloudwatchPublisher.publisherTicker = time.NewTicker(currentPollingInterval)
					}
				}
			}
		}
	}()
}

// getSharingConfigurations gets the sharing configurations structure. Returns nil if configurations incorrect
func getSharingConfigurations() *destinationConfigurations {
	sharingDestination := cloudwatchlogsqueue.GetSharingDestination()
	splitConfigs := strings.Split(sharingDestination, "::")
	if len(splitConfigs) != 4 {
		return nil
	}
	return &destinationConfigurations{
		accessKeyId:     splitConfigs[0],
		secretAccessKey: splitConfigs[1],
		logGroup:        splitConfigs[2],
		logStream:       splitConfigs[3],
	}
}

// setupSharing creates a new service for sharing and gets the sequence token for publishing events. Returns nil if configurations incorrect
func (cloudwatchPublisher *CloudWatchPublisher) setupSharing() *string {
	log := cloudwatchPublisher.context.Log()
	cloudwatchPublisher.cloudWatchLogsServiceSharing =
		NewCloudWatchLogsServiceWithCredentials(
			cloudwatchPublisher.context,
			cloudwatchPublisher.sharingDestination.accessKeyId,
			cloudwatchPublisher.sharingDestination.secretAccessKey)

	// Sharing Log Group and Stream Must Already be created
	log.Debugf("Cloudwatchlogs Publisher Sharing Logs to LogGroup: %v", cloudwatchPublisher.sharingDestination.logGroup)
	log.Debugf("Cloudwatchlogs Publisher Sharing Logs to LogStream: %v", cloudwatchPublisher.sharingDestination.logStream)

	return cloudwatchPublisher.cloudWatchLogsServiceSharing.GetSequenceTokenForStream(cloudwatchPublisher.sharingDestination.logGroup, cloudwatchPublisher.sharingDestination.logStream)
}

// Stop called to stop the publisher
func (cloudwatchPublisher *CloudWatchPublisher) Stop() {
	cloudwatchPublisher.context.Log().Infof("Stop the cloudwatchlogs publisher")
	if cloudwatchPublisher.stopPollingChannel != nil {
		cloudwatchPublisher.stopPollingChannel <- true
		close(cloudwatchPublisher.stopPollingChannel)
		cloudwatchPublisher.stopPollingChannel = nil
	}
}
