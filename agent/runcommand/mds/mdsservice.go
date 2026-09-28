package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"

	agentcontext "github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/network"
	"github.com/aws/amazon-ssm-agent/common/identity/endpoint"
)

// NewMdsClient creates a new MDS client using independent credential resolution
func NewMdsClient(agentContext agentcontext.T, httpClient *http.Client) *Ssmmds {
	agentConfig := agentContext.AppConfig()

	// Use agentConfig for region resolution
	var region string
	region, _ = agentContext.Identity().Region()
	if agentConfig.Agent.Region != "" {
		region = agentConfig.Agent.Region
	}

	// Use agentConfig for endpoint resolution
	endpointStr := agentConfig.Mds.Endpoint
	if endpointStr != "" && !strings.HasPrefix(endpointStr, "http://") && !strings.HasPrefix(endpointStr, "https://") {
		endpointStr = "https://" + endpointStr
	}
	if endpointStr == "" {
		endpointStr = agentContext.Identity().GetServiceEndpoint("ec2messages")
	}
	if endpointStr == "" {
		// Last-resort fallback: construct the endpoint from the same
		// partition-aware service-domain map the identity helper uses so
		// MDS stays partition-correct on cn-, us-gov-, us-iso-, us-isob-,
		// us-isof-, eu-isoe-, and eusc-de- regions.
		serviceDomain := endpoint.GetServiceDomainByPrefix(region)
		endpointStr = fmt.Sprintf("https://ec2messages.%s.%s", region, serviceDomain)
	}

	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				Proxy:               http.ProxyFromEnvironment,
				TLSHandshakeTimeout: 10 * time.Second,
				TLSClientConfig:     network.GetDefaultTLSConfig(agentContext.Log(), agentConfig),
			},
		}
	}

	// Store credential provider instead of static credentials
	credProvider := agentContext.Identity().CredentialsProvider()

	// Build the User-Agent string from the agent name/version, e.g. "amazon-ssm-agent/VERSION".
	userAgent := fmt.Sprintf("%s/%s", agentConfig.Agent.Name, agentConfig.Agent.Version)

	return &Ssmmds{
		client:             httpClient,
		endpoint:           endpointStr,
		region:             region,
		credentialProvider: credProvider,
		userAgent:          userAgent,
	}
}

// Error codes
const (
	ErrCodeAuthorizationFailureException        = "AuthorizationFailureException"
	ErrCodeInternalServerException              = "InternalServerException"
	ErrCodeInvalidDestinationException          = "InvalidDestinationException"
	ErrCodeInvalidMessageIdException            = "InvalidMessageIdException"
	ErrCodeRequestTimeoutException              = "RequestTimeoutException"
	ErrCodeTooManyRequestsException             = "TooManyRequestsException"
	ErrCodeUnsupportedMessageOperationException = "UnsupportedMessageOperationException"
)

// Service constants
const (
	ServiceName = "ssmmds"
	EndpointsID = "ec2messages"
	ServiceID   = "ssmmds"
)

// Ssmmds provides the API operation methods
type Ssmmds struct {
	client             *http.Client
	endpoint           string
	region             string
	credentialProvider aws.CredentialsProvider
	userAgent          string
}

// Config holds AWS configuration
type Config struct {
	Region       string
	Endpoint     string
	AccessKey    string
	SecretKey    string
	SessionToken string
	HTTPClient   *http.Client
}

// Request represents an AWS request
type Request struct {
	HTTPRequest   *http.Request
	Params        interface{}
	Data          interface{}
	Error         error
	context       context.Context
	httpClient    *http.Client
	body          []byte // original request body for re-signing on retry
	operationName string // API operation name, used to tune retry behavior
}

// Send executes the request
func (r *Request) Send() error {
	if r.Error != nil {
		return r.Error
	}

	if r.HTTPRequest == nil {
		return fmt.Errorf("HTTPRequest is nil")
	}

	// Use the context if available
	ctx := r.context
	if ctx == nil {
		ctx = context.Background()
	}

	// Set the context on the HTTP request using a local variable. Reassigning
	// r.HTTPRequest here would race with Stop()/cancelSdkRequest, which reads
	// r.HTTPRequest concurrently to cancel the in-flight request.
	httpReq := r.HTTPRequest.WithContext(ctx)

	// Use the configured client (preserves TLS config, proxy, timeouts)
	client := r.httpClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		r.Error = err
		return err
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		r.Error = err
		return err
	}

	// Handle HTTP errors
	if resp.StatusCode >= 400 {
		r.Error = parseError(body, resp.StatusCode)
		return r.Error
	}

	// Parse response into Data if it's provided
	if len(body) > 0 && r.Data != nil {
		if err := json.Unmarshal(body, r.Data); err != nil {
			r.Error = err
			return err
		}
	}

	return nil
}

