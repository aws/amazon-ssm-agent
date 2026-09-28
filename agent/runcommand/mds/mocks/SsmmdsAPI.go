package mocks

import (
	"context"

	service "github.com/aws/amazon-ssm-agent/agent/runcommand/mds"
	"github.com/stretchr/testify/mock"
)

// SsmmdsAPI is a mock type for the local SsmmdsAPI interface
type SsmmdsAPI struct {
	mock.Mock
}

// AcknowledgeMessage provides a mock function with given fields: input
func (_m *SsmmdsAPI) AcknowledgeMessage(input *service.AcknowledgeMessageInput) (*service.AcknowledgeMessageOutput, error) {
	ret := _m.Called(input)

	var r0 *service.AcknowledgeMessageOutput
	var r1 error
	if rf, ok := ret.Get(0).(func(*service.AcknowledgeMessageInput) (*service.AcknowledgeMessageOutput, error)); ok {
		return rf(input)
	}
	if rf, ok := ret.Get(0).(func(*service.AcknowledgeMessageInput) *service.AcknowledgeMessageOutput); ok {
		r0 = rf(input)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*service.AcknowledgeMessageOutput)
		}
	}

	if rf, ok := ret.Get(1).(func(*service.AcknowledgeMessageInput) error); ok {
		r1 = rf(input)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// AcknowledgeMessageWithContext provides a mock function with given fields: ctx, input, opts
func (_m *SsmmdsAPI) AcknowledgeMessageWithContext(ctx context.Context, input *service.AcknowledgeMessageInput, opts ...interface{}) (*service.AcknowledgeMessageOutput, error) {
	args := []interface{}{ctx, input}
	for _, opt := range opts {
		args = append(args, opt)
	}
	ret := _m.Called(args...)

	var r0 *service.AcknowledgeMessageOutput
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *service.AcknowledgeMessageInput, ...interface{}) (*service.AcknowledgeMessageOutput, error)); ok {
		return rf(ctx, input, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *service.AcknowledgeMessageInput, ...interface{}) *service.AcknowledgeMessageOutput); ok {
		r0 = rf(ctx, input, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*service.AcknowledgeMessageOutput)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *service.AcknowledgeMessageInput, ...interface{}) error); ok {
		r1 = rf(ctx, input, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// AcknowledgeMessageRequest provides a mock function with given fields: input
func (_m *SsmmdsAPI) AcknowledgeMessageRequest(input *service.AcknowledgeMessageInput) (*service.Request, *service.AcknowledgeMessageOutput) {
	ret := _m.Called(input)

	var r0 *service.Request
	var r1 *service.AcknowledgeMessageOutput
	if rf, ok := ret.Get(0).(func(*service.AcknowledgeMessageInput) (*service.Request, *service.AcknowledgeMessageOutput)); ok {
		return rf(input)
	}
	if rf, ok := ret.Get(0).(func(*service.AcknowledgeMessageInput) *service.Request); ok {
		r0 = rf(input)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*service.Request)
		}
	}

	if rf, ok := ret.Get(1).(func(*service.AcknowledgeMessageInput) *service.AcknowledgeMessageOutput); ok {
		r1 = rf(input)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*service.AcknowledgeMessageOutput)
		}
	}

	return r0, r1
}

