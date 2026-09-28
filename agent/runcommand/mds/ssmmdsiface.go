package service

import (
	"context"
)

// SsmmdsAPI provides an interface to enable mocking the
// ssmmds.Ssmmds service client's API operation
type SsmmdsAPI interface {
	AcknowledgeMessage(*AcknowledgeMessageInput) (*AcknowledgeMessageOutput, error)
	AcknowledgeMessageWithContext(context.Context, *AcknowledgeMessageInput, ...interface{}) (*AcknowledgeMessageOutput, error)
	AcknowledgeMessageRequest(*AcknowledgeMessageInput) (*Request, *AcknowledgeMessageOutput)

	DeleteMessage(*DeleteMessageInput) (*DeleteMessageOutput, error)
	DeleteMessageWithContext(context.Context, *DeleteMessageInput, ...interface{}) (*DeleteMessageOutput, error)
	DeleteMessageRequest(*DeleteMessageInput) (*Request, *DeleteMessageOutput)

	FailMessage(*FailMessageInput) (*FailMessageOutput, error)
	FailMessageWithContext(context.Context, *FailMessageInput, ...interface{}) (*FailMessageOutput, error)
	FailMessageRequest(*FailMessageInput) (*Request, *FailMessageOutput)

	GetEndpoint(*GetEndpointInput) (*GetEndpointOutput, error)
	GetEndpointWithContext(context.Context, *GetEndpointInput, ...interface{}) (*GetEndpointOutput, error)
	GetEndpointRequest(*GetEndpointInput) (*Request, *GetEndpointOutput)

	GetMessages(*GetMessagesInput) (*GetMessagesOutput, error)
	GetMessagesWithContext(context.Context, *GetMessagesInput, ...interface{}) (*GetMessagesOutput, error)
	GetMessagesRequest(*GetMessagesInput) (*Request, *GetMessagesOutput)

	SendReply(*SendReplyInput) (*SendReplyOutput, error)
	SendReplyWithContext(context.Context, *SendReplyInput, ...interface{}) (*SendReplyOutput, error)
	SendReplyRequest(*SendReplyInput) (*Request, *SendReplyOutput)
}

var _ SsmmdsAPI = (*Ssmmds)(nil)
