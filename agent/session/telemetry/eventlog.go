// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package telemetry is used to schedule and send the audit logs to MGS
package telemetry

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math/rand"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/session/communicator"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	"github.com/aws/amazon-ssm-agent/agent/session/telemetry/metrics"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/carlescere/scheduler"
	"github.com/gorilla/websocket"
	"github.com/twinj/uuid"
)

const (
	EventLogDelayFactor   = 12600 // 12600 seconds - 3.5 hrs
	EventLogDelayBase     = 900   // 900 seconds
	EventLogFreqHrs       = 5     // 5 hrs
	MGSJitterMilliSeconds = 4000  // 4 seconds
)

// mu used to lock the send health message process.
var mu = &sync.Mutex{}

// AgentTelemetry is the agent health message format being used as payload for MGS message
type AgentTelemetry struct {
	SchemaVersion           int    `json:"SchemaVersion"`
	NumberOfAgentReboot     int    `json:"NumberOfAgentReboot"`
	NumberOfSSMWorkerReboot int    `json:"NumberOfSSMWorkerReboot"`
	AgentVersion            string `json:"AgentVersion"`
}

// AgentUpdateCodes is the agent health message format being used as payload for MGS message
type AgentUpdateResultDiagnosis struct {
	SchemaVersion         int    `json:"SchemaVersion"`
	AgentUpdateResultCode string `json:"AgentUpdateResultCode"`
	SourceVersion         string `json:"SourceVersion"`
	TargetVersion         string `json:"TargetVersion"`
}

// IAuditLogTelemetry is the scheduler used for the AuditLogScheduler
type IAuditLogTelemetry interface {
	ScheduleAuditEvents()
	SendAuditMessage()
	StopScheduler()
}

var auditLogTelemetryInstance *AuditLogTelemetry

// AuditLogTelemetry helps us in scheduling the process to send audit logs to MGS
type AuditLogTelemetry struct {
	channel                       communicator.IWebSocketChannel
	cloudWatchService             metrics.ICloudWatchService
	ctx                           context.T
	auditSchedulerJob             *scheduler.Job
	auditSchedulerTimer           chan bool
	eventLogDelayFactor           int
	isMGSTelemetryTransportEnable bool
	eventLogDelayBase             int
	frequency                     int
	mgsDelay                      int
}

// GetAuditLogTelemetryInstance returns us the singleton instance of AuditLogTelemetry
func GetAuditLogTelemetryInstance(ctx context.T, channel communicator.IWebSocketChannel) *AuditLogTelemetry {
	if auditLogTelemetryInstance != nil {
		return auditLogTelemetryInstance
	}

	// to bring extra randomness
	instanceId, _ := ctx.Identity().InstanceID()
	hash := fnv.New32a()
	hash.Write([]byte(instanceId))
	rand.Seed(time.Now().UTC().UnixNano() + int64(hash.Sum32()))

	auditLogTelemetryInstance = &AuditLogTelemetry{
		channel:                       channel,
		ctx:                           ctx,
		auditSchedulerTimer:           make(chan bool, 1),
		eventLogDelayFactor:           EventLogDelayFactor,
		eventLogDelayBase:             EventLogDelayBase,
		frequency:                     EventLogFreqHrs,
		mgsDelay:                      MGSJitterMilliSeconds,
		cloudWatchService:             metrics.NewCloudWatchService(ctx),
		isMGSTelemetryTransportEnable: ctx.AppConfig().Agent.TelemetryMetricsToSSM,
	}
	return auditLogTelemetryInstance
}

// ScheduleAuditEvents sets up the scheduler
func (a *AuditLogTelemetry) ScheduleAuditEvents() {
	log := a.ctx.Log()
	if a.isTelemetryEnabled() {
		a.setupScheduler()
	} else {
		log.Info("agent telemetry metrics disabled")
	}
}

// SendAuditMessage triggers the send process in a separate go routine
func (a *AuditLogTelemetry) SendAuditMessage() {
	if a.isTelemetryEnabled() {
		go a.sendHealthInfoWithDelay()
	}
}

// StopScheduler stops the scheduler
func (a *AuditLogTelemetry) StopScheduler() {
	if a.auditSchedulerJob != nil {
		a.auditSchedulerJob.Quit <- true
		a.auditSchedulerTimer <- true
	}
}