// DeleteMessage provides a mock function with given fields: input
func (_m *SsmmdsAPI) DeleteMessage(input *service.DeleteMessageInput) (*service.DeleteMessageOutput, error) {
	ret := _m.Called(input)

	var r0 *service.DeleteMessageOutput
	var r1 error
	if rf, ok := ret.Get(0).(func(*service.DeleteMessageInput) (*service.DeleteMessageOutput, error)); ok {
		return rf(input)
	}
	if rf, ok := ret.Get(0).(func(*service.DeleteMessageInput) *service.DeleteMessageOutput); ok {
		r0 = rf(input)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*service.DeleteMessageOutput)
		}
	}

	if rf, ok := ret.Get(1).(func(*service.DeleteMessageInput) error); ok {
		r1 = rf(input)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// DeleteMessageWithContext provides a mock function with given fields: ctx, input, opts
func (_m *SsmmdsAPI) DeleteMessageWithContext(ctx context.Context, input *service.DeleteMessageInput, opts ...interface{}) (*service.DeleteMessageOutput, error) {
	args := []interface{}{ctx, input}
	for _, opt := range opts {
		args = append(args, opt)
	}
	ret := _m.Called(args...)

	var r0 *service.DeleteMessageOutput
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *service.DeleteMessageInput, ...interface{}) (*service.DeleteMessageOutput, error)); ok {
		return rf(ctx, input, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *service.DeleteMessageInput, ...interface{}) *service.DeleteMessageOutput); ok {
		r0 = rf(ctx, input, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*service.DeleteMessageOutput)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *service.DeleteMessageInput, ...interface{}) error); ok {
		r1 = rf(ctx, input, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// DeleteMessageRequest provides a mock function with given fields: input
func (_m *SsmmdsAPI) DeleteMessageRequest(input *service.DeleteMessageInput) (*service.Request, *service.DeleteMessageOutput) {
	ret := _m.Called(input)

	var r0 *service.Request
	var r1 *service.DeleteMessageOutput
	if rf, ok := ret.Get(0).(func(*service.DeleteMessageInput) (*service.Request, *service.DeleteMessageOutput)); ok {
		return rf(input)
	}
	if rf, ok := ret.Get(0).(func(*service.DeleteMessageInput) *service.Request); ok {
		r0 = rf(input)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*service.Request)
		}
	}

	if rf, ok := ret.Get(1).(func(*service.DeleteMessageInput) *service.DeleteMessageOutput); ok {
		r1 = rf(input)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*service.DeleteMessageOutput)
		}
	}

	return r0, r1
}

// FailMessage provides a mock function with given fields: input
func (_m *SsmmdsAPI) FailMessage(input *service.FailMessageInput) (*service.FailMessageOutput, error) {
	ret := _m.Called(input)

	var r0 *service.FailMessageOutput
	var r1 error
	if rf, ok := ret.Get(0).(func(*service.FailMessageInput) (*service.FailMessageOutput, error)); ok {
		return rf(input)
	}
	if rf, ok := ret.Get(0).(func(*service.FailMessageInput) *service.FailMessageOutput); ok {
		r0 = rf(input)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*service.FailMessageOutput)
		}
	}

	if rf, ok := ret.Get(1).(func(*service.FailMessageInput) error); ok {
		r1 = rf(input)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// FailMessageWithContext provides a mock function with given fields: ctx, input, opts
func (_m *SsmmdsAPI) FailMessageWithContext(ctx context.Context, input *service.FailMessageInput, opts ...interface{}) (*service.FailMessageOutput, error) {
	args := []interface{}{ctx, input}
	for _, opt := range opts {
		args = append(args, opt)
	}
	ret := _m.Called(args...)

	var r0 *service.FailMessageOutput
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *service.FailMessageInput, ...interface{}) (*service.FailMessageOutput, error)); ok {
		return rf(ctx, input, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *service.FailMessageInput, ...interface{}) *service.FailMessageOutput); ok {
		r0 = rf(ctx, input, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*service.FailMessageOutput)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *service.FailMessageInput, ...interface{}) error); ok {
		r1 = rf(ctx, input, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// FailMessageRequest provides a mock function with given fields: input
func (_m *SsmmdsAPI) FailMessageRequest(input *service.FailMessageInput) (*service.Request, *service.FailMessageOutput) {
	ret := _m.Called(input)

	var r0 *service.Request
	var r1 *service.FailMessageOutput
	if rf, ok := ret.Get(0).(func(*service.FailMessageInput) (*service.Request, *service.FailMessageOutput)); ok {
		return rf(input)
	}
	if rf, ok := ret.Get(0).(func(*service.FailMessageInput) *service.Request); ok {
		r0 = rf(input)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*service.Request)
		}
	}

	if rf, ok := ret.Get(1).(func(*service.FailMessageInput) *service.FailMessageOutput); ok {
		r1 = rf(input)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*service.FailMessageOutput)
		}
	}

	return r0, r1
}

