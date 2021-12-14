package replytypes

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/stretchr/testify/assert"
	"github.com/twinj/uuid"
)

func TestGetReplyTypeObject_RunCommandResult(t *testing.T) {
	res := contracts.DocumentResult{ResultType: contracts.RunCommandResult}
	ctx := context.NewMockDefault()
	uuid := uuid.NewV4()
	replyType, err := GetReplyTypeObject(ctx, res, uuid, 0)
	expectedReply := NewAgentRunCommandReplyType(ctx, res, uuid, 0)
	assert.Nil(t, err)
	assert.Equal(t, expectedReply, replyType)
}

func TestGetReplyType_SessionResult(t *testing.T) {
	res := contracts.DocumentResult{ResultType: contracts.SessionResult}
	ctx := context.NewMockDefault()
	uuid := uuid.NewV4()
	replyType, err := GetReplyTypeObject(ctx, res, uuid, 0)
	expectedReply := NewSessionCompleteType(ctx, res, uuid, 0)
	assert.Nil(t, err)
	assert.Equal(t, expectedReply, replyType)
}

func TestGetReplyType_InvalidResultType(t *testing.T) {
	res := contracts.DocumentResult{ResultType: ""}
	ctx := context.NewMockDefault()
	uuid := uuid.NewV4()
	_, err := GetReplyTypeObject(ctx, res, uuid, 0)
	assert.NotNil(t, err)
}
