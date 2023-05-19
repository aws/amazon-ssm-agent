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

// cloudwatchlogsqueue queues up agent's context event log, to be consumed by the CloudWatchLogs publisher

package cloudwatchlogsqueue

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/Workiva/go-datastructures/queue"
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/cihub/seelog"
)

const (
	batchSize            int   = 10000   // The Max Batch Supported by the AWS CW Logs Push API
	batchByteSizeMax     int   = 1000000 // CloudWatch batch size - 1 MB
	initialQueueCapacity int64 = 10      // The initial capacity of slice. Would not need to resize till this length
	queueLimit           int64 = 10000   // The Limit of the number of messages in the queue (~40kB of queue)
	defaultLogGroup            = "SSMAgentLogs"

	docWorkerLogGroupSeelogAttrib     = "document-worker-log-group"
	sessionWorkerLogGroupSeelogAttrib = "session-worker-log-group"
	agentWorkerLogGroupSeelogAttrib   = "log-group"
)

// logDataFacade stores the CloudWatchLogs Destination and Queue being used to store the messages
type logDataFacade struct {
	logGroup           string
	logSharingEnabled  bool
	sharingDestination string
	messageQueue       *queue.Queue // Access to message queue is restricted from the facade
}

// CloudWatchLogsEvents event codes for changes in cloudwatchlogs publishing
type CloudWatchLogsEvents int

const (
	QueueActivated            = iota //On Queue Activation
	QueueDeactivated                 // On Queue Deactivation
	LoggingDestinationChanged        // On Change in logging destination
)

var logDataFacadeInstance *logDataFacade
var once = new(sync.Once)
var mutex sync.RWMutex
var verifiedLogGroupName string

// CloudWatchLogsEventsChannel channel used for communication with cloudwatch publisher
var CloudWatchLogsEventsChannel = make(chan CloudWatchLogsEvents)

// CreateCloudWatchDataInstance creates an instance of logDataFacade if not created
func CreateCloudWatchDataInstance(initArgs seelog.CustomReceiverInitArgs) (err error) {
	// Acquiring Read Write Lock on the instance to ensure enqueue/dequeue not happening
	fmt.Println("Create Instance")
	mutex.Lock()
	defer mutex.Unlock()
	if err := verifyLogGroupName(initArgs, os.Args); err != nil {
		return err
	}
	// Ensuring just one instance is created. Returning the same instance if already created
	once.Do(func() {
		defer func() {
			// In case the creation panics,
			if msg := recover(); msg != nil {
				fmt.Printf("Create CloudWatchLogs Data Facade Instance Failed: %v", msg)
				// Allow Creation of another instance
				once = new(sync.Once)
				err = errors.New("Create CloudWatchLogs Data Facade Instance Failed")
			}
		}()

		logDataFacadeInstance = &logDataFacade{}

		// Create the queue for placing log messages in it
		createQueue()
	})
	// Ensuring an instance is present, else erroring out
	if !IsActive() {
		return errors.New("CloudWatchLogs Data Facade Instance Not Active. Create Failed.")
	}
	setLogDestination(initArgs)
	return
}

// setLogDestination updates the logGroup if needed
func setLogDestination(initArgs seelog.CustomReceiverInitArgs) {
	logGroup, sharingDestination, logSharingEnabled := parseXMLConfigs(initArgs)
	if logDataFacadeInstance.logGroup == logGroup && logDataFacadeInstance.logSharingEnabled == logSharingEnabled && logDataFacadeInstance.sharingDestination == sharingDestination {
		return
	}

	fmt.Println("Logging to LogGroup:", logGroup)
	fmt.Println("Log Sharing:", logSharingEnabled)

	logDataFacadeInstance.logGroup = logGroup
	logDataFacadeInstance.logSharingEnabled = logSharingEnabled
	logDataFacadeInstance.sharingDestination = sharingDestination

	// Signal the publisher that there has been a change in destination in non-blocking way
	select {
	case CloudWatchLogsEventsChannel <- LoggingDestinationChanged:
		fmt.Println("Logging Destination Change. Signalled Publisher Successfully")
	default:
		fmt.Println("Logging Destination Change. Publisher not active")
	}
}

func verifyLogGroupName(xmlConfig seelog.CustomReceiverInitArgs, args []string) error {
	invalidLogGroupErrMsg := "invalid log group name"
	var logGroup = defaultLogGroup
	var ok bool
	if len(args) > 0 {
		if args[0] == appconfig.DefaultDocumentWorker {
			logGroup, ok = xmlConfig.XmlCustomAttrs[docWorkerLogGroupSeelogAttrib]
			if logGroup == "" || !ok {
				return fmt.Errorf(invalidLogGroupErrMsg)
			}
		} else if args[0] == appconfig.DefaultSessionWorker {
			logGroup, ok = xmlConfig.XmlCustomAttrs[sessionWorkerLogGroupSeelogAttrib]
			if logGroup == "" || !ok {
				return fmt.Errorf(invalidLogGroupErrMsg)
			}
		} else {
			logGroup, ok = xmlConfig.XmlCustomAttrs[agentWorkerLogGroupSeelogAttrib]
			if logGroup == "" || !ok {
				fmt.Println("No Log Group in Config. Will log in default group")
				logGroup = defaultLogGroup
			}
		}
	}
	verifiedLogGroupName = logGroup
	return nil
}

