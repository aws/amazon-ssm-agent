package main

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/proc"
	"github.com/stretchr/testify/assert"
)

func TestParseArgs(t *testing.T) {
	input := []string{appconfig.DefaultDocumentWorker, "documentID"}
	channelName, err := proc.ParseArgv(input)
	assert.NoError(t, err)
	assert.Equal(t, "documentID", channelName)
}

func TestWorkerInitializeLightWeight(t *testing.T) {
	_, _, err := initialize([]string{"test_binary", "documentID"})
	assert.Error(t, err)
}