// SetContext sets the request context
func (r *Request) SetContext(ctx context.Context) {
	r.context = ctx
}

// ApplyOptions applies request options (minimal implementation)
func (r *Request) ApplyOptions(opts ...interface{}) {
	// Minimal implementation
}

// Operation represents an API operation
type Operation struct {
	Name       string
	HTTPMethod string
	HTTPPath   string
}

// newRequest creates a new request
func (c *Ssmmds) newRequest(op *Operation, params, data interface{}) *Request {
	body, _ := json.Marshal(params)

	req, err := http.NewRequest(op.HTTPMethod, c.endpoint+op.HTTPPath, bytes.NewReader(body))
	if err != nil {
		return &Request{Error: err}
	}

	// Set headers
	req.Header.Set("Content-Type", "application/x-amz-json-1.1")
	req.Header.Set("X-Amz-Target", "EC2WindowsMessageDeliveryService."+op.Name)
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}

	// Retrieve fresh credentials and sign request
	if err := c.signRequest(req, body); err != nil {
		return &Request{Error: err}
	}

	return &Request{
		HTTPRequest:   req,
		Data:          data,
		httpClient:    c.client,
		body:          body,
		operationName: op.Name,
	}
}

// signRequest signs the HTTP request using AWS Signature Version 4
func (c *Ssmmds) signRequest(req *http.Request, body []byte) error {
	creds, err := c.credentialProvider.Retrieve(context.TODO())
	if err != nil {
		return fmt.Errorf("failed to retrieve credentials: %w", err)
	}

	hasher := sha256.New()
	hasher.Write(body)
	payloadHash := hex.EncodeToString(hasher.Sum(nil))

	signer := v4.NewSigner()
	return signer.SignHTTP(context.TODO(), creds, req, payloadHash, EndpointsID, c.region, time.Now().UTC())
}

// executeRequest executes HTTP request and handles response with retry
func (c *Ssmmds) executeRequest(req *Request, output interface{}) error {
	if req.Error != nil {
		return req.Error
	}

	ctx := req.context
	if ctx == nil {
		ctx = context.Background()
	}

	const maxAttempts = 4
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			var delay time.Duration
			// GetMessages is a long-poll operation that should recover quickly
			// after a timeout. When the previous failure was a timeout, use a
			// short fixed 100ms retry delay instead of exponential backoff so
			// the agent resumes polling for messages promptly.
			if req.operationName == opGetMessages && isTimeoutError(lastErr) {
				delay = 100 * time.Millisecond
			} else {
				// Exponential backoff with jitter: base 1s × 2^attempt + random 0-500ms
				backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
				jitter := time.Duration(rand.Intn(500)) * time.Millisecond
				delay = backoff + jitter
			}
			time.Sleep(delay)

			// Reset body and re-sign the request since the signature has a timestamp
			req.HTTPRequest.Body = io.NopCloser(bytes.NewReader(req.body))
			if err := c.signRequest(req.HTTPRequest, req.body); err != nil {
				return err
			}
		}

		httpReq := req.HTTPRequest.WithContext(ctx)
		resp, err := c.client.Do(httpReq)
		if err != nil {
			lastErr = err
			continue // network error — retry
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode >= 400 {
			lastErr = parseError(body, resp.StatusCode)
			if isRetryableError(lastErr, resp.StatusCode) {
				continue
			}
			return lastErr // non-retryable API error
		}

		// Success
		if len(body) > 0 && output != nil {
			return json.Unmarshal(body, output)
		}
		return nil
	}

	return fmt.Errorf("exceeded maximum number of attempts (%d): %w", maxAttempts, lastErr)
}

// isRetryableError returns true for transient/throttle errors that should be retried
func isRetryableError(err error, statusCode int) bool {
	if statusCode == 429 || statusCode == 500 || statusCode == 502 || statusCode == 503 || statusCode == 504 {
		return true
	}
	switch err.(type) {
	case *InternalServerException, *RequestTimeoutException, *TooManyRequestsException:
		return true
	}
	return false
}

