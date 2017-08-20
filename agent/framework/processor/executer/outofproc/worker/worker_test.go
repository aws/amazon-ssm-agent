package main

import (
	"testing"

	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/messaging"
	"github.com/stretchr/testify/assert"
)

func TestParseArgs(t *testing.T) {
	input := []string{"ssm-document-worker", "documentID"}
	name, channelName, err := messaging.ParseArgv(input)
	assert.NoError(t, err)
	assert.Equal(t, "ssm-document-worker", name)
	assert.Equal(t, "documentID", channelName)
}

func TestCreateContextLightWeight(t *testing.T) {
	ctxLight := createContext("test")
	fmt.Println(ctxLight)
}