// parseXMLConfigs parses the logGroup from seelog config
func parseXMLConfigs(xmlConfig seelog.CustomReceiverInitArgs) (logGroup, sharingDestination string, logSharingEnabled bool) {
	// Getting the log group from seelog config

	var err error
	logSharingEnabledParam, ok := xmlConfig.XmlCustomAttrs["log-sharing-enabled"]
	if !ok {
		fmt.Println("No Sharing parameter in XML. Assuming sharing false")
		logSharingEnabled = false
	} else {
		logSharingEnabled, err = strconv.ParseBool(logSharingEnabledParam)
		if err != nil {
			fmt.Printf("Incorrect format of log-sharing-enabled : %v", err)
			logSharingEnabled = false
		}
	}

	if logSharingEnabled {
		sharingDestination, ok = xmlConfig.XmlCustomAttrs["sharing-destination"]
		if !ok {
			fmt.Println("No Sharing Destination in XML. Turning Sharing Off")
			logSharingEnabled = false
		}
	}

	return verifiedLogGroupName, sharingDestination, logSharingEnabled
}

// Dequeue Returns the batch of messages present in the queue. Returns nil if no messages or no queue present
func Dequeue(pollingWaitTime time.Duration) ([]*cloudwatchlogs.InputLogEvent, error) {
	// Acquiring Read Lock on the instance to allow multiple enqueuers/dequeuers to access queue
	mutex.RLock()
	defer mutex.RUnlock()

	// Dequeue Message if queue present
	if IsActive() {
		messages := make([]*cloudwatchlogs.InputLogEvent, 0, 10)
		cwCurrentBatchSize := 0
		for i := 0; i < batchSize; i++ {
			cwEvent, err := logDataFacadeInstance.messageQueue.Peek()
			if err == nil {
				if message, ok := cwEvent.(*cloudwatchlogs.InputLogEvent); ok {
					if messageByte, marshallErr := json.Marshal(message); marshallErr == nil {
						cwCurrentBatchSize += len(messageByte)
						if cwCurrentBatchSize > batchByteSizeMax {
							err = fmt.Errorf("cw batch byte size exceeded the limit")
						}
					}
				}
			}

			if err != nil {
				if err == queue.ErrEmptyQueue {
					return messages, nil
				}
				return messages, err
			}

			genericMessages, err := logDataFacadeInstance.messageQueue.Poll(1, pollingWaitTime)

			if err != nil {
				if err == queue.ErrTimeout {
					// There were no messages in queue even after wait time
					return messages, nil
				}
				return messages, err
			}

			// Returning nil if no messages in queue
			if len(genericMessages) == 0 {
				return nil, nil
			}

			for i := range genericMessages {
				// Safe type conversion from interface{} to *cloudwatchlogs.InputLogEvent
				if message, ok := genericMessages[i].(*cloudwatchlogs.InputLogEvent); ok {
					messages = append(messages, message)
				}
			}
		}
		return messages, nil
	}
	return nil, errors.New("CloudWatchLogs Queue not initialized or destroyed on Dequeue")
}

// GetLogGroup returns the log group intended for logging
func GetLogGroup() string {
	return logDataFacadeInstance.logGroup
}

// IsLogSharingEnabled returns true if log sharing is enabled
func IsLogSharingEnabled() bool {
	return logDataFacadeInstance.logSharingEnabled
}

// GetSharingDestination returns the destination for sharing
func GetSharingDestination() string {
	return logDataFacadeInstance.sharingDestination
}

// Enqueue to add message to queue
func Enqueue(message *cloudwatchlogs.InputLogEvent) error {
	// skip log group when log group or logGroup is not set
	if logDataFacadeInstance.logGroup == "" {
		if logDataFacadeInstance.sharingDestination == "" || !logDataFacadeInstance.logSharingEnabled {
			return fmt.Errorf("log group not found")
		}
	}
	// Acquiring Read Lock on the instance to allow multiple enquequers/dequeuers to access queue
	mutex.RLock()
	defer mutex.RUnlock()
	// Enqueue if the queue is present
	if IsActive() {
		if logDataFacadeInstance.messageQueue.Len() < queueLimit {
			return logDataFacadeInstance.messageQueue.Put(message)
		}
		return errors.New("CloudWatchLogs Queue Overflow. Enqueue failed")
	}
	return errors.New("CloudWatchLogs Queue not initialized or destroyed on Enqueue")
}

// createQueue creates a cloudwatchlogs queue
func createQueue() {
	logDataFacadeInstance.messageQueue = queue.New(initialQueueCapacity)
}

// DestroyCloudWatchDataInstance to clear the memory of queue and enable new instance creation
func DestroyCloudWatchDataInstance() {
	// Signal the publisher that there has been a change in destination in non-blocking way
	select {
	case CloudWatchLogsEventsChannel <- QueueDeactivated:
		fmt.Println("Queue Deactivated. Signalled Publisher Successfully")
	default:
		fmt.Println("Queue Deactivated. Publisher not active")
	}
	// Acquiring Read Write Lock on the instance. Will wait until all  running Enqueues/Dequeues have completed
	mutex.Lock()
	defer mutex.Unlock()
	if IsActive() {
		// Dispose the queue
		logDataFacadeInstance.messageQueue.Dispose()
		// Discard the old instance
		logDataFacadeInstance = nil
	}
	// Allow the creation of new instance
	once = new(sync.Once)
}

// IsActive returns true if the queue is active
func IsActive() bool {
	return logDataFacadeInstance != nil && logDataFacadeInstance.messageQueue != nil
}