// isTimeoutError returns true if the error is a network/transport timeout or a
// service-side RequestTimeoutException.
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	if _, ok := err.(*RequestTimeoutException); ok {
		return true
	}
	return strings.Contains(err.Error(), "Client.Timeout") || strings.Contains(err.Error(), "timeout")
}

// parseError parses AWS error response
func parseError(body []byte, statusCode int) error {
	var errResp struct {
		Type    string `json:"__type"`
		Message string `json:"message"`
	}

	if err := json.Unmarshal(body, &errResp); err != nil {
		return fmt.Errorf("HTTP %d: %s", statusCode, string(body))
	}

	switch errResp.Type {
	case "AuthorizationFailureException":
		return &AuthorizationFailureException{Message_: &errResp.Message, statusCode: statusCode}
	case "InternalServerException":
		return &InternalServerException{Message_: &errResp.Message, statusCode: statusCode}
	case "InvalidDestinationException":
		return &InvalidDestinationException{Message_: &errResp.Message, statusCode: statusCode}
	case "InvalidMessageIdException":
		return &InvalidMessageIdException{Message_: &errResp.Message, statusCode: statusCode}
	case "RequestTimeoutException":
		return &RequestTimeoutException{Message_: &errResp.Message, statusCode: statusCode}
	case "TooManyRequestsException":
		return &TooManyRequestsException{Message_: &errResp.Message, statusCode: statusCode}
	case "UnsupportedMessageOperationException":
		return &UnsupportedMessageOperationException{Message_: &errResp.Message, statusCode: statusCode}
	default:
		return fmt.Errorf("%s: %s", errResp.Type, errResp.Message)
	}
}

// Data structures
type AcknowledgeMessageInput struct {
	MessageId *string `json:"MessageId"`
}

func (s *AcknowledgeMessageInput) SetMessageId(v string) *AcknowledgeMessageInput {
	s.MessageId = &v
	return s
}

type AcknowledgeMessageOutput struct{}

type DeleteMessageInput struct {
	MessageId *string `json:"MessageId"`
}

func (s *DeleteMessageInput) SetMessageId(v string) *DeleteMessageInput {
	s.MessageId = &v
	return s
}

type DeleteMessageOutput struct{}

type FailMessageInput struct {
	FailureType *string `json:"FailureType"`
	MessageId   *string `json:"MessageId"`
}

func (s *FailMessageInput) SetFailureType(v string) *FailMessageInput {
	s.FailureType = &v
	return s
}

func (s *FailMessageInput) SetMessageId(v string) *FailMessageInput {
	s.MessageId = &v
	return s
}

type FailMessageOutput struct{}

type GetEndpointInput struct {
	Destination *string `json:"Destination"`
}

func (s *GetEndpointInput) SetDestination(v string) *GetEndpointInput {
	s.Destination = &v
	return s
}

type GetEndpointOutput struct {
	Endpoint *string `json:"Endpoint"`
}

func (s *GetEndpointOutput) SetEndpoint(v string) *GetEndpointOutput {
	s.Endpoint = &v
	return s
}

type GetMessagesInput struct {
	Destination                *string `json:"Destination"`
	MessagesRequestId          *string `json:"MessagesRequestId"`
	NextToken                  *string `json:"NextToken,omitempty"`
	VisibilityTimeoutInSeconds *int64  `json:"VisibilityTimeoutInSeconds,omitempty"`
}

func (s *GetMessagesInput) SetDestination(v string) *GetMessagesInput {
	s.Destination = &v
	return s
}

func (s *GetMessagesInput) SetMessagesRequestId(v string) *GetMessagesInput {
	s.MessagesRequestId = &v
	return s
}

func (s *GetMessagesInput) SetNextToken(v string) *GetMessagesInput {
	s.NextToken = &v
	return s
}

func (s *GetMessagesInput) SetVisibilityTimeoutInSeconds(v int64) *GetMessagesInput {
	s.VisibilityTimeoutInSeconds = &v
	return s
}

type GetMessagesOutput struct {
	Destination       *string    `json:"Destination"`
	Messages          []*Message `json:"Messages"`
	MessagesRequestId *string    `json:"MessagesRequestId"`
}

func (s *GetMessagesOutput) SetDestination(v string) *GetMessagesOutput {
	s.Destination = &v
	return s
}

func (s *GetMessagesOutput) SetMessages(v []*Message) *GetMessagesOutput {
	s.Messages = v
	return s
}

func (s *GetMessagesOutput) SetMessagesRequestId(v string) *GetMessagesOutput {
	s.MessagesRequestId = &v
	return s
}

