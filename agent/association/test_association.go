// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Amazon Software License (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/asl/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package association

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/stretchr/testify/mock"
)

type TestCasePollOnce struct {
	ContextMock *context.Mock

	SsmMock *MockedSSM
}

// MockedSSM stands for a mock SSM service.
type MockedSSM struct {
	mock.Mock
}

func (ssmMock *MockedSSM) AddTagsToResourceRequest(a *ssm.AddTagsToResourceInput) (*request.Request, *ssm.AddTagsToResourceOutput) {
	return nil, nil
}

func (ssmMock *MockedSSM) AddTagsToResource(*ssm.AddTagsToResourceInput) (*ssm.AddTagsToResourceOutput, error) {
	return nil, nil
}

func (ssmMock *MockedSSM) CancelCommandRequest(*ssm.CancelCommandInput) (*request.Request, *ssm.CancelCommandOutput) {
	return nil, nil
}

func (ssmMock *MockedSSM) CancelCommand(*ssm.CancelCommandInput) (*ssm.CancelCommandOutput, error) {
	return nil, nil
}

func (ssmMock *MockedSSM) CreateActivationRequest(*ssm.CreateActivationInput) (*request.Request, *ssm.CreateActivationOutput) {
	return nil, nil
}

func (ssmMock *MockedSSM) CreateActivation(*ssm.CreateActivationInput) (*ssm.CreateActivationOutput, error) {
	return nil, nil
}

func (ssmMock *MockedSSM) CreateAssociationRequest(*ssm.CreateAssociationInput) (*request.Request, *ssm.CreateAssociationOutput) {
	return nil, nil
}

func (ssmMock *MockedSSM) CreateAssociation(*ssm.CreateAssociationInput) (*ssm.CreateAssociationOutput, error) {
	return nil, nil
}

func (ssmMock *MockedSSM) CreateAssociationBatchRequest(*ssm.CreateAssociationBatchInput) (*request.Request, *ssm.CreateAssociationBatchOutput) {
	return nil, nil
}

func (ssmMock *MockedSSM) CreateAssociationBatch(*ssm.CreateAssociationBatchInput) (*ssm.CreateAssociationBatchOutput, error) {
	return nil, nil
}

func (ssmMock *MockedSSM) CreateDocumentRequest(*ssm.CreateDocumentInput) (*request.Request, *ssm.CreateDocumentOutput) {
	return nil, nil
}

func (ssmMock *MockedSSM) CreateDocument(*ssm.CreateDocumentInput) (*ssm.CreateDocumentOutput, error) {
	return nil, nil
}

func (ssmMock *MockedSSM) DeleteActivationRequest(*ssm.DeleteActivationInput) (*request.Request, *ssm.DeleteActivationOutput) {
	return nil, nil
}

func (ssmMock *MockedSSM) DeleteActivation(*ssm.DeleteActivationInput) (*ssm.DeleteActivationOutput, error) {
	return nil, nil
}

func (ssmMock *MockedSSM) DeleteAssociationRequest(*ssm.DeleteAssociationInput) (*request.Request, *ssm.DeleteAssociationOutput) {
	return nil, nil
}

func (ssmMock *MockedSSM) DeleteAssociation(*ssm.DeleteAssociationInput) (*ssm.DeleteAssociationOutput, error) {
	return nil, nil
}

func (ssmMock *MockedSSM) DeleteDocumentRequest(*ssm.DeleteDocumentInput) (*request.Request, *ssm.DeleteDocumentOutput) {
	return nil, nil
}

func (ssmMock *MockedSSM) DeleteDocument(*ssm.DeleteDocumentInput) (*ssm.DeleteDocumentOutput, error) {
	return nil, nil
}

func (ssmMock *MockedSSM) DeregisterManagedInstanceRequest(*ssm.DeregisterManagedInstanceInput) (*request.Request, *ssm.DeregisterManagedInstanceOutput) {
	return nil, nil
}

func (ssmMock *MockedSSM) DeregisterManagedInstance(*ssm.DeregisterManagedInstanceInput) (*ssm.DeregisterManagedInstanceOutput, error) {
	return nil, nil
}

func (ssmMock *MockedSSM) DescribeActivationsRequest(*ssm.DescribeActivationsInput) (*request.Request, *ssm.DescribeActivationsOutput) {
	return nil, nil
}

func (ssmMock *MockedSSM) DescribeActivations(*ssm.DescribeActivationsInput) (*ssm.DescribeActivationsOutput, error) {
	return nil, nil
}

func (ssmMock *MockedSSM) DescribeAssociationRequest(*ssm.DescribeAssociationInput) (*request.Request, *ssm.DescribeAssociationOutput) {
	return nil, nil
}

