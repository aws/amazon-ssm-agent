package replytypes

import (
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	"github.com/twinj/uuid"
)

// IReplyType is interface implemented by replytyes
type IReplyType interface {
	GetName() contracts.ResultType
	ConvertToAgentMessage() (*mgsContracts.AgentMessage, error)
	GetMessageUUID() uuid.UUID
	GetRetryNumber() int
	GetNumberOfContinuousRetries() int
	ShouldPersistData() bool
	GetBackOffSecond() int
	IncrementRetries() int
	GetResult() contracts.DocumentResult
}