// setupScheduler sets up the scheduler and triggers every 1 hour.
// It only checks the last line of the file when gathering the count
func (a *AuditLogTelemetry) setupScheduler() {
	log := a.ctx.Log()
	log.Info("Setting up agent telemetry scheduler")
	if !a.isMGSTelemetryTransportEnable {
		log.Info("agent telemetry metrics to MGS disabled")
	}
	var err error

	if a.auditSchedulerJob, err = scheduler.Every(a.frequency).NotImmediately().Hours().Run(func() {
		a.sendHealthInfoWithDelay()
	}); err != nil {
		log.Errorf("Unable to schedule agent health audit process. %v", err)
	}
}

// isTelemetryEnabled verifies whether the agent telemetry is completely enabled or not
func (a *AuditLogTelemetry) isTelemetryEnabled() bool {
	// cloud watch or mgs transport enabled here
	return a.cloudWatchService.IsCloudWatchEnabled() || a.isMGSTelemetryTransportEnable
}

// sendHealthWithDelay send the AgentTelemetry message to MGS with a delay added to it
func (a *AuditLogTelemetry) sendHealthInfoWithDelay() {
	nextTrigger := time.Duration(a.eventLogDelayBase+rand.Intn(a.eventLogDelayFactor)) * time.Second
	select {
	case <-time.After(nextTrigger):
		a.sendAgentHealthMessage()
	case <-a.auditSchedulerTimer:
		return
	}
}

// sendAgentHealthMessage send the AgentTelemetry message to MGS
// One solution to send audit files for shorter duration is to split the file while rolling in a day
func (a *AuditLogTelemetry) sendAgentHealthMessage() {
	log := a.ctx.Log()
	defer func() {
		if err := recover(); err != nil {
			log.Errorf("Agent Telemetry panicked with: %v", err)
			log.Errorf("Stacktrace:\n%s", debug.Stack())
		}
	}()
	mu.Lock()
	defer mu.Unlock()
	eventCounts, err := logger.GetEventCounter()
	if err != nil {
		log.Errorf("Unable to get event counters from audit file. %v", err)
	}
	for i := len(eventCounts) - 1; i >= 0; i-- {
		var err error
		//default is not added as we are logging the bytes read even though we have wrong message type
		switch eventCounts[i].EventChunkType {
		case logger.AgentTelemetryMessage:
			err = a.sendBasicAgentTelemetryMessage(eventCounts[i])
		case logger.AgentUpdateResultMessage:
			err = a.sendAgentUpdateResultMessage(eventCounts[i])
		}

		if err != nil {
			log.Errorf("unexpected error while sending message to MGS: %s", err)
			return
		}
		// this will update the file even when TelemetryMetricsToSSM is false
		// had to follow this behavior because of common tracker(AuditSentSuccessFooter in the file) for CloudWatch and MGS
		if err = logger.WriteLastLineFile(eventCounts[i]); err != nil {
			log.Errorf("Unable to update success status in audit file: %s", err)
			return // This will stop the iteration and try to send the health data during next schedule
		} else {
			log.Infof(
				"Successfully sent Agent health message for the date %v and last sent time %v",
				eventCounts[i].AuditDate,
				eventCounts[i].LastReadTime)
		}
		time.Sleep(time.Duration(rand.Intn(a.mgsDelay)) * time.Millisecond)
	}
}

