// Code generated by private/model/cli/gen-api/main.go. DO NOT EDIT.

// Package cleanroomsiface provides an interface to enable mocking the AWS Clean Rooms Service service client
// for testing your code.
//
// It is important to note that this interface will have breaking changes
// when the service model is updated and adds new API operations, paginators,
// and waiters.
package cleanroomsiface

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/cleanrooms"
)

// CleanRoomsAPI provides an interface to enable mocking the
// cleanrooms.CleanRooms service client's API operation,
// paginators, and waiters. This make unit testing your code that calls out
// to the SDK's service client's calls easier.
//
// The best way to use this interface is so the SDK's service client's calls
// can be stubbed out for unit testing your code with the SDK without needing
// to inject custom request handlers into the SDK's request pipeline.
//
//	// myFunc uses an SDK service client to make a request to
//	// AWS Clean Rooms Service.
//	func myFunc(svc cleanroomsiface.CleanRoomsAPI) bool {
//	    // Make svc.BatchGetCollaborationAnalysisTemplate request
//	}
//
//	func main() {
//	    sess := session.New()
//	    svc := cleanrooms.New(sess)
//
//	    myFunc(svc)
//	}
//
// In your _test.go file:
//
//	// Define a mock struct to be used in your unit tests of myFunc.
//	type mockCleanRoomsClient struct {
//	    cleanroomsiface.CleanRoomsAPI
//	}
//	func (m *mockCleanRoomsClient) BatchGetCollaborationAnalysisTemplate(input *cleanrooms.BatchGetCollaborationAnalysisTemplateInput) (*cleanrooms.BatchGetCollaborationAnalysisTemplateOutput, error) {
//	    // mock response/functionality
//	}
//
//	func TestMyFunc(t *testing.T) {
//	    // Setup Test
//	    mockSvc := &mockCleanRoomsClient{}
//
//	    myfunc(mockSvc)
//
//	    // Verify myFunc's functionality
//	}
//
// It is important to note that this interface will have breaking changes
// when the service model is updated and adds new API operations, paginators,
// and waiters. Its suggested to use the pattern above for testing, or using
// tooling to generate mocks to satisfy the interfaces.
type CleanRoomsAPI interface {
	BatchGetCollaborationAnalysisTemplate(*cleanrooms.BatchGetCollaborationAnalysisTemplateInput) (*cleanrooms.BatchGetCollaborationAnalysisTemplateOutput, error)
	BatchGetCollaborationAnalysisTemplateWithContext(aws.Context, *cleanrooms.BatchGetCollaborationAnalysisTemplateInput, ...request.Option) (*cleanrooms.BatchGetCollaborationAnalysisTemplateOutput, error)
	BatchGetCollaborationAnalysisTemplateRequest(*cleanrooms.BatchGetCollaborationAnalysisTemplateInput) (*request.Request, *cleanrooms.BatchGetCollaborationAnalysisTemplateOutput)

	BatchGetSchema(*cleanrooms.BatchGetSchemaInput) (*cleanrooms.BatchGetSchemaOutput, error)
	BatchGetSchemaWithContext(aws.Context, *cleanrooms.BatchGetSchemaInput, ...request.Option) (*cleanrooms.BatchGetSchemaOutput, error)
	BatchGetSchemaRequest(*cleanrooms.BatchGetSchemaInput) (*request.Request, *cleanrooms.BatchGetSchemaOutput)

	BatchGetSchemaAnalysisRule(*cleanrooms.BatchGetSchemaAnalysisRuleInput) (*cleanrooms.BatchGetSchemaAnalysisRuleOutput, error)
	BatchGetSchemaAnalysisRuleWithContext(aws.Context, *cleanrooms.BatchGetSchemaAnalysisRuleInput, ...request.Option) (*cleanrooms.BatchGetSchemaAnalysisRuleOutput, error)
	BatchGetSchemaAnalysisRuleRequest(*cleanrooms.BatchGetSchemaAnalysisRuleInput) (*request.Request, *cleanrooms.BatchGetSchemaAnalysisRuleOutput)

	CreateAnalysisTemplate(*cleanrooms.CreateAnalysisTemplateInput) (*cleanrooms.CreateAnalysisTemplateOutput, error)
	CreateAnalysisTemplateWithContext(aws.Context, *cleanrooms.CreateAnalysisTemplateInput, ...request.Option) (*cleanrooms.CreateAnalysisTemplateOutput, error)
	CreateAnalysisTemplateRequest(*cleanrooms.CreateAnalysisTemplateInput) (*request.Request, *cleanrooms.CreateAnalysisTemplateOutput)

	CreateCollaboration(*cleanrooms.CreateCollaborationInput) (*cleanrooms.CreateCollaborationOutput, error)
	CreateCollaborationWithContext(aws.Context, *cleanrooms.CreateCollaborationInput, ...request.Option) (*cleanrooms.CreateCollaborationOutput, error)
	CreateCollaborationRequest(*cleanrooms.CreateCollaborationInput) (*request.Request, *cleanrooms.CreateCollaborationOutput)

	CreateConfiguredAudienceModelAssociation(*cleanrooms.CreateConfiguredAudienceModelAssociationInput) (*cleanrooms.CreateConfiguredAudienceModelAssociationOutput, error)
	CreateConfiguredAudienceModelAssociationWithContext(aws.Context, *cleanrooms.CreateConfiguredAudienceModelAssociationInput, ...request.Option) (*cleanrooms.CreateConfiguredAudienceModelAssociationOutput, error)
	CreateConfiguredAudienceModelAssociationRequest(*cleanrooms.CreateConfiguredAudienceModelAssociationInput) (*request.Request, *cleanrooms.CreateConfiguredAudienceModelAssociationOutput)

	CreateConfiguredTable(*cleanrooms.CreateConfiguredTableInput) (*cleanrooms.CreateConfiguredTableOutput, error)
	CreateConfiguredTableWithContext(aws.Context, *cleanrooms.CreateConfiguredTableInput, ...request.Option) (*cleanrooms.CreateConfiguredTableOutput, error)
	CreateConfiguredTableRequest(*cleanrooms.CreateConfiguredTableInput) (*request.Request, *cleanrooms.CreateConfiguredTableOutput)

	CreateConfiguredTableAnalysisRule(*cleanrooms.CreateConfiguredTableAnalysisRuleInput) (*cleanrooms.CreateConfiguredTableAnalysisRuleOutput, error)
	CreateConfiguredTableAnalysisRuleWithContext(aws.Context, *cleanrooms.CreateConfiguredTableAnalysisRuleInput, ...request.Option) (*cleanrooms.CreateConfiguredTableAnalysisRuleOutput, error)
	CreateConfiguredTableAnalysisRuleRequest(*cleanrooms.CreateConfiguredTableAnalysisRuleInput) (*request.Request, *cleanrooms.CreateConfiguredTableAnalysisRuleOutput)

	CreateConfiguredTableAssociation(*cleanrooms.CreateConfiguredTableAssociationInput) (*cleanrooms.CreateConfiguredTableAssociationOutput, error)
	CreateConfiguredTableAssociationWithContext(aws.Context, *cleanrooms.CreateConfiguredTableAssociationInput, ...request.Option) (*cleanrooms.CreateConfiguredTableAssociationOutput, error)
	CreateConfiguredTableAssociationRequest(*cleanrooms.CreateConfiguredTableAssociationInput) (*request.Request, *cleanrooms.CreateConfiguredTableAssociationOutput)

	CreateMembership(*cleanrooms.CreateMembershipInput) (*cleanrooms.CreateMembershipOutput, error)
	CreateMembershipWithContext(aws.Context, *cleanrooms.CreateMembershipInput, ...request.Option) (*cleanrooms.CreateMembershipOutput, error)
	CreateMembershipRequest(*cleanrooms.CreateMembershipInput) (*request.Request, *cleanrooms.CreateMembershipOutput)

	CreatePrivacyBudgetTemplate(*cleanrooms.CreatePrivacyBudgetTemplateInput) (*cleanrooms.CreatePrivacyBudgetTemplateOutput, error)
	CreatePrivacyBudgetTemplateWithContext(aws.Context, *cleanrooms.CreatePrivacyBudgetTemplateInput, ...request.Option) (*cleanrooms.CreatePrivacyBudgetTemplateOutput, error)
	CreatePrivacyBudgetTemplateRequest(*cleanrooms.CreatePrivacyBudgetTemplateInput) (*request.Request, *cleanrooms.CreatePrivacyBudgetTemplateOutput)

	DeleteAnalysisTemplate(*cleanrooms.DeleteAnalysisTemplateInput) (*cleanrooms.DeleteAnalysisTemplateOutput, error)
	DeleteAnalysisTemplateWithContext(aws.Context, *cleanrooms.DeleteAnalysisTemplateInput, ...request.Option) (*cleanrooms.DeleteAnalysisTemplateOutput, error)
	DeleteAnalysisTemplateRequest(*cleanrooms.DeleteAnalysisTemplateInput) (*request.Request, *cleanrooms.DeleteAnalysisTemplateOutput)

	DeleteCollaboration(*cleanrooms.DeleteCollaborationInput) (*cleanrooms.DeleteCollaborationOutput, error)
	DeleteCollaborationWithContext(aws.Context, *cleanrooms.DeleteCollaborationInput, ...request.Option) (*cleanrooms.DeleteCollaborationOutput, error)
	DeleteCollaborationRequest(*cleanrooms.DeleteCollaborationInput) (*request.Request, *cleanrooms.DeleteCollaborationOutput)

	DeleteConfiguredAudienceModelAssociation(*cleanrooms.DeleteConfiguredAudienceModelAssociationInput) (*cleanrooms.DeleteConfiguredAudienceModelAssociationOutput, error)
	DeleteConfiguredAudienceModelAssociationWithContext(aws.Context, *cleanrooms.DeleteConfiguredAudienceModelAssociationInput, ...request.Option) (*cleanrooms.DeleteConfiguredAudienceModelAssociationOutput, error)
	DeleteConfiguredAudienceModelAssociationRequest(*cleanrooms.DeleteConfiguredAudienceModelAssociationInput) (*request.Request, *cleanrooms.DeleteConfiguredAudienceModelAssociationOutput)

	DeleteConfiguredTable(*cleanrooms.DeleteConfiguredTableInput) (*cleanrooms.DeleteConfiguredTableOutput, error)
	DeleteConfiguredTableWithContext(aws.Context, *cleanrooms.DeleteConfiguredTableInput, ...request.Option) (*cleanrooms.DeleteConfiguredTableOutput, error)
	DeleteConfiguredTableRequest(*cleanrooms.DeleteConfiguredTableInput) (*request.Request, *cleanrooms.DeleteConfiguredTableOutput)

	DeleteConfiguredTableAnalysisRule(*cleanrooms.DeleteConfiguredTableAnalysisRuleInput) (*cleanrooms.DeleteConfiguredTableAnalysisRuleOutput, error)
	DeleteConfiguredTableAnalysisRuleWithContext(aws.Context, *cleanrooms.DeleteConfiguredTableAnalysisRuleInput, ...request.Option) (*cleanrooms.DeleteConfiguredTableAnalysisRuleOutput, error)
	DeleteConfiguredTableAnalysisRuleRequest(*cleanrooms.DeleteConfiguredTableAnalysisRuleInput) (*request.Request, *cleanrooms.DeleteConfiguredTableAnalysisRuleOutput)

	DeleteConfiguredTableAssociation(*cleanrooms.DeleteConfiguredTableAssociationInput) (*cleanrooms.DeleteConfiguredTableAssociationOutput, error)
	DeleteConfiguredTableAssociationWithContext(aws.Context, *cleanrooms.DeleteConfiguredTableAssociationInput, ...request.Option) (*cleanrooms.DeleteConfiguredTableAssociationOutput, error)
	DeleteConfiguredTableAssociationRequest(*cleanrooms.DeleteConfiguredTableAssociationInput) (*request.Request, *cleanrooms.DeleteConfiguredTableAssociationOutput)

	DeleteMember(*cleanrooms.DeleteMemberInput) (*cleanrooms.DeleteMemberOutput, error)
	DeleteMemberWithContext(aws.Context, *cleanrooms.DeleteMemberInput, ...request.Option) (*cleanrooms.DeleteMemberOutput, error)
	DeleteMemberRequest(*cleanrooms.DeleteMemberInput) (*request.Request, *cleanrooms.DeleteMemberOutput)

	DeleteMembership(*cleanrooms.DeleteMembershipInput) (*cleanrooms.DeleteMembershipOutput, error)
	DeleteMembershipWithContext(aws.Context, *cleanrooms.DeleteMembershipInput, ...request.Option) (*cleanrooms.DeleteMembershipOutput, error)
	DeleteMembershipRequest(*cleanrooms.DeleteMembershipInput) (*request.Request, *cleanrooms.DeleteMembershipOutput)

	DeletePrivacyBudgetTemplate(*cleanrooms.DeletePrivacyBudgetTemplateInput) (*cleanrooms.DeletePrivacyBudgetTemplateOutput, error)
	DeletePrivacyBudgetTemplateWithContext(aws.Context, *cleanrooms.DeletePrivacyBudgetTemplateInput, ...request.Option) (*cleanrooms.DeletePrivacyBudgetTemplateOutput, error)
	DeletePrivacyBudgetTemplateRequest(*cleanrooms.DeletePrivacyBudgetTemplateInput) (*request.Request, *cleanrooms.DeletePrivacyBudgetTemplateOutput)

	GetAnalysisTemplate(*cleanrooms.GetAnalysisTemplateInput) (*cleanrooms.GetAnalysisTemplateOutput, error)
	GetAnalysisTemplateWithContext(aws.Context, *cleanrooms.GetAnalysisTemplateInput, ...request.Option) (*cleanrooms.GetAnalysisTemplateOutput, error)
	GetAnalysisTemplateRequest(*cleanrooms.GetAnalysisTemplateInput) (*request.Request, *cleanrooms.GetAnalysisTemplateOutput)

	GetCollaboration(*cleanrooms.GetCollaborationInput) (*cleanrooms.GetCollaborationOutput, error)
	GetCollaborationWithContext(aws.Context, *cleanrooms.GetCollaborationInput, ...request.Option) (*cleanrooms.GetCollaborationOutput, error)
	GetCollaborationRequest(*cleanrooms.GetCollaborationInput) (*request.Request, *cleanrooms.GetCollaborationOutput)

	GetCollaborationAnalysisTemplate(*cleanrooms.GetCollaborationAnalysisTemplateInput) (*cleanrooms.GetCollaborationAnalysisTemplateOutput, error)
	GetCollaborationAnalysisTemplateWithContext(aws.Context, *cleanrooms.GetCollaborationAnalysisTemplateInput, ...request.Option) (*cleanrooms.GetCollaborationAnalysisTemplateOutput, error)
	GetCollaborationAnalysisTemplateRequest(*cleanrooms.GetCollaborationAnalysisTemplateInput) (*request.Request, *cleanrooms.GetCollaborationAnalysisTemplateOutput)

	GetCollaborationConfiguredAudienceModelAssociation(*cleanrooms.GetCollaborationConfiguredAudienceModelAssociationInput) (*cleanrooms.GetCollaborationConfiguredAudienceModelAssociationOutput, error)
	GetCollaborationConfiguredAudienceModelAssociationWithContext(aws.Context, *cleanrooms.GetCollaborationConfiguredAudienceModelAssociationInput, ...request.Option) (*cleanrooms.GetCollaborationConfiguredAudienceModelAssociationOutput, error)
	GetCollaborationConfiguredAudienceModelAssociationRequest(*cleanrooms.GetCollaborationConfiguredAudienceModelAssociationInput) (*request.Request, *cleanrooms.GetCollaborationConfiguredAudienceModelAssociationOutput)

	GetCollaborationPrivacyBudgetTemplate(*cleanrooms.GetCollaborationPrivacyBudgetTemplateInput) (*cleanrooms.GetCollaborationPrivacyBudgetTemplateOutput, error)
	GetCollaborationPrivacyBudgetTemplateWithContext(aws.Context, *cleanrooms.GetCollaborationPrivacyBudgetTemplateInput, ...request.Option) (*cleanrooms.GetCollaborationPrivacyBudgetTemplateOutput, error)
	GetCollaborationPrivacyBudgetTemplateRequest(*cleanrooms.GetCollaborationPrivacyBudgetTemplateInput) (*request.Request, *cleanrooms.GetCollaborationPrivacyBudgetTemplateOutput)

	GetConfiguredAudienceModelAssociation(*cleanrooms.GetConfiguredAudienceModelAssociationInput) (*cleanrooms.GetConfiguredAudienceModelAssociationOutput, error)
	GetConfiguredAudienceModelAssociationWithContext(aws.Context, *cleanrooms.GetConfiguredAudienceModelAssociationInput, ...request.Option) (*cleanrooms.GetConfiguredAudienceModelAssociationOutput, error)
	GetConfiguredAudienceModelAssociationRequest(*cleanrooms.GetConfiguredAudienceModelAssociationInput) (*request.Request, *cleanrooms.GetConfiguredAudienceModelAssociationOutput)

	GetConfiguredTable(*cleanrooms.GetConfiguredTableInput) (*cleanrooms.GetConfiguredTableOutput, error)
	GetConfiguredTableWithContext(aws.Context, *cleanrooms.GetConfiguredTableInput, ...request.Option) (*cleanrooms.GetConfiguredTableOutput, error)
	GetConfiguredTableRequest(*cleanrooms.GetConfiguredTableInput) (*request.Request, *cleanrooms.GetConfiguredTableOutput)

	GetConfiguredTableAnalysisRule(*cleanrooms.GetConfiguredTableAnalysisRuleInput) (*cleanrooms.GetConfiguredTableAnalysisRuleOutput, error)
	GetConfiguredTableAnalysisRuleWithContext(aws.Context, *cleanrooms.GetConfiguredTableAnalysisRuleInput, ...request.Option) (*cleanrooms.GetConfiguredTableAnalysisRuleOutput, error)
	GetConfiguredTableAnalysisRuleRequest(*cleanrooms.GetConfiguredTableAnalysisRuleInput) (*request.Request, *cleanrooms.GetConfiguredTableAnalysisRuleOutput)

	GetConfiguredTableAssociation(*cleanrooms.GetConfiguredTableAssociationInput) (*cleanrooms.GetConfiguredTableAssociationOutput, error)
	GetConfiguredTableAssociationWithContext(aws.Context, *cleanrooms.GetConfiguredTableAssociationInput, ...request.Option) (*cleanrooms.GetConfiguredTableAssociationOutput, error)
	GetConfiguredTableAssociationRequest(*cleanrooms.GetConfiguredTableAssociationInput) (*request.Request, *cleanrooms.GetConfiguredTableAssociationOutput)

	GetMembership(*cleanrooms.GetMembershipInput) (*cleanrooms.GetMembershipOutput, error)
	GetMembershipWithContext(aws.Context, *cleanrooms.GetMembershipInput, ...request.Option) (*cleanrooms.GetMembershipOutput, error)
	GetMembershipRequest(*cleanrooms.GetMembershipInput) (*request.Request, *cleanrooms.GetMembershipOutput)

	GetPrivacyBudgetTemplate(*cleanrooms.GetPrivacyBudgetTemplateInput) (*cleanrooms.GetPrivacyBudgetTemplateOutput, error)
	GetPrivacyBudgetTemplateWithContext(aws.Context, *cleanrooms.GetPrivacyBudgetTemplateInput, ...request.Option) (*cleanrooms.GetPrivacyBudgetTemplateOutput, error)
	GetPrivacyBudgetTemplateRequest(*cleanrooms.GetPrivacyBudgetTemplateInput) (*request.Request, *cleanrooms.GetPrivacyBudgetTemplateOutput)

	GetProtectedQuery(*cleanrooms.GetProtectedQueryInput) (*cleanrooms.GetProtectedQueryOutput, error)
	GetProtectedQueryWithContext(aws.Context, *cleanrooms.GetProtectedQueryInput, ...request.Option) (*cleanrooms.GetProtectedQueryOutput, error)
	GetProtectedQueryRequest(*cleanrooms.GetProtectedQueryInput) (*request.Request, *cleanrooms.GetProtectedQueryOutput)

	GetSchema(*cleanrooms.GetSchemaInput) (*cleanrooms.GetSchemaOutput, error)
	GetSchemaWithContext(aws.Context, *cleanrooms.GetSchemaInput, ...request.Option) (*cleanrooms.GetSchemaOutput, error)
	GetSchemaRequest(*cleanrooms.GetSchemaInput) (*request.Request, *cleanrooms.GetSchemaOutput)

	GetSchemaAnalysisRule(*cleanrooms.GetSchemaAnalysisRuleInput) (*cleanrooms.GetSchemaAnalysisRuleOutput, error)
	GetSchemaAnalysisRuleWithContext(aws.Context, *cleanrooms.GetSchemaAnalysisRuleInput, ...request.Option) (*cleanrooms.GetSchemaAnalysisRuleOutput, error)
	GetSchemaAnalysisRuleRequest(*cleanrooms.GetSchemaAnalysisRuleInput) (*request.Request, *cleanrooms.GetSchemaAnalysisRuleOutput)

	ListAnalysisTemplates(*cleanrooms.ListAnalysisTemplatesInput) (*cleanrooms.ListAnalysisTemplatesOutput, error)
	ListAnalysisTemplatesWithContext(aws.Context, *cleanrooms.ListAnalysisTemplatesInput, ...request.Option) (*cleanrooms.ListAnalysisTemplatesOutput, error)
	ListAnalysisTemplatesRequest(*cleanrooms.ListAnalysisTemplatesInput) (*request.Request, *cleanrooms.ListAnalysisTemplatesOutput)

	ListAnalysisTemplatesPages(*cleanrooms.ListAnalysisTemplatesInput, func(*cleanrooms.ListAnalysisTemplatesOutput, bool) bool) error
	ListAnalysisTemplatesPagesWithContext(aws.Context, *cleanrooms.ListAnalysisTemplatesInput, func(*cleanrooms.ListAnalysisTemplatesOutput, bool) bool, ...request.Option) error

	ListCollaborationAnalysisTemplates(*cleanrooms.ListCollaborationAnalysisTemplatesInput) (*cleanrooms.ListCollaborationAnalysisTemplatesOutput, error)
	ListCollaborationAnalysisTemplatesWithContext(aws.Context, *cleanrooms.ListCollaborationAnalysisTemplatesInput, ...request.Option) (*cleanrooms.ListCollaborationAnalysisTemplatesOutput, error)
	ListCollaborationAnalysisTemplatesRequest(*cleanrooms.ListCollaborationAnalysisTemplatesInput) (*request.Request, *cleanrooms.ListCollaborationAnalysisTemplatesOutput)

	ListCollaborationAnalysisTemplatesPages(*cleanrooms.ListCollaborationAnalysisTemplatesInput, func(*cleanrooms.ListCollaborationAnalysisTemplatesOutput, bool) bool) error
	ListCollaborationAnalysisTemplatesPagesWithContext(aws.Context, *cleanrooms.ListCollaborationAnalysisTemplatesInput, func(*cleanrooms.ListCollaborationAnalysisTemplatesOutput, bool) bool, ...request.Option) error

	ListCollaborationConfiguredAudienceModelAssociations(*cleanrooms.ListCollaborationConfiguredAudienceModelAssociationsInput) (*cleanrooms.ListCollaborationConfiguredAudienceModelAssociationsOutput, error)
	ListCollaborationConfiguredAudienceModelAssociationsWithContext(aws.Context, *cleanrooms.ListCollaborationConfiguredAudienceModelAssociationsInput, ...request.Option) (*cleanrooms.ListCollaborationConfiguredAudienceModelAssociationsOutput, error)
	ListCollaborationConfiguredAudienceModelAssociationsRequest(*cleanrooms.ListCollaborationConfiguredAudienceModelAssociationsInput) (*request.Request, *cleanrooms.ListCollaborationConfiguredAudienceModelAssociationsOutput)

	ListCollaborationConfiguredAudienceModelAssociationsPages(*cleanrooms.ListCollaborationConfiguredAudienceModelAssociationsInput, func(*cleanrooms.ListCollaborationConfiguredAudienceModelAssociationsOutput, bool) bool) error
	ListCollaborationConfiguredAudienceModelAssociationsPagesWithContext(aws.Context, *cleanrooms.ListCollaborationConfiguredAudienceModelAssociationsInput, func(*cleanrooms.ListCollaborationConfiguredAudienceModelAssociationsOutput, bool) bool, ...request.Option) error

	ListCollaborationPrivacyBudgetTemplates(*cleanrooms.ListCollaborationPrivacyBudgetTemplatesInput) (*cleanrooms.ListCollaborationPrivacyBudgetTemplatesOutput, error)
	ListCollaborationPrivacyBudgetTemplatesWithContext(aws.Context, *cleanrooms.ListCollaborationPrivacyBudgetTemplatesInput, ...request.Option) (*cleanrooms.ListCollaborationPrivacyBudgetTemplatesOutput, error)
	ListCollaborationPrivacyBudgetTemplatesRequest(*cleanrooms.ListCollaborationPrivacyBudgetTemplatesInput) (*request.Request, *cleanrooms.ListCollaborationPrivacyBudgetTemplatesOutput)

	ListCollaborationPrivacyBudgetTemplatesPages(*cleanrooms.ListCollaborationPrivacyBudgetTemplatesInput, func(*cleanrooms.ListCollaborationPrivacyBudgetTemplatesOutput, bool) bool) error
	ListCollaborationPrivacyBudgetTemplatesPagesWithContext(aws.Context, *cleanrooms.ListCollaborationPrivacyBudgetTemplatesInput, func(*cleanrooms.ListCollaborationPrivacyBudgetTemplatesOutput, bool) bool, ...request.Option) error

	ListCollaborationPrivacyBudgets(*cleanrooms.ListCollaborationPrivacyBudgetsInput) (*cleanrooms.ListCollaborationPrivacyBudgetsOutput, error)
	ListCollaborationPrivacyBudgetsWithContext(aws.Context, *cleanrooms.ListCollaborationPrivacyBudgetsInput, ...request.Option) (*cleanrooms.ListCollaborationPrivacyBudgetsOutput, error)
	ListCollaborationPrivacyBudgetsRequest(*cleanrooms.ListCollaborationPrivacyBudgetsInput) (*request.Request, *cleanrooms.ListCollaborationPrivacyBudgetsOutput)

	ListCollaborationPrivacyBudgetsPages(*cleanrooms.ListCollaborationPrivacyBudgetsInput, func(*cleanrooms.ListCollaborationPrivacyBudgetsOutput, bool) bool) error
	ListCollaborationPrivacyBudgetsPagesWithContext(aws.Context, *cleanrooms.ListCollaborationPrivacyBudgetsInput, func(*cleanrooms.ListCollaborationPrivacyBudgetsOutput, bool) bool, ...request.Option) error

	ListCollaborations(*cleanrooms.ListCollaborationsInput) (*cleanrooms.ListCollaborationsOutput, error)
	ListCollaborationsWithContext(aws.Context, *cleanrooms.ListCollaborationsInput, ...request.Option) (*cleanrooms.ListCollaborationsOutput, error)
	ListCollaborationsRequest(*cleanrooms.ListCollaborationsInput) (*request.Request, *cleanrooms.ListCollaborationsOutput)

	ListCollaborationsPages(*cleanrooms.ListCollaborationsInput, func(*cleanrooms.ListCollaborationsOutput, bool) bool) error
	ListCollaborationsPagesWithContext(aws.Context, *cleanrooms.ListCollaborationsInput, func(*cleanrooms.ListCollaborationsOutput, bool) bool, ...request.Option) error

	ListConfiguredAudienceModelAssociations(*cleanrooms.ListConfiguredAudienceModelAssociationsInput) (*cleanrooms.ListConfiguredAudienceModelAssociationsOutput, error)
	ListConfiguredAudienceModelAssociationsWithContext(aws.Context, *cleanrooms.ListConfiguredAudienceModelAssociationsInput, ...request.Option) (*cleanrooms.ListConfiguredAudienceModelAssociationsOutput, error)
	ListConfiguredAudienceModelAssociationsRequest(*cleanrooms.ListConfiguredAudienceModelAssociationsInput) (*request.Request, *cleanrooms.ListConfiguredAudienceModelAssociationsOutput)

	ListConfiguredAudienceModelAssociationsPages(*cleanrooms.ListConfiguredAudienceModelAssociationsInput, func(*cleanrooms.ListConfiguredAudienceModelAssociationsOutput, bool) bool) error
	ListConfiguredAudienceModelAssociationsPagesWithContext(aws.Context, *cleanrooms.ListConfiguredAudienceModelAssociationsInput, func(*cleanrooms.ListConfiguredAudienceModelAssociationsOutput, bool) bool, ...request.Option) error

	ListConfiguredTableAssociations(*cleanrooms.ListConfiguredTableAssociationsInput) (*cleanrooms.ListConfiguredTableAssociationsOutput, error)
	ListConfiguredTableAssociationsWithContext(aws.Context, *cleanrooms.ListConfiguredTableAssociationsInput, ...request.Option) (*cleanrooms.ListConfiguredTableAssociationsOutput, error)
	ListConfiguredTableAssociationsRequest(*cleanrooms.ListConfiguredTableAssociationsInput) (*request.Request, *cleanrooms.ListConfiguredTableAssociationsOutput)

	ListConfiguredTableAssociationsPages(*cleanrooms.ListConfiguredTableAssociationsInput, func(*cleanrooms.ListConfiguredTableAssociationsOutput, bool) bool) error
	ListConfiguredTableAssociationsPagesWithContext(aws.Context, *cleanrooms.ListConfiguredTableAssociationsInput, func(*cleanrooms.ListConfiguredTableAssociationsOutput, bool) bool, ...request.Option) error

	ListConfiguredTables(*cleanrooms.ListConfiguredTablesInput) (*cleanrooms.ListConfiguredTablesOutput, error)
	ListConfiguredTablesWithContext(aws.Context, *cleanrooms.ListConfiguredTablesInput, ...request.Option) (*cleanrooms.ListConfiguredTablesOutput, error)
	ListConfiguredTablesRequest(*cleanrooms.ListConfiguredTablesInput) (*request.Request, *cleanrooms.ListConfiguredTablesOutput)

	ListConfiguredTablesPages(*cleanrooms.ListConfiguredTablesInput, func(*cleanrooms.ListConfiguredTablesOutput, bool) bool) error
	ListConfiguredTablesPagesWithContext(aws.Context, *cleanrooms.ListConfiguredTablesInput, func(*cleanrooms.ListConfiguredTablesOutput, bool) bool, ...request.Option) error

	ListMembers(*cleanrooms.ListMembersInput) (*cleanrooms.ListMembersOutput, error)
	ListMembersWithContext(aws.Context, *cleanrooms.ListMembersInput, ...request.Option) (*cleanrooms.ListMembersOutput, error)
	ListMembersRequest(*cleanrooms.ListMembersInput) (*request.Request, *cleanrooms.ListMembersOutput)

	ListMembersPages(*cleanrooms.ListMembersInput, func(*cleanrooms.ListMembersOutput, bool) bool) error
	ListMembersPagesWithContext(aws.Context, *cleanrooms.ListMembersInput, func(*cleanrooms.ListMembersOutput, bool) bool, ...request.Option) error

	ListMemberships(*cleanrooms.ListMembershipsInput) (*cleanrooms.ListMembershipsOutput, error)
	ListMembershipsWithContext(aws.Context, *cleanrooms.ListMembershipsInput, ...request.Option) (*cleanrooms.ListMembershipsOutput, error)
	ListMembershipsRequest(*cleanrooms.ListMembershipsInput) (*request.Request, *cleanrooms.ListMembershipsOutput)

	ListMembershipsPages(*cleanrooms.ListMembershipsInput, func(*cleanrooms.ListMembershipsOutput, bool) bool) error
	ListMembershipsPagesWithContext(aws.Context, *cleanrooms.ListMembershipsInput, func(*cleanrooms.ListMembershipsOutput, bool) bool, ...request.Option) error

	ListPrivacyBudgetTemplates(*cleanrooms.ListPrivacyBudgetTemplatesInput) (*cleanrooms.ListPrivacyBudgetTemplatesOutput, error)
	ListPrivacyBudgetTemplatesWithContext(aws.Context, *cleanrooms.ListPrivacyBudgetTemplatesInput, ...request.Option) (*cleanrooms.ListPrivacyBudgetTemplatesOutput, error)
	ListPrivacyBudgetTemplatesRequest(*cleanrooms.ListPrivacyBudgetTemplatesInput) (*request.Request, *cleanrooms.ListPrivacyBudgetTemplatesOutput)

	ListPrivacyBudgetTemplatesPages(*cleanrooms.ListPrivacyBudgetTemplatesInput, func(*cleanrooms.ListPrivacyBudgetTemplatesOutput, bool) bool) error
	ListPrivacyBudgetTemplatesPagesWithContext(aws.Context, *cleanrooms.ListPrivacyBudgetTemplatesInput, func(*cleanrooms.ListPrivacyBudgetTemplatesOutput, bool) bool, ...request.Option) error

	ListPrivacyBudgets(*cleanrooms.ListPrivacyBudgetsInput) (*cleanrooms.ListPrivacyBudgetsOutput, error)
	ListPrivacyBudgetsWithContext(aws.Context, *cleanrooms.ListPrivacyBudgetsInput, ...request.Option) (*cleanrooms.ListPrivacyBudgetsOutput, error)
	ListPrivacyBudgetsRequest(*cleanrooms.ListPrivacyBudgetsInput) (*request.Request, *cleanrooms.ListPrivacyBudgetsOutput)

	ListPrivacyBudgetsPages(*cleanrooms.ListPrivacyBudgetsInput, func(*cleanrooms.ListPrivacyBudgetsOutput, bool) bool) error
	ListPrivacyBudgetsPagesWithContext(aws.Context, *cleanrooms.ListPrivacyBudgetsInput, func(*cleanrooms.ListPrivacyBudgetsOutput, bool) bool, ...request.Option) error

	ListProtectedQueries(*cleanrooms.ListProtectedQueriesInput) (*cleanrooms.ListProtectedQueriesOutput, error)
	ListProtectedQueriesWithContext(aws.Context, *cleanrooms.ListProtectedQueriesInput, ...request.Option) (*cleanrooms.ListProtectedQueriesOutput, error)
	ListProtectedQueriesRequest(*cleanrooms.ListProtectedQueriesInput) (*request.Request, *cleanrooms.ListProtectedQueriesOutput)

	ListProtectedQueriesPages(*cleanrooms.ListProtectedQueriesInput, func(*cleanrooms.ListProtectedQueriesOutput, bool) bool) error
	ListProtectedQueriesPagesWithContext(aws.Context, *cleanrooms.ListProtectedQueriesInput, func(*cleanrooms.ListProtectedQueriesOutput, bool) bool, ...request.Option) error

	ListSchemas(*cleanrooms.ListSchemasInput) (*cleanrooms.ListSchemasOutput, error)
	ListSchemasWithContext(aws.Context, *cleanrooms.ListSchemasInput, ...request.Option) (*cleanrooms.ListSchemasOutput, error)
	ListSchemasRequest(*cleanrooms.ListSchemasInput) (*request.Request, *cleanrooms.ListSchemasOutput)

	ListSchemasPages(*cleanrooms.ListSchemasInput, func(*cleanrooms.ListSchemasOutput, bool) bool) error
	ListSchemasPagesWithContext(aws.Context, *cleanrooms.ListSchemasInput, func(*cleanrooms.ListSchemasOutput, bool) bool, ...request.Option) error

	ListTagsForResource(*cleanrooms.ListTagsForResourceInput) (*cleanrooms.ListTagsForResourceOutput, error)
	ListTagsForResourceWithContext(aws.Context, *cleanrooms.ListTagsForResourceInput, ...request.Option) (*cleanrooms.ListTagsForResourceOutput, error)
	ListTagsForResourceRequest(*cleanrooms.ListTagsForResourceInput) (*request.Request, *cleanrooms.ListTagsForResourceOutput)

	PreviewPrivacyImpact(*cleanrooms.PreviewPrivacyImpactInput) (*cleanrooms.PreviewPrivacyImpactOutput, error)
	PreviewPrivacyImpactWithContext(aws.Context, *cleanrooms.PreviewPrivacyImpactInput, ...request.Option) (*cleanrooms.PreviewPrivacyImpactOutput, error)
	PreviewPrivacyImpactRequest(*cleanrooms.PreviewPrivacyImpactInput) (*request.Request, *cleanrooms.PreviewPrivacyImpactOutput)

	StartProtectedQuery(*cleanrooms.StartProtectedQueryInput) (*cleanrooms.StartProtectedQueryOutput, error)
	StartProtectedQueryWithContext(aws.Context, *cleanrooms.StartProtectedQueryInput, ...request.Option) (*cleanrooms.StartProtectedQueryOutput, error)
	StartProtectedQueryRequest(*cleanrooms.StartProtectedQueryInput) (*request.Request, *cleanrooms.StartProtectedQueryOutput)

	TagResource(*cleanrooms.TagResourceInput) (*cleanrooms.TagResourceOutput, error)
	TagResourceWithContext(aws.Context, *cleanrooms.TagResourceInput, ...request.Option) (*cleanrooms.TagResourceOutput, error)
	TagResourceRequest(*cleanrooms.TagResourceInput) (*request.Request, *cleanrooms.TagResourceOutput)

	UntagResource(*cleanrooms.UntagResourceInput) (*cleanrooms.UntagResourceOutput, error)
	UntagResourceWithContext(aws.Context, *cleanrooms.UntagResourceInput, ...request.Option) (*cleanrooms.UntagResourceOutput, error)
	UntagResourceRequest(*cleanrooms.UntagResourceInput) (*request.Request, *cleanrooms.UntagResourceOutput)

	UpdateAnalysisTemplate(*cleanrooms.UpdateAnalysisTemplateInput) (*cleanrooms.UpdateAnalysisTemplateOutput, error)
	UpdateAnalysisTemplateWithContext(aws.Context, *cleanrooms.UpdateAnalysisTemplateInput, ...request.Option) (*cleanrooms.UpdateAnalysisTemplateOutput, error)
	UpdateAnalysisTemplateRequest(*cleanrooms.UpdateAnalysisTemplateInput) (*request.Request, *cleanrooms.UpdateAnalysisTemplateOutput)

	UpdateCollaboration(*cleanrooms.UpdateCollaborationInput) (*cleanrooms.UpdateCollaborationOutput, error)
	UpdateCollaborationWithContext(aws.Context, *cleanrooms.UpdateCollaborationInput, ...request.Option) (*cleanrooms.UpdateCollaborationOutput, error)
	UpdateCollaborationRequest(*cleanrooms.UpdateCollaborationInput) (*request.Request, *cleanrooms.UpdateCollaborationOutput)

	UpdateConfiguredAudienceModelAssociation(*cleanrooms.UpdateConfiguredAudienceModelAssociationInput) (*cleanrooms.UpdateConfiguredAudienceModelAssociationOutput, error)
	UpdateConfiguredAudienceModelAssociationWithContext(aws.Context, *cleanrooms.UpdateConfiguredAudienceModelAssociationInput, ...request.Option) (*cleanrooms.UpdateConfiguredAudienceModelAssociationOutput, error)
	UpdateConfiguredAudienceModelAssociationRequest(*cleanrooms.UpdateConfiguredAudienceModelAssociationInput) (*request.Request, *cleanrooms.UpdateConfiguredAudienceModelAssociationOutput)

	UpdateConfiguredTable(*cleanrooms.UpdateConfiguredTableInput) (*cleanrooms.UpdateConfiguredTableOutput, error)
	UpdateConfiguredTableWithContext(aws.Context, *cleanrooms.UpdateConfiguredTableInput, ...request.Option) (*cleanrooms.UpdateConfiguredTableOutput, error)
	UpdateConfiguredTableRequest(*cleanrooms.UpdateConfiguredTableInput) (*request.Request, *cleanrooms.UpdateConfiguredTableOutput)

	UpdateConfiguredTableAnalysisRule(*cleanrooms.UpdateConfiguredTableAnalysisRuleInput) (*cleanrooms.UpdateConfiguredTableAnalysisRuleOutput, error)
	UpdateConfiguredTableAnalysisRuleWithContext(aws.Context, *cleanrooms.UpdateConfiguredTableAnalysisRuleInput, ...request.Option) (*cleanrooms.UpdateConfiguredTableAnalysisRuleOutput, error)
	UpdateConfiguredTableAnalysisRuleRequest(*cleanrooms.UpdateConfiguredTableAnalysisRuleInput) (*request.Request, *cleanrooms.UpdateConfiguredTableAnalysisRuleOutput)

	UpdateConfiguredTableAssociation(*cleanrooms.UpdateConfiguredTableAssociationInput) (*cleanrooms.UpdateConfiguredTableAssociationOutput, error)
	UpdateConfiguredTableAssociationWithContext(aws.Context, *cleanrooms.UpdateConfiguredTableAssociationInput, ...request.Option) (*cleanrooms.UpdateConfiguredTableAssociationOutput, error)
	UpdateConfiguredTableAssociationRequest(*cleanrooms.UpdateConfiguredTableAssociationInput) (*request.Request, *cleanrooms.UpdateConfiguredTableAssociationOutput)

	UpdateMembership(*cleanrooms.UpdateMembershipInput) (*cleanrooms.UpdateMembershipOutput, error)
	UpdateMembershipWithContext(aws.Context, *cleanrooms.UpdateMembershipInput, ...request.Option) (*cleanrooms.UpdateMembershipOutput, error)
	UpdateMembershipRequest(*cleanrooms.UpdateMembershipInput) (*request.Request, *cleanrooms.UpdateMembershipOutput)

	UpdatePrivacyBudgetTemplate(*cleanrooms.UpdatePrivacyBudgetTemplateInput) (*cleanrooms.UpdatePrivacyBudgetTemplateOutput, error)
	UpdatePrivacyBudgetTemplateWithContext(aws.Context, *cleanrooms.UpdatePrivacyBudgetTemplateInput, ...request.Option) (*cleanrooms.UpdatePrivacyBudgetTemplateOutput, error)
	UpdatePrivacyBudgetTemplateRequest(*cleanrooms.UpdatePrivacyBudgetTemplateInput) (*request.Request, *cleanrooms.UpdatePrivacyBudgetTemplateOutput)

	UpdateProtectedQuery(*cleanrooms.UpdateProtectedQueryInput) (*cleanrooms.UpdateProtectedQueryOutput, error)
	UpdateProtectedQueryWithContext(aws.Context, *cleanrooms.UpdateProtectedQueryInput, ...request.Option) (*cleanrooms.UpdateProtectedQueryOutput, error)
	UpdateProtectedQueryRequest(*cleanrooms.UpdateProtectedQueryInput) (*request.Request, *cleanrooms.UpdateProtectedQueryOutput)
}

var _ CleanRoomsAPI = (*cleanrooms.CleanRooms)(nil)