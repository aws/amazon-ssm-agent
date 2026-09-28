package ssm

import (
	"context"
	"fmt"
	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"time"
)

// Requests a managed instance role token for authentication.
func (c *Client) RequestManagedInstanceRoleToken(ctx context.Context, params *RequestManagedInstanceRoleTokenInput, optFns ...func(*Options)) (*RequestManagedInstanceRoleTokenOutput, error) {
	if params == nil {
		params = &RequestManagedInstanceRoleTokenInput{}
	}

	result, metadata, err := c.invokeOperation(ctx, "RequestManagedInstanceRoleToken", params, optFns, c.addOperationRequestManagedInstanceRoleTokenMiddlewares)
	if err != nil {
		return nil, err
	}

	out := result.(*RequestManagedInstanceRoleTokenOutput)
	out.ResultMetadata = metadata
	return out, nil
}

type RequestManagedInstanceRoleTokenInput struct {
	// Fingerprint is a required field
	Fingerprint *string

	noSmithyDocumentSerde
}

type RequestManagedInstanceRoleTokenOutput struct {
	// AccessKeyId is a sensitive parameter and its value will be
	// replaced with "sensitive" in string returned by RequestManagedInstanceRoleTokenOutput's
	// String and GoString methods.
	AccessKeyId *string

	// SecretAccessKey is a sensitive parameter and its value will be
	// replaced with "sensitive" in string returned by RequestManagedInstanceRoleTokenOutput's
	// String and GoString methods.
	SecretAccessKey *string

	// SessionToken is a sensitive parameter and its value will be
	// replaced with "sensitive" in string returned by RequestManagedInstanceRoleTokenOutput's
	// String and GoString methods.
	SessionToken *string

	TokenExpirationDate *time.Time

	UpdateKeyPair *bool

	// Metadata pertaining to the operation's result.
	ResultMetadata middleware.Metadata

	noSmithyDocumentSerde
}

func (c *Client) addOperationRequestManagedInstanceRoleTokenMiddlewares(stack *middleware.Stack, options Options) (err error) {
	if err := stack.Serialize.Add(&setOperationInputMiddleware{}, middleware.After); err != nil {
		return err
	}
	// Change once added to serializers.go
	err = stack.Serialize.Add(&awsAwsjson11_serializeOpRequestManagedInstanceRoleToken{}, middleware.After)
	if err != nil {
		return err
	}
	// Change once added to deserializers.go
	err = stack.Deserialize.Add(&awsAwsjson11_deserializeOpRequestManagedInstanceRoleToken{}, middleware.After)
	if err != nil {
		return err
	}
	if err := addProtocolFinalizerMiddlewares(stack, options, "RequestManagedInstanceRoleToken"); err != nil {
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
	if err = addOpRequestManagedInstanceRoleTokenValidationMiddleware(stack); err != nil {
		return err
	}
	if err = stack.Initialize.Add(newServiceMetadataMiddleware_opRequestManagedInstanceRoleToken(options.Region), middleware.Before); err != nil {
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

func newServiceMetadataMiddleware_opRequestManagedInstanceRoleToken(region string) *awsmiddleware.RegisterServiceMetadata {
	return &awsmiddleware.RegisterServiceMetadata{
		Region:        region,
		ServiceID:     ServiceID,
		OperationName: "RequestManagedInstanceRoleToken",
	}
}