func (ssmMock *MockedSSM) DescribeAssociation(*ssm.DescribeAssociationInput) (*ssm.DescribeAssociationOutput, error) {
	return nil, nil
}

func (ssmMock *MockedSSM) DescribeDocumentRequest(*ssm.DescribeDocumentInput) (*request.Request, *ssm.DescribeDocumentOutput) {
	return nil, nil
}

func (ssmMock *MockedSSM) DescribeDocument(*ssm.DescribeDocumentInput) (*ssm.DescribeDocumentOutput, error) {
	return nil, nil
}

func (ssmMock *MockedSSM) DescribeDocumentParametersRequest(*ssm.DescribeDocumentParametersInput) (*request.Request, *ssm.DescribeDocumentParametersOutput) {
	return nil, nil
}

func (ssmMock *MockedSSM) DescribeDocumentParameters(*ssm.DescribeDocumentParametersInput) (*ssm.DescribeDocumentParametersOutput, error) {
	return nil, nil
}

func (ssmMock *MockedSSM) DescribeDocumentPermissionRequest(*ssm.DescribeDocumentPermissionInput) (*request.Request, *ssm.DescribeDocumentPermissionOutput) {
	return nil, nil
}

func (ssmMock *MockedSSM) DescribeDocumentPermission(*ssm.DescribeDocumentPermissionInput) (*ssm.DescribeDocumentPermissionOutput, error) {
	return nil, nil
}

func (ssmMock *MockedSSM) DescribeInstanceInformationRequest(*ssm.DescribeInstanceInformationInput) (*request.Request, *ssm.DescribeInstanceInformationOutput) {
	return nil, nil
}

func (ssmMock *MockedSSM) DescribeInstanceInformation(*ssm.DescribeInstanceInformationInput) (*ssm.DescribeInstanceInformationOutput, error) {
	return nil, nil
}

func (ssmMock *MockedSSM) DescribeInstancePropertiesRequest(*ssm.DescribeInstancePropertiesInput) (*request.Request, *ssm.DescribeInstancePropertiesOutput) {
	return nil, nil
}

func (ssmMock *MockedSSM) DescribeInstanceProperties(*ssm.DescribeInstancePropertiesInput) (*ssm.DescribeInstancePropertiesOutput, error) {
	return nil, nil
}

func (ssmMock *MockedSSM) GetDocumentRequest(*ssm.GetDocumentInput) (*request.Request, *ssm.GetDocumentOutput) {
	return nil, nil
}

func (ssmMock *MockedSSM) GetDocument(gdi *ssm.GetDocumentInput) (*ssm.GetDocumentOutput, error) {
	args := ssmMock.Called(gdi)
	return args.Get(0).(*ssm.GetDocumentOutput), args.Error(1)
}

func (ssmMock *MockedSSM) ListAssociationsRequest(*ssm.ListAssociationsInput) (*request.Request, *ssm.ListAssociationsOutput) {
	return nil, nil
}

func (ssmMock *MockedSSM) ListAssociations(lai *ssm.ListAssociationsInput) (*ssm.ListAssociationsOutput, error) {
	args := ssmMock.Called(lai)
	return args.Get(0).(*ssm.ListAssociationsOutput), args.Error(1)
}

func (ssmMock *MockedSSM) ListAssociationsPages(*ssm.ListAssociationsInput, func(*ssm.ListAssociationsOutput, bool) bool) error {
	return nil
}

func (ssmMock *MockedSSM) ListCommandInvocationsRequest(*ssm.ListCommandInvocationsInput) (*request.Request, *ssm.ListCommandInvocationsOutput) {
	return nil, nil
}

func (ssmMock *MockedSSM) ListCommandInvocations(*ssm.ListCommandInvocationsInput) (*ssm.ListCommandInvocationsOutput, error) {
	return nil, nil
}

func (ssmMock *MockedSSM) ListCommandInvocationsPages(*ssm.ListCommandInvocationsInput, func(*ssm.ListCommandInvocationsOutput, bool) bool) error {
	return nil
}

func (ssmMock *MockedSSM) ListCommandsRequest(*ssm.ListCommandsInput) (*request.Request, *ssm.ListCommandsOutput) {
	return nil, nil
}

func (ssmMock *MockedSSM) ListCommands(*ssm.ListCommandsInput) (*ssm.ListCommandsOutput, error) {
	return nil, nil
}

func (ssmMock *MockedSSM) ListCommandsPages(*ssm.ListCommandsInput, func(*ssm.ListCommandsOutput, bool) bool) error {
	return nil
}

func (ssmMock *MockedSSM) ListDocumentsRequest(*ssm.ListDocumentsInput) (*request.Request, *ssm.ListDocumentsOutput) {
	return nil, nil
}