// GetEndpoint provides a mock function with given fields: input
func (_m *SsmmdsAPI) GetEndpoint(input *service.GetEndpointInput) (*service.GetEndpointOutput, error) {
	ret := _m.Called(input)

	var r0 *service.GetEndpointOutput
	var r1 error
	if rf, ok := ret.Get(0).(func(*service.GetEndpointInput) (*service.GetEndpointOutput, error)); ok {
		return rf(input)
	}
	if rf, ok := ret.Get(0).(func(*service.GetEndpointInput) *service.GetEndpointOutput); ok {
		r0 = rf(input)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*service.GetEndpointOutput)
		}
	}

	if rf, ok := ret.Get(1).(func(*service.GetEndpointInput) error); ok {
		r1 = rf(input)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetEndpointWithContext provides a mock function with given fields: ctx, input, opts
func (_m *SsmmdsAPI) GetEndpointWithContext(ctx context.Context, input *service.GetEndpointInput, opts ...interface{}) (*service.GetEndpointOutput, error) {
	args := []interface{}{ctx, input}
	for _, opt := range opts {
		args = append(args, opt)
	}
	ret := _m.Called(args...)

	var r0 *service.GetEndpointOutput
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *service.GetEndpointInput, ...interface{}) (*service.GetEndpointOutput, error)); ok {
		return rf(ctx, input, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *service.GetEndpointInput, ...interface{}) *service.GetEndpointOutput); ok {
		r0 = rf(ctx, input, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*service.GetEndpointOutput)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *service.GetEndpointInput, ...interface{}) error); ok {
		r1 = rf(ctx, input, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetEndpointRequest provides a mock function with given fields: input
func (_m *SsmmdsAPI) GetEndpointRequest(input *service.GetEndpointInput) (*service.Request, *service.GetEndpointOutput) {
	ret := _m.Called(input)

	var r0 *service.Request
	var r1 *service.GetEndpointOutput
	if rf, ok := ret.Get(0).(func(*service.GetEndpointInput) (*service.Request, *service.GetEndpointOutput)); ok {
		return rf(input)
	}
	if rf, ok := ret.Get(0).(func(*service.GetEndpointInput) *service.Request); ok {
		r0 = rf(input)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*service.Request)
		}
	}

	if rf, ok := ret.Get(1).(func(*service.GetEndpointInput) *service.GetEndpointOutput); ok {
		r1 = rf(input)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*service.GetEndpointOutput)
		}
	}

	return r0, r1
}