func (a *AuditLogTelemetry) sendAgentUpdateResultMessage(eventCount *logger.EventCounter) (err error) {
	log := a.ctx.Log()
	schemaVal, _ := strconv.Atoi(eventCount.SchemaVersion)
	for updateEvent, val := range eventCount.CountMap { // will contain only one update event
		updateCodeWithTargetVersion := strings.Split(updateEvent, "-")
		targetVersion := ""
		if len(updateCodeWithTargetVersion) > 1 {
			targetVersion = updateCodeWithTargetVersion[1]
		}
		sourceVersion := eventCount.AgentVersion

		if targetVersion != "" {
			if matched, regexErr := regexp.MatchString(logger.VersionRegexPattern, targetVersion); matched == false || regexErr != nil {
				log.Warnf("invalid agent version: %s", targetVersion)
				return
			}
		}
		agentUpdateResultJson := AgentUpdateResultDiagnosis{
			SchemaVersion:         schemaVal,
			AgentUpdateResultCode: updateCodeWithTargetVersion[0],
			SourceVersion:         sourceVersion,
			TargetVersion:         targetVersion,
		}
		if a.isMGSTelemetryTransportEnable {
			auditBytes, err := json.Marshal(agentUpdateResultJson)
			if err != nil { // return error only when telemetry to MGS is enabled
				return fmt.Errorf("unable to marshal agent update result payload to json string: %s, err: %s", auditBytes, err)
			}
			if err = a.sendChannelContract(auditBytes, logger.AgentUpdateResultMessage); err != nil {
				return fmt.Errorf("unable to send agent update result message to MGS: %s", err)
			}
		}

		var MetricData []*cloudwatch.MetricDatum
		if targetVersion != "" {
			// for normal update events
			MetricData = append(MetricData, a.cloudWatchService.GenerateUpdateMetrics(
				updateCodeWithTargetVersion[0], // Update code without target version
				float64(val),
				agentUpdateResultJson.SourceVersion,
				targetVersion))
		} else {
			// for self update events
			MetricData = append(MetricData, a.cloudWatchService.GenerateBasicTelemetryMetrics(
				updateCodeWithTargetVersion[0], // Update code without target version
				float64(val),
				agentUpdateResultJson.SourceVersion))
		}

		if err = a.cloudWatchService.PutMetrics(MetricData); err == nil {
			log.Infof("Successfully sent Agent Update Result message to CloudWatch")
		}
	}
	return nil
}

func (a *AuditLogTelemetry) sendBasicAgentTelemetryMessage(eventCount *logger.EventCounter) (err error) {
	log := a.ctx.Log()
	schemaVal, _ := strconv.Atoi(eventCount.SchemaVersion)
	startEventCount, workerStartEventCount := eventCount.CountMap[logger.AmazonAgentStartEvent], eventCount.CountMap[logger.AmazonAgentWorkerStartEvent]
	if startEventCount+workerStartEventCount == 0 {
		log.Warnf("wrong event type mapped to the event log")
		return
	}
	agentHealthJson := AgentTelemetry{
		NumberOfAgentReboot:     startEventCount,
		NumberOfSSMWorkerReboot: workerStartEventCount,
		SchemaVersion:           schemaVal,
		AgentVersion:            eventCount.AgentVersion,
	}
	if a.isMGSTelemetryTransportEnable {
		auditBytes, err := json.Marshal(agentHealthJson)
		if err != nil { // return error only when telemetry to MGS is enabled
			return fmt.Errorf("unable to marshal Agent Audit Reboot Log payload to json string: %s, err: %s", auditBytes, err)
		}
		if err = a.sendChannelContract(auditBytes, logger.AgentTelemetryMessage); err != nil {
			return fmt.Errorf("unable to send message to MGS: %s", err)
		}
	}

	var MetricData []*cloudwatch.MetricDatum
	MetricData = append(
		MetricData,
		a.cloudWatchService.GenerateBasicTelemetryMetrics(
			logger.AmazonAgentStartEvent,
			float64(agentHealthJson.NumberOfAgentReboot),
			agentHealthJson.AgentVersion))
	MetricData = append(
		MetricData,
		a.cloudWatchService.GenerateBasicTelemetryMetrics(
			logger.AmazonAgentWorkerStartEvent,
			float64(agentHealthJson.NumberOfSSMWorkerReboot),
			agentHealthJson.AgentVersion))

	if err = a.cloudWatchService.PutMetrics(MetricData); err == nil {
		log.Infof("Successfully sent Agent health message to CloudWatch")
	}
	return nil
}

// sendChannelContract send through the web socket connection with necessary packaging
func (a *AuditLogTelemetry) sendChannelContract(payload []byte, messageType string) error {
	// blocks sending metrics to MGS
	if !a.isMGSTelemetryTransportEnable {
		return fmt.Errorf("agent telemetry metrics to MGS disabled")
	}
	log := a.ctx.Log()
	agentMessage := &mgsContracts.AgentMessage{
		MessageType:    messageType,
		MessageId:      uuid.NewV4(),
		SchemaVersion:  1,
		CreatedDate:    uint64(time.Now().UnixNano() / 1000000),
		SequenceNumber: 0,
		Flags:          0,
		Payload:        payload,
	}
	log.Info("Sending log to MGS: ", jsonutil.Indent(string(payload)))
	agentBytes, err := agentMessage.Serialize(log)
	if err != nil {
		return err
	}
	return a.channel.SendMessage(log, agentBytes, websocket.BinaryMessage)
}
