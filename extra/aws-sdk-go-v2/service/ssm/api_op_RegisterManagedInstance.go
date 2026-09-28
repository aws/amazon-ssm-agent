package ssm

import (
	"context"
	"fmt"
	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
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

func (c *Client) RegisterManagedInstance(ctx context.Context, params *RegisterManagedInstanceInput, optFns ...func(*Options)) (*RegisterManagedInstanceOutput, error) {
	if params == nil {
		params = &RegisterManagedInstanceInput{}
	}

	result, metadata, err := c.invokeOperation(ctx, "RegisterManagedInstance", params, optFns, c.addOperationRegisterManagedInstanceMiddlewares)
	if err != nil {
		return nil, err
	}

	out := result.(*RegisterManagedInstanceOutput)
	out.ResultMetadata = metadata
	return out, nil
}

type RegisterManagedInstanceInput struct {

	// The ID assigned to the managed node when you registered it using the activation
	// process.
	//
	// This member is required.
	ActivationCode *string

	ActivationId *string

	Fingerprint *string

	IamRole *string

	PublicKey *string

	PublicKeyType *string

	Provider *string

	Tags []types.Tag

	noSmithyDocumentSerde
}

type RegisterManagedInstanceOutput struct {
	InstanceId *string
	// Metadata pertaining to the operation's result.
	ResultMetadata middleware.Metadata

	noSmithyDocumentSerde
}

func (c *Client) addOperationRegisterManagedInstanceMiddlewares(stack *middleware.Stack, options Options) (err error) {
	if err := stack.Serialize.Add(&setOperationInputMiddleware{}, middleware.After); err != nil {
		return err
	}
	// Change once added to serializers.go
	err = stack.Serialize.Add(&awsAwsjson11_serializeOpRegisterManagedInstance{}, middleware.After)
	if err != nil {
		return err
	}
	// Change once added to deserializers.go
	err = stack.Deserialize.Add(&awsAwsjson11_deserializeOpRegisterManagedInstance{}, middleware.After)
	if err != nil {
		return err
	}
	if err := addProtocolFinalizerMiddlewares(stack, options, "RegisterManagedInstance"); err != nil {
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
	if err = addOpRegisterManagedInstanceValidationMiddleware(stack); err != nil {
		return err
	}
	if err = stack.Initialize.Add(newServiceMetadataMiddleware_opRegisterManagedInstance(options.Region), middleware.Before); err != nil {
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

func newServiceMetadataMiddleware_opRegisterManagedInstance(region string) *awsmiddleware.RegisterServiceMetadata {
	return &awsmiddleware.RegisterServiceMetadata{
		Region:        region,
		ServiceID:     ServiceID,
		OperationName: "RegisterManagedInstance",
	}
}