type Message struct {
	CreatedDate   *string `json:"CreatedDate"`
	Destination   *string `json:"Destination"`
	MessageId     *string `json:"MessageId"`
	NextToken     *string `json:"NextToken"`
	Payload       *string `json:"Payload"`
	PayloadDigest *string `json:"PayloadDigest"`
	Topic         *string `json:"Topic"`
}

func (s *Message) SetCreatedDate(v string) *Message {
	s.CreatedDate = &v
	return s
}

func (s *Message) SetDestination(v string) *Message {
	s.Destination = &v
	return s
}

func (s *Message) SetMessageId(v string) *Message {
	s.MessageId = &v
	return s
}

func (s *Message) SetNextToken(v string) *Message {
	s.NextToken = &v
	return s
}

func (s *Message) SetPayload(v string) *Message {
	s.Payload = &v
	return s
}

func (s *Message) SetPayloadDigest(v string) *Message {
	s.PayloadDigest = &v
	return s
}

func (s *Message) SetTopic(v string) *Message {
	s.Topic = &v
	return s
}

type SendReplyInput struct {
	DeliveryOptions *DeliveryOptions `json:"DeliveryOptions,omitempty"`
	MessageId       *string          `json:"MessageId"`
	Payload         *string          `json:"Payload"`
	ReplyId         *string          `json:"ReplyId"`
}

func (s *SendReplyInput) SetDeliveryOptions(v *DeliveryOptions) *SendReplyInput {
	s.DeliveryOptions = v
	return s
}

func (s *SendReplyInput) SetMessageId(v string) *SendReplyInput {
	s.MessageId = &v
	return s
}

func (s *SendReplyInput) SetPayload(v string) *SendReplyInput {
	s.Payload = &v
	return s
}

func (s *SendReplyInput) SetReplyId(v string) *SendReplyInput {
	s.ReplyId = &v
	return s
}

type SendReplyOutput struct {
	Description *string `json:"Description"`
	MessageId   *string `json:"MessageId"`
	ReplyId     *string `json:"ReplyId"`
	ReplyStatus *string `json:"ReplyStatus"`
}

func (s *SendReplyOutput) SetDescription(v string) *SendReplyOutput {
	s.Description = &v
	return s
}

func (s *SendReplyOutput) SetMessageId(v string) *SendReplyOutput {
	s.MessageId = &v
	return s
}

func (s *SendReplyOutput) SetReplyId(v string) *SendReplyOutput {
	s.ReplyId = &v
	return s
}

func (s *SendReplyOutput) SetReplyStatus(v string) *SendReplyOutput {
	s.ReplyStatus = &v
	return s
}

type DeliveryOptions struct {
	ExpiresAfter   *string `json:"ExpiresAfter,omitempty"`
	ReplacementKey *string `json:"ReplacementKey,omitempty"`
	SchedulePush   *string `json:"SchedulePush,omitempty"`
}

func (s *DeliveryOptions) SetExpiresAfter(v string) *DeliveryOptions {
	s.ExpiresAfter = &v
	return s
}

func (s *DeliveryOptions) SetReplacementKey(v string) *DeliveryOptions {
	s.ReplacementKey = &v
	return s
}

func (s *DeliveryOptions) SetSchedulePush(v string) *DeliveryOptions {
	s.SchedulePush = &v
	return s
}

// API Operations
const (
	opAcknowledgeMessage = "AcknowledgeMessage"
	opDeleteMessage      = "DeleteMessage"
	opFailMessage        = "FailMessage"
	opGetEndpoint        = "GetEndpoint"
	opGetMessages        = "GetMessages"
	opSendReply          = "SendReply"
)

func (c *Ssmmds) AcknowledgeMessageRequest(input *AcknowledgeMessageInput) (*Request, *AcknowledgeMessageOutput) {
	op := &Operation{Name: opAcknowledgeMessage, HTTPMethod: "POST", HTTPPath: "/"}
	if input == nil {
		input = &AcknowledgeMessageInput{}
	}
	output := &AcknowledgeMessageOutput{}
	return c.newRequest(op, input, output), output
}

func (c *Ssmmds) AcknowledgeMessage(input *AcknowledgeMessageInput) (*AcknowledgeMessageOutput, error) {
	req, out := c.AcknowledgeMessageRequest(input)
	return out, c.executeRequest(req, out)
}