func (ssmMock *MockedSSM) ListDocuments(*ssm.ListDocumentsInput) (*ssm.ListDocumentsOutput, error) {
	return nil, nil
}

func (ssmMock *MockedSSM) ListDocumentsPages(*ssm.ListDocumentsInput, func(*ssm.ListDocumentsOutput, bool) bool) error {
	return nil
}

func (ssmMock *MockedSSM) ListTagsForResourceRequest(*ssm.ListTagsForResourceInput) (*request.Request, *ssm.ListTagsForResourceOutput) {
	return nil, nil
}

func (ssmMock *MockedSSM) ListTagsForResource(*ssm.ListTagsForResourceInput) (*ssm.ListTagsForResourceOutput, error) {
	return nil, nil
}

func (ssmMock *MockedSSM) ModifyDocumentPermissionRequest(*ssm.ModifyDocumentPermissionInput) (*request.Request, *ssm.ModifyDocumentPermissionOutput) {
	return nil, nil
}

func (ssmMock *MockedSSM) ModifyDocumentPermission(*ssm.ModifyDocumentPermissionInput) (*ssm.ModifyDocumentPermissionOutput, error) {
	return nil, nil
}

func (ssmMock *MockedSSM) RegisterManagedInstanceRequest(*ssm.RegisterManagedInstanceInput) (*request.Request, *ssm.RegisterManagedInstanceOutput) {
	return nil, nil
}

func (ssmMock *MockedSSM) RegisterManagedInstance(*ssm.RegisterManagedInstanceInput) (*ssm.RegisterManagedInstanceOutput, error) {
	return nil, nil
}

func (ssmMock *MockedSSM) RemoveTagsFromResourceRequest(*ssm.RemoveTagsFromResourceInput) (*request.Request, *ssm.RemoveTagsFromResourceOutput) {
	return nil, nil
}

func (ssmMock *MockedSSM) RemoveTagsFromResource(*ssm.RemoveTagsFromResourceInput) (*ssm.RemoveTagsFromResourceOutput, error) {
	return nil, nil
}

func (ssmMock *MockedSSM) RequestManagedInstanceRoleTokenRequest(*ssm.RequestManagedInstanceRoleTokenInput) (*request.Request, *ssm.RequestManagedInstanceRoleTokenOutput) {
	return nil, nil
}

func (ssmMock *MockedSSM) RequestManagedInstanceRoleToken(*ssm.RequestManagedInstanceRoleTokenInput) (*ssm.RequestManagedInstanceRoleTokenOutput, error) {
	return nil, nil
}

func (ssmMock *MockedSSM) SendCommandRequest(*ssm.SendCommandInput) (*request.Request, *ssm.SendCommandOutput) {
	return nil, nil
}

func (ssmMock *MockedSSM) SendCommand(*ssm.SendCommandInput) (*ssm.SendCommandOutput, error) {
	return nil, nil
}

func (ssmMock *MockedSSM) UpdateAssociationStatusRequest(*ssm.UpdateAssociationStatusInput) (*request.Request, *ssm.UpdateAssociationStatusOutput) {
	return nil, nil
}

func (ssmMock *MockedSSM) UpdateAssociationStatus(*ssm.UpdateAssociationStatusInput) (*ssm.UpdateAssociationStatusOutput, error) {
	return nil, nil
}

func (ssmMock *MockedSSM) UpdateInstanceInformationRequest(*ssm.UpdateInstanceInformationInput) (*request.Request, *ssm.UpdateInstanceInformationOutput) {
	return nil, nil
}

func (ssmMock *MockedSSM) UpdateInstanceInformation(*ssm.UpdateInstanceInformationInput) (*ssm.UpdateInstanceInformationOutput, error) {
	return nil, nil
}

func (ssmMock *MockedSSM) UpdateManagedInstancePublicKeyRequest(*ssm.UpdateManagedInstancePublicKeyInput) (*request.Request, *ssm.UpdateManagedInstancePublicKeyOutput) {
	return nil, nil
}

func (ssmMock *MockedSSM) UpdateManagedInstancePublicKey(*ssm.UpdateManagedInstancePublicKeyInput) (*ssm.UpdateManagedInstancePublicKeyOutput, error) {
	return nil, nil
}

func (ssmMock *MockedSSM) UpdateManagedInstanceRoleRequest(*ssm.UpdateManagedInstanceRoleInput) (*request.Request, *ssm.UpdateManagedInstanceRoleOutput) {
	return nil, nil
}

func (ssmMock *MockedSSM) UpdateManagedInstanceRole(*ssm.UpdateManagedInstanceRoleInput) (*ssm.UpdateManagedInstanceRoleOutput, error) {
	return nil, nil
}
