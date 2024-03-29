// Code generated by private/model/cli/gen-api/main.go. DO NOT EDIT.

// Package ivsrealtimeiface provides an interface to enable mocking the Amazon Interactive Video Service RealTime service client
// for testing your code.
//
// It is important to note that this interface will have breaking changes
// when the service model is updated and adds new API operations, paginators,
// and waiters.
package ivsrealtimeiface

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ivsrealtime"
)

// IVSRealTimeAPI provides an interface to enable mocking the
// ivsrealtime.IVSRealTime service client's API operation,
// paginators, and waiters. This make unit testing your code that calls out
// to the SDK's service client's calls easier.
//
// The best way to use this interface is so the SDK's service client's calls
// can be stubbed out for unit testing your code with the SDK without needing
// to inject custom request handlers into the SDK's request pipeline.
//
//    // myFunc uses an SDK service client to make a request to
//    // Amazon Interactive Video Service RealTime.
//    func myFunc(svc ivsrealtimeiface.IVSRealTimeAPI) bool {
//        // Make svc.CreateParticipantToken request
//    }
//
//    func main() {
//        sess := session.New()
//        svc := ivsrealtime.New(sess)
//
//        myFunc(svc)
//    }
//
// In your _test.go file:
//
//    // Define a mock struct to be used in your unit tests of myFunc.
//    type mockIVSRealTimeClient struct {
//        ivsrealtimeiface.IVSRealTimeAPI
//    }
//    func (m *mockIVSRealTimeClient) CreateParticipantToken(input *ivsrealtime.CreateParticipantTokenInput) (*ivsrealtime.CreateParticipantTokenOutput, error) {
//        // mock response/functionality
//    }
//
//    func TestMyFunc(t *testing.T) {
//        // Setup Test
//        mockSvc := &mockIVSRealTimeClient{}
//
//        myfunc(mockSvc)
//
//        // Verify myFunc's functionality
//    }
//
// It is important to note that this interface will have breaking changes
// when the service model is updated and adds new API operations, paginators,
// and waiters. Its suggested to use the pattern above for testing, or using
// tooling to generate mocks to satisfy the interfaces.
type IVSRealTimeAPI interface {
	CreateParticipantToken(*ivsrealtime.CreateParticipantTokenInput) (*ivsrealtime.CreateParticipantTokenOutput, error)
	CreateParticipantTokenWithContext(aws.Context, *ivsrealtime.CreateParticipantTokenInput, ...request.Option) (*ivsrealtime.CreateParticipantTokenOutput, error)
	CreateParticipantTokenRequest(*ivsrealtime.CreateParticipantTokenInput) (*request.Request, *ivsrealtime.CreateParticipantTokenOutput)

	CreateStage(*ivsrealtime.CreateStageInput) (*ivsrealtime.CreateStageOutput, error)
	CreateStageWithContext(aws.Context, *ivsrealtime.CreateStageInput, ...request.Option) (*ivsrealtime.CreateStageOutput, error)
	CreateStageRequest(*ivsrealtime.CreateStageInput) (*request.Request, *ivsrealtime.CreateStageOutput)

	DeleteStage(*ivsrealtime.DeleteStageInput) (*ivsrealtime.DeleteStageOutput, error)
	DeleteStageWithContext(aws.Context, *ivsrealtime.DeleteStageInput, ...request.Option) (*ivsrealtime.DeleteStageOutput, error)
	DeleteStageRequest(*ivsrealtime.DeleteStageInput) (*request.Request, *ivsrealtime.DeleteStageOutput)

	DisconnectParticipant(*ivsrealtime.DisconnectParticipantInput) (*ivsrealtime.DisconnectParticipantOutput, error)
	DisconnectParticipantWithContext(aws.Context, *ivsrealtime.DisconnectParticipantInput, ...request.Option) (*ivsrealtime.DisconnectParticipantOutput, error)
	DisconnectParticipantRequest(*ivsrealtime.DisconnectParticipantInput) (*request.Request, *ivsrealtime.DisconnectParticipantOutput)

	GetStage(*ivsrealtime.GetStageInput) (*ivsrealtime.GetStageOutput, error)
	GetStageWithContext(aws.Context, *ivsrealtime.GetStageInput, ...request.Option) (*ivsrealtime.GetStageOutput, error)
	GetStageRequest(*ivsrealtime.GetStageInput) (*request.Request, *ivsrealtime.GetStageOutput)

	ListStages(*ivsrealtime.ListStagesInput) (*ivsrealtime.ListStagesOutput, error)
	ListStagesWithContext(aws.Context, *ivsrealtime.ListStagesInput, ...request.Option) (*ivsrealtime.ListStagesOutput, error)
	ListStagesRequest(*ivsrealtime.ListStagesInput) (*request.Request, *ivsrealtime.ListStagesOutput)

	ListStagesPages(*ivsrealtime.ListStagesInput, func(*ivsrealtime.ListStagesOutput, bool) bool) error
	ListStagesPagesWithContext(aws.Context, *ivsrealtime.ListStagesInput, func(*ivsrealtime.ListStagesOutput, bool) bool, ...request.Option) error

	ListTagsForResource(*ivsrealtime.ListTagsForResourceInput) (*ivsrealtime.ListTagsForResourceOutput, error)
	ListTagsForResourceWithContext(aws.Context, *ivsrealtime.ListTagsForResourceInput, ...request.Option) (*ivsrealtime.ListTagsForResourceOutput, error)
	ListTagsForResourceRequest(*ivsrealtime.ListTagsForResourceInput) (*request.Request, *ivsrealtime.ListTagsForResourceOutput)

	TagResource(*ivsrealtime.TagResourceInput) (*ivsrealtime.TagResourceOutput, error)
	TagResourceWithContext(aws.Context, *ivsrealtime.TagResourceInput, ...request.Option) (*ivsrealtime.TagResourceOutput, error)
	TagResourceRequest(*ivsrealtime.TagResourceInput) (*request.Request, *ivsrealtime.TagResourceOutput)

	UntagResource(*ivsrealtime.UntagResourceInput) (*ivsrealtime.UntagResourceOutput, error)
	UntagResourceWithContext(aws.Context, *ivsrealtime.UntagResourceInput, ...request.Option) (*ivsrealtime.UntagResourceOutput, error)
	UntagResourceRequest(*ivsrealtime.UntagResourceInput) (*request.Request, *ivsrealtime.UntagResourceOutput)

	UpdateStage(*ivsrealtime.UpdateStageInput) (*ivsrealtime.UpdateStageOutput, error)
	UpdateStageWithContext(aws.Context, *ivsrealtime.UpdateStageInput, ...request.Option) (*ivsrealtime.UpdateStageOutput, error)
	UpdateStageRequest(*ivsrealtime.UpdateStageInput) (*request.Request, *ivsrealtime.UpdateStageOutput)
}

var _ IVSRealTimeAPI = (*ivsrealtime.IVSRealTime)(nil)