func (c *Ssmmds) AcknowledgeMessageWithContext(ctx context.Context, input *AcknowledgeMessageInput, opts ...interface{}) (*AcknowledgeMessageOutput, error) {
	req, out := c.AcknowledgeMessageRequest(input)
	req.SetContext(ctx)
	return out, c.executeRequest(req, out)
}

func (c *Ssmmds) DeleteMessageRequest(input *DeleteMessageInput) (*Request, *DeleteMessageOutput) {
	op := &Operation{Name: opDeleteMessage, HTTPMethod: "POST", HTTPPath: "/"}
	if input == nil {
		input = &DeleteMessageInput{}
	}
	output := &DeleteMessageOutput{}
	return c.newRequest(op, input, output), output
}

func (c *Ssmmds) DeleteMessage(input *DeleteMessageInput) (*DeleteMessageOutput, error) {
	req, out := c.DeleteMessageRequest(input)
	return out, c.executeRequest(req, out)
}

func (c *Ssmmds) DeleteMessageWithContext(ctx context.Context, input *DeleteMessageInput, opts ...interface{}) (*DeleteMessageOutput, error) {
	req, out := c.DeleteMessageRequest(input)
	req.SetContext(ctx)
	return out, c.executeRequest(req, out)
}

func (c *Ssmmds) FailMessageRequest(input *FailMessageInput) (*Request, *FailMessageOutput) {
	op := &Operation{Name: opFailMessage, HTTPMethod: "POST", HTTPPath: "/"}
	if input == nil {
		input = &FailMessageInput{}
	}
	output := &FailMessageOutput{}
	return c.newRequest(op, input, output), output
}

func (c *Ssmmds) FailMessage(input *FailMessageInput) (*FailMessageOutput, error) {
	req, out := c.FailMessageRequest(input)
	return out, c.executeRequest(req, out)
}

func (c *Ssmmds) FailMessageWithContext(ctx context.Context, input *FailMessageInput, opts ...interface{}) (*FailMessageOutput, error) {
	req, out := c.FailMessageRequest(input)
	req.SetContext(ctx)
	return out, c.executeRequest(req, out)
}

func (c *Ssmmds) GetEndpointRequest(input *GetEndpointInput) (*Request, *GetEndpointOutput) {
	op := &Operation{Name: opGetEndpoint, HTTPMethod: "POST", HTTPPath: "/"}
	if input == nil {
		input = &GetEndpointInput{}
	}
	output := &GetEndpointOutput{}
	return c.newRequest(op, input, output), output
}

func (c *Ssmmds) GetEndpoint(input *GetEndpointInput) (*GetEndpointOutput, error) {
	req, out := c.GetEndpointRequest(input)
	return out, c.executeRequest(req, out)
}

func (c *Ssmmds) GetEndpointWithContext(ctx context.Context, input *GetEndpointInput, opts ...interface{}) (*GetEndpointOutput, error) {
	req, out := c.GetEndpointRequest(input)
	req.SetContext(ctx)
	return out, c.executeRequest(req, out)
}

func (c *Ssmmds) GetMessagesRequest(input *GetMessagesInput) (*Request, *GetMessagesOutput) {
	op := &Operation{Name: opGetMessages, HTTPMethod: "POST", HTTPPath: "/"}
	if input == nil {
		input = &GetMessagesInput{}
	}
	output := &GetMessagesOutput{}
	return c.newRequest(op, input, output), output
}

func (c *Ssmmds) GetMessages(input *GetMessagesInput) (*GetMessagesOutput, error) {
	req, out := c.GetMessagesRequest(input)
	return out, c.executeRequest(req, out)
}

func (c *Ssmmds) GetMessagesWithContext(ctx context.Context, input *GetMessagesInput, opts ...interface{}) (*GetMessagesOutput, error) {
	req, out := c.GetMessagesRequest(input)
	req.SetContext(ctx)
	return out, c.executeRequest(req, out)
}

func (c *Ssmmds) SendReplyRequest(input *SendReplyInput) (*Request, *SendReplyOutput) {
	op := &Operation{Name: opSendReply, HTTPMethod: "POST", HTTPPath: "/"}
	if input == nil {
		input = &SendReplyInput{}
	}
	output := &SendReplyOutput{}
	return c.newRequest(op, input, output), output
}

func (c *Ssmmds) SendReply(input *SendReplyInput) (*SendReplyOutput, error) {
	req, out := c.SendReplyRequest(input)
	return out, c.executeRequest(req, out)
}

func (c *Ssmmds) SendReplyWithContext(ctx context.Context, input *SendReplyInput, opts ...interface{}) (*SendReplyOutput, error) {
	req, out := c.SendReplyRequest(input)
	req.SetContext(ctx)
	return out, c.executeRequest(req, out)
}

