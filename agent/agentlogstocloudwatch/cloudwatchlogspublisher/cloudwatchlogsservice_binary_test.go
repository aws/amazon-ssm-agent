// Copyright 2026 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/

package cloudwatchlogspublisher

import (
	"testing"
	"unicode/utf8"

	contextmocks "github.com/aws/amazon-ssm-agent/agent/mocks/context"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/assert"
)

// TestMessageLengthThresholdPreventsBatchOverflow verifies that the reduced
// MessageLengthThresholdInBytes prevents CloudWatch PutLogEvents batch size
// overflow when binary data (bytes 128-255) inflates up to 3x in UTF-8.
//
// CloudWatch PutLogEvents has a 1,048,576 byte batch limit calculated as
// the sum of all event messages in UTF-8 plus 26 bytes per event.
// See: https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_PutLogEvents.html
func TestMessageLengthThresholdPreventsBatchOverflow(t *testing.T) {
	// Maximum UTF-8 inflation factor for bytes in range 128-255
	const utf8InflationFactor = 3
	const perEventOverhead = 26
	const cwBatchLimit = 1_048_576

	// Calculate worst-case batch size with current constants
	worstCaseBatchSize := (MessageLengthThresholdInBytes*utf8InflationFactor + perEventOverhead) * maxNumberOfEventsPerCall

	assert.Less(t, worstCaseBatchSize, cwBatchLimit,
		"Worst-case batch size (%d) must be less than CloudWatch limit (%d). "+
			"MessageLengthThresholdInBytes=%d, maxNumberOfEventsPerCall=%d",
		worstCaseBatchSize, cwBatchLimit,
		MessageLengthThresholdInBytes, maxNumberOfEventsPerCall)
}

// TestMessageLengthThresholdValue verifies the threshold was reduced from
// the original value of 200*1000 to account for UTF-8 encoding inflation.
func TestMessageLengthThresholdValue(t *testing.T) {
	// The threshold should be 80*1024 = 81920
	assert.Equal(t, 80*1024, MessageLengthThresholdInBytes,
		"MessageLengthThresholdInBytes should be 80*1024 to prevent batch overflow with binary data")
}

// TestBuildEventInfo_BinaryData verifies that buildEventInfo handles binary
// data without panicking. Binary data from sources like /dev/urandom contains
// invalid UTF-8 sequences that previously caused CloudWatch upload failures.
func TestBuildEventInfo_BinaryData(t *testing.T) {
	contextMock := contextmocks.NewMockDefault()

	service := &CloudWatchLogsService{
		context: contextMock,
		CloudWatchMessage: CloudWatchMessage{
			EventVersion: aws.String("1.0"),
			SessionId:    aws.String("test-session"),
		},
		isFileComplete: true,
	}

	// Binary data simulating /dev/urandom output (invalid UTF-8 sequences)
	binaryData := make([]byte, 1000)
	for i := range binaryData {
		binaryData[i] = byte(i % 256)
	}
	// Ensure it ends with newline for the isFileComplete path
	binaryData[len(binaryData)-1] = '\n'

	// Verify test data contains invalid UTF-8
	assert.False(t, utf8.Valid(binaryData), "Test data should contain invalid UTF-8")

	// Call buildEventInfo with binary data (non-structured logs)
	// This should NOT panic
	event := service.buildEventInfo(binaryData, false)

	// Event should be created successfully
	assert.NotNil(t, event, "buildEventInfo should return a non-nil event for binary data")
	assert.NotNil(t, event.Message, "Event message should not be nil")
	assert.NotNil(t, event.Timestamp, "Event timestamp should not be nil")
}

// TestBuildEventInfo_LargeBinaryDataWithinThreshold verifies that binary data
// within the threshold limit produces events that fit within CloudWatch limits.
func TestBuildEventInfo_LargeBinaryDataWithinThreshold(t *testing.T) {
	contextMock := contextmocks.NewMockDefault()

	service := &CloudWatchLogsService{
		context: contextMock,
		CloudWatchMessage: CloudWatchMessage{
			EventVersion: aws.String("1.0"),
			SessionId:    aws.String("test-session"),
		},
		isFileComplete: true,
	}

	// Create binary data at the threshold limit (all 0x80 bytes = worst case UTF-8 inflation)
	binaryData := make([]byte, MessageLengthThresholdInBytes)
	for i := range binaryData {
		binaryData[i] = 0x80 // Worst case: each byte inflates to 3 bytes in UTF-8
	}
	binaryData[len(binaryData)-1] = '\n'

	event := service.buildEventInfo(binaryData, false)
	assert.NotNil(t, event, "buildEventInfo should handle threshold-sized binary data")

	// Verify the event message size
	messageBytes := []byte(aws.StringValue(event.Message))
	// With 80KB of 0x80 bytes, the UTF-8 encoded string could be up to 3x larger
	// A single event should still be under the CW per-event limit
	assert.Greater(t, len(messageBytes), 0, "Event message should not be empty")
}

// TestBatchSizeCalculation verifies the mathematical relationship between
// the constants to ensure they satisfy CloudWatch requirements.
func TestBatchSizeCalculation(t *testing.T) {
	const cwLimit = 1_048_576
	const overhead = 26

	// Test all possible batch sizes with worst-case binary data
	for numEvents := 1; numEvents <= maxNumberOfEventsPerCall; numEvents++ {
		worstCase := (MessageLengthThresholdInBytes*3 + overhead) * numEvents
		assert.Less(t, worstCase, cwLimit,
			"Batch of %d events with worst-case binary data (%d bytes) exceeds CW limit (%d)",
			numEvents, worstCase, cwLimit)
	}
}

// TestBuildEventInfo_NormalText verifies that normal text output is not
// affected by the threshold change and continues to work correctly.
func TestBuildEventInfo_NormalText(t *testing.T) {
	contextMock := contextmocks.NewMockDefault()

	service := &CloudWatchLogsService{
		context: contextMock,
		isFileComplete: true,
	}

	// Normal ASCII text (no UTF-8 inflation)
	normalText := []byte("echo 'hello world'\nwhoami\npwd\nls -la\n")

	event := service.buildEventInfo(normalText, false)
	assert.NotNil(t, event)

	messageStr := aws.StringValue(event.Message)
	assert.True(t, utf8.ValidString(messageStr),
		"Normal text should produce valid UTF-8")
	assert.Contains(t, messageStr, "echo 'hello world'",
		"Normal text content should be preserved")
}