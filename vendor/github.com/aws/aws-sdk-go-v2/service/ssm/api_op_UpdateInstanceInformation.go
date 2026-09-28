package ssm

import (
	"context"
	"fmt"
	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// Removes the server or virtual machine from the list of registered servers.
//
// If you want to reregister an on-premises server, edge device, or VM, you must
// use a different Activation Code and Activation ID than used to register the
// machine previously. The Activation Code and Activation ID must not have already
// been used on the maximum number of activations specified when they were created.
// For more information, see [Deregistering managed nodes in a hybrid and multicloud environment]in the Amazon Web Services Systems Manager User Guide.
//

func (c *Client) UpdateInstanceInformation(ctx context.Context, params *UpdateInstanceInformationInput, optFns ...func(*Options)) (*UpdateInstanceInformationOutput, error) {
	if params == nil {
		params = &UpdateInstanceInformationInput{}
	}

	result, metadata, err := c.invokeOperation(ctx, "UpdateInstanceInformation", params, optFns, c.addOperationUpdateInstanceInformationMiddlewares)
	if err != nil {
		return nil, err
	}

	out := result.(*UpdateInstanceInformationOutput)
	out.ResultMetadata = metadata
	return out, nil
}

type UpdateInstanceInformationInput struct {
	AgentName *string

	AgentStatus *string

	AgentVersion *string

	AvailabilityZone *string

	AvailabilityZoneId *string

	ComputerName *string

	// IPAddress is a sensitive parameter and its value will be
	// replaced with "sensitive" in string returned by UpdateInstanceInformationInput's
	// String and GoString methods.
	IPAddress *string

	// InstanceId is a required field
	InstanceId *string

	PlatformName *string

	PlatformType *string

	PlatformVersion *string

	SourceId *string

	SourceLocation *string

	SourceType *string

	SSMConnectionChannel *string

	noSmithyDocumentSerde
}

type UpdateInstanceInformationOutput struct {
	// Metadata pertaining to the operation's result.
	ResultMetadata middleware.Metadata

	noSmithyDocumentSerde
}

func (c *Client) addOperationUpdateInstanceInformationMiddlewares(stack *middleware.Stack, options Options) (err error) {
	if err := stack.Serialize.Add(&setOperationInputMiddleware{}, middleware.After); err != nil {
		return err
	}
	// Change once added to serializers.go
	err = stack.Serialize.Add(&awsAwsjson11_serializeOpUpdateInstanceInformation{}, middleware.After)
	if err != nil {
		return err
	}
	// Change once added to deserializers.go
	err = stack.Deserialize.Add(&awsAwsjson11_deserializeOpUpdateInstanceInformation{}, middleware.After)
	if err != nil {
		return err
	}
	if err := addProtocolFinalizerMiddlewares(stack, options, "UpdateInstanceInformation"); err != nil {
		return fmt.Errorf("add protocol finalizers: %v", err)
	}

	if err = addlegacyEndpointContextSetter(stack, options); err != nil {
		return err
	}
	if err = addSetLoggerMiddleware(stack, options); err != nil {
		return err
	}
	if err = addClientRequestID(stack); err != nil {
		return err
	}
	if err = addComputeContentLength(stack); err != nil {
		return err
	}
	if err = addResolveEndpointMiddleware(stack, options); err != nil {
		return err
	}
	if err = addComputePayloadSHA256(stack); err != nil {
		return err
	}
	if err = addRetry(stack, options); err != nil {
		return err
	}
	if err = addRawResponseToMetadata(stack); err != nil {
		return err
	}
	if err = addRecordResponseTiming(stack); err != nil {
		return err
	}
	if err = addSpanRetryLoop(stack, options); err != nil {
		return err
	}
	if err = addClientUserAgent(stack, options); err != nil {
		return err
	}
	if err = smithyhttp.AddErrorCloseResponseBodyMiddleware(stack); err != nil {
		return err
	}
	if err = smithyhttp.AddCloseResponseBodyMiddleware(stack); err != nil {
		return err
	}
	if err = addSetLegacyContextSigningOptionsMiddleware(stack); err != nil {
		return err
	}
	if err = addTimeOffsetBuild(stack, c); err != nil {
		return err
	}
	if err = addUserAgentRetryMode(stack, options); err != nil {
		return err
	}
	if err = addCredentialSource(stack, options); err != nil {
		return err
	}
	if err = addOpUpdateInstanceInformationValidationMiddleware(stack); err != nil {
		return err
	}
	if err = stack.Initialize.Add(newServiceMetadataMiddleware_opUpdateInstanceInformation(options.Region), middleware.Before); err != nil {
		return err
	}
	if err = addRecursionDetection(stack); err != nil {
		return err
	}
	if err = addRequestIDRetrieverMiddleware(stack); err != nil {
		return err
	}
	if err = addResponseErrorMiddleware(stack); err != nil {
		return err
	}
	if err = addRequestResponseLogging(stack, options); err != nil {
		return err
	}
	if err = addDisableHTTPSMiddleware(stack, options); err != nil {
		return err
	}
	if err = addSpanInitializeStart(stack); err != nil {
		return err
	}
	if err = addSpanInitializeEnd(stack); err != nil {
		return err
	}
	if err = addSpanBuildRequestStart(stack); err != nil {
		return err
	}
	if err = addSpanBuildRequestEnd(stack); err != nil {
		return err
	}
	return nil
}

func newServiceMetadataMiddleware_opUpdateInstanceInformation(region string) *awsmiddleware.RegisterServiceMetadata {
	return &awsmiddleware.RegisterServiceMetadata{
		Region:        region,
		ServiceID:     ServiceID,
		OperationName: "UpdateInstanceInformation",
	}
}