type InvalidDestinationException struct {
	Message_   *string
	statusCode int
}

func (e *InvalidDestinationException) Error() string {
	if e.Message_ != nil {
		return *e.Message_
	}
	return "InvalidDestinationException"
}

func (e *InvalidDestinationException) Code() string {
	return "InvalidDestinationException"
}

func (e *InvalidDestinationException) Message() string {
	if e.Message_ != nil {
		return *e.Message_
	}
	return ""
}

func (e *InvalidDestinationException) StatusCode() int {
	return e.statusCode
}

type RequestTimeoutException struct {
	Message_   *string
	statusCode int
}

func (e *RequestTimeoutException) Error() string {
	if e.Message_ != nil {
		return *e.Message_
	}
	return "RequestTimeoutException"
}

func (e *RequestTimeoutException) Code() string {
	return "RequestTimeoutException"
}

func (e *RequestTimeoutException) Message() string {
	if e.Message_ != nil {
		return *e.Message_
	}
	return ""
}

func (e *RequestTimeoutException) StatusCode() int {
	return e.statusCode
}

type TooManyRequestsException struct {
	Message_   *string
	statusCode int
}

func (e *TooManyRequestsException) Error() string {
	if e.Message_ != nil {
		return *e.Message_
	}
	return "TooManyRequestsException"
}

func (e *TooManyRequestsException) Code() string {
	return "TooManyRequestsException"
}

func (e *TooManyRequestsException) Message() string {
	if e.Message_ != nil {
		return *e.Message_
	}
	return ""
}

func (e *TooManyRequestsException) StatusCode() int {
	return e.statusCode
}

// Error types
type AuthorizationFailureException struct {
	Message_   *string
	statusCode int
}

func (e *AuthorizationFailureException) Error() string {
	if e.Message_ != nil {
		return *e.Message_
	}
	return "AuthorizationFailureException"
}

func (e *AuthorizationFailureException) Code() string {
	return "AuthorizationFailureException"
}

func (e *AuthorizationFailureException) Message() string {
	if e.Message_ != nil {
		return *e.Message_
	}
	return ""
}

func (e *AuthorizationFailureException) StatusCode() int {
	return e.statusCode
}

type InternalServerException struct {
	Message_   *string
	statusCode int
}

func (e *InternalServerException) Error() string {
	if e.Message_ != nil {
		return *e.Message_
	}
	return "InternalServerException"
}

func (e *InternalServerException) Code() string {
	return "InternalServerException"
}

func (e *InternalServerException) Message() string {
	if e.Message_ != nil {
		return *e.Message_
	}
	return ""
}

func (e *InternalServerException) StatusCode() int {
	return e.statusCode
}

type InvalidMessageIdException struct {
	Message_   *string
	statusCode int
}

func (e *InvalidMessageIdException) Error() string {
	if e.Message_ != nil {
		return *e.Message_
	}
	return "InvalidMessageIdException"
}

func (e *InvalidMessageIdException) Code() string {
	return "InvalidMessageIdException"
}

func (e *InvalidMessageIdException) Message() string {
	if e.Message_ != nil {
		return *e.Message_
	}
	return ""
}

func (e *InvalidMessageIdException) StatusCode() int {
	return e.statusCode
}

type UnsupportedMessageOperationException struct {
	Message_   *string
	statusCode int
}

func (e *UnsupportedMessageOperationException) Error() string {
	if e.Message_ != nil {
		return *e.Message_
	}
	return "UnsupportedMessageOperationException"
}

func (e *UnsupportedMessageOperationException) Code() string {
	return "UnsupportedMessageOperationException"
}

func (e *UnsupportedMessageOperationException) Message() string {
	if e.Message_ != nil {
		return *e.Message_
	}
	return ""
}

func (e *UnsupportedMessageOperationException) StatusCode() int {
	return e.statusCode
}

// Constants
const (
	FailureTypeNoHandlerExists          = "NoHandlerExists"
	FailureTypeInternalHandlerException = "InternalHandlerException"
	ReplyStatusCreated                  = "Created"
	ReplyStatusQueued                   = "Queued"
	ReplyStatusAcknowledged             = "Acknowledged"
	ReplyStatusNoActionTaken            = "NoActionTaken"
	SchedulePushEventually              = "EVENTUALLY"
	SchedulePushAsap                    = "ASAP"
)