// GetMessages provides a mock function with given fields: input
func (_m *SsmmdsAPI) GetMessages(input *service.GetMessagesInput) (*service.GetMessagesOutput, error) {
	ret := _m.Called(input)

	var r0 *service.GetMessagesOutput
	var r1 error
	if rf, ok := ret.Get(0).(func(*service.GetMessagesInput) (*service.GetMessagesOutput, error)); ok {
		return rf(input)
	}
	if rf, ok := ret.Get(0).(func(*service.GetMessagesInput) *service.GetMessagesOutput); ok {
		r0 = rf(input)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*service.GetMessagesOutput)
		}
	}

	if rf, ok := ret.Get(1).(func(*service.GetMessagesInput) error); ok {
		r1 = rf(input)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetMessagesWithContext provides a mock function with given fields: ctx, input, opts
func (_m *SsmmdsAPI) GetMessagesWithContext(ctx context.Context, input *service.GetMessagesInput, opts ...interface{}) (*service.GetMessagesOutput, error) {
	args := []interface{}{ctx, input}
	for _, opt := range opts {
		args = append(args, opt)
	}
	ret := _m.Called(args...)

	var r0 *service.GetMessagesOutput
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *service.GetMessagesInput, ...interface{}) (*service.GetMessagesOutput, error)); ok {
		return rf(ctx, input, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *service.GetMessagesInput, ...interface{}) *service.GetMessagesOutput); ok {
		r0 = rf(ctx, input, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*service.GetMessagesOutput)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *service.GetMessagesInput, ...interface{}) error); ok {
		r1 = rf(ctx, input, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetMessagesRequest provides a mock function with given fields: input
func (_m *SsmmdsAPI) GetMessagesRequest(input *service.GetMessagesInput) (*service.Request, *service.GetMessagesOutput) {
	ret := _m.Called(input)

	var r0 *service.Request
	var r1 *service.GetMessagesOutput
	if rf, ok := ret.Get(0).(func(*service.GetMessagesInput) (*service.Request, *service.GetMessagesOutput)); ok {
		return rf(input)
	}
	if rf, ok := ret.Get(0).(func(*service.GetMessagesInput) *service.Request); ok {
		r0 = rf(input)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*service.Request)
		}
	}

	if rf, ok := ret.Get(1).(func(*service.GetMessagesInput) *service.GetMessagesOutput); ok {
		r1 = rf(input)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*service.GetMessagesOutput)
		}
	}

	return r0, r1
}

// SendReply provides a mock function with given fields: input
func (_m *SsmmdsAPI) SendReply(input *service.SendReplyInput) (*service.SendReplyOutput, error) {
	ret := _m.Called(input)

	var r0 *service.SendReplyOutput
	var r1 error
	if rf, ok := ret.Get(0).(func(*service.SendReplyInput) (*service.SendReplyOutput, error)); ok {
		return rf(input)
	}
	if rf, ok := ret.Get(0).(func(*service.SendReplyInput) *service.SendReplyOutput); ok {
		r0 = rf(input)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*service.SendReplyOutput)
		}
	}

	if rf, ok := ret.Get(1).(func(*service.SendReplyInput) error); ok {
		r1 = rf(input)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// SendReplyWithContext provides a mock function with given fields: ctx, input, opts
func (_m *SsmmdsAPI) SendReplyWithContext(ctx context.Context, input *service.SendReplyInput, opts ...interface{}) (*service.SendReplyOutput, error) {
	args := []interface{}{ctx, input}
	for _, opt := range opts {
		args = append(args, opt)
	}
	ret := _m.Called(args...)

	var r0 *service.SendReplyOutput
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *service.SendReplyInput, ...interface{}) (*service.SendReplyOutput, error)); ok {
		return rf(ctx, input, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *service.SendReplyInput, ...interface{}) *service.SendReplyOutput); ok {
		r0 = rf(ctx, input, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*service.SendReplyOutput)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *service.SendReplyInput, ...interface{}) error); ok {
		r1 = rf(ctx, input, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// SendReplyRequest provides a mock function with given fields: input
func (_m *SsmmdsAPI) SendReplyRequest(input *service.SendReplyInput) (*service.Request, *service.SendReplyOutput) {
	ret := _m.Called(input)

	var r0 *service.Request
	var r1 *service.SendReplyOutput
	if rf, ok := ret.Get(0).(func(*service.SendReplyInput) (*service.Request, *service.SendReplyOutput)); ok {
		return rf(input)
	}
	if rf, ok := ret.Get(0).(func(*service.SendReplyInput) *service.Request); ok {
		r0 = rf(input)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*service.Request)
		}
	}

	if rf, ok := ret.Get(1).(func(*service.SendReplyInput) *service.SendReplyOutput); ok {
		r1 = rf(input)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*service.SendReplyOutput)
		}
	}

	return r0, r1
}

// NewSsmmdsAPI creates a new instance of SsmmdsAPI. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewSsmmdsAPI(t interface {
	mock.TestingT
	Cleanup(func())
}) *SsmmdsAPI {
	mock := &SsmmdsAPI{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
