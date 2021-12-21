package utils

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/stretchr/testify/assert"
)

func TestLoadProcessorWorkerConfig(t *testing.T) {
	mockContext := context.NewMockDefault()

	expectedConfig := make(map[WorkerName]*ProcessorWorkerConfig)
	appConfigVal := mockContext.AppConfig()
	docProcWorkConfig := &ProcessorWorkerConfig{
		ProcessorName:           CommandProcessor,
		WorkerName:              DocumentWorkerName,
		StartWorkerLimit:        appConfigVal.Mds.CommandWorkersLimit,
		CancelWorkerLimit:       appconfig.DefaultCancelWorkersLimit,
		StartWorkerDocType:      contracts.SendCommand,
		CancelWorkerDocType:     contracts.CancelCommand,
		StartWorkerBufferLimit:  appConfigVal.Mds.CommandWorkerBufferLimit, // provides buffer limit equivalent to worker limit
		CancelWorkerBufferLimit: 1,
	}

	sessionProcWorkConfig := &ProcessorWorkerConfig{
		ProcessorName:           SessionProcessor,
		WorkerName:              SessionWorkerName,
		StartWorkerLimit:        appConfigVal.Mgs.SessionWorkersLimit,
		CancelWorkerLimit:       appconfig.DefaultCancelWorkersLimit,
		StartWorkerDocType:      contracts.StartSession,
		CancelWorkerDocType:     contracts.TerminateSession,
		StartWorkerBufferLimit:  appConfigVal.Mgs.SessionWorkerBufferLimit, // providing just 1 as buffer
		CancelWorkerBufferLimit: 1,                                         // providing just 1 as buffer
	}
	expectedConfig[docProcWorkConfig.WorkerName] = docProcWorkConfig
	expectedConfig[sessionProcWorkConfig.WorkerName] = sessionProcWorkConfig

	actualConfig := LoadProcessorWorkerConfig(mockContext)

	assert.Equal(t, expectedConfig, actualConfig)

}
