package main

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/proc"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/stretchr/testify/assert"
)

func TestParseArgs(t *testing.T) {
	input := []string{"ssm-document-worker", "documentID", "instanceID"}
	channelName, instanceID, err := proc.ParseArgv(input)
	assert.NoError(t, err)
	assert.Equal(t, "documentID", channelName)
	assert.Equal(t, "instanceID", instanceID)
}

func TestWorkerInitializeLightWeight(t *testing.T) {
	ctxLight, name, err := initialize([]string{"test_binary", "documentID", "instanceID"})
	assert.NoError(t, err)
	assert.Equal(t, "documentID", name)
	instanceID, err := platform.InstanceID()
	assert.NoError(t, err)
	assert.Equal(t, "instanceID", instanceID)
	assert.Equal(t, ctxLight.CurrentContext(), []string{defaultWorkerContextName, "[" + name + "]"})
}
