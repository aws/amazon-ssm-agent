// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package s3util

import (
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	contextmocks "github.com/aws/amazon-ssm-agent/agent/mocks/context"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	identityMocks "github.com/aws/amazon-ssm-agent/common/identity/mocks"
	"github.com/aws/aws-sdk-go-v2/aws"

	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockedHttpProvider struct {
	mock.Mock
}

func (m *MockedHttpProvider) Head(url string) (*http.Response, error) {
	args := m.Called(url)
	return args.Get(0).(*http.Response), args.Error(1)
}

func setBucketRegionFromSignedHeadBucketRequest(bucketRegion string) {
	getBucketRegionFromSignedHeadBucketRequestFunc = func(context context.T, region, regionalEndpoint, bucketName string) string {
		return bucketRegion
	}
}

func setS3Endpoint(region, endpoint string, err error) {
	getS3Endpoint = func(context context.T, region string) (string, error) {
		return endpoint, err
	}
}

func setS3FallbackEndpoint(region, endpoint string) {
	getFallbackS3EndpointFunc = func(context context.T, region string) string {
		return endpoint
	}
}

func TestBucketRegion_WithHeadBucketRequestSuccessful(t *testing.T) {
	setBucketRegionFromSignedHeadBucketRequest("us-east-1")
	setS3Endpoint("us-east-1", "", fmt.Errorf("invalid region"))

	resp := &http.Response{
		StatusCode: 200,
		Header: http.Header{
			bucketRegionHeader: []string{"us-east-1"},
		},
	}
	var err error = nil
	httpProvider := &MockedHttpProvider{}
	httpProvider.On("Head", "https://bucket-1.s3.amazonaws.com").Return(resp, err)
	actual := getBucketRegion(contextmocks.NewMockDefault(), "us-east-1", "bucket-1", httpProvider)
	expected := "us-east-1"
	assert.Equal(t, expected, actual)
}

func TestGetBucketRegion_NoError_InvalidS3Endpoint_ReturnsRegionFromFallback(t *testing.T) {
	setBucketRegionFromSignedHeadBucketRequest("")
	setS3Endpoint("us-east-1", "", fmt.Errorf("invalid region"))
	setS3FallbackEndpoint("us-east-1", "s3.amazonaws.com")

	resp := &http.Response{
		StatusCode: 200,
		Header: http.Header{
			bucketRegionHeader: []string{"us-east-1"},
		},
	}
	var err error = nil
	httpProvider := &MockedHttpProvider{}
	httpProvider.On("Head", "https://bucket-1.s3.amazonaws.com").Return(resp, err)
	actual := getBucketRegion(contextmocks.NewMockDefault(), "us-east-1", "bucket-1", httpProvider)
	expected := "us-east-1"
	assert.Equal(t, expected, actual)
}

func TestGetBucketRegion_NoError_InvalidFallbackS3Endpoint_ReturnsRegionFroms3(t *testing.T) {
	setBucketRegionFromSignedHeadBucketRequest("")
	setS3Endpoint("us-east-1", "s3.us-east-1.amazonaws.com", nil)
	setS3FallbackEndpoint("us-east-1", "")
	resp := &http.Response{
		StatusCode: 200,
		Header: http.Header{
			bucketRegionHeader: []string{"us-east-1"},
		},
	}
	var err error = nil
	httpProvider := &MockedHttpProvider{}
	httpProvider.On("Head", "https://bucket-1.s3.us-east-1.amazonaws.com").Return(resp, err)
	actual := getBucketRegion(contextmocks.NewMockDefault(), "us-east-1", "bucket-1", httpProvider)
	expected := "us-east-1"
	assert.Equal(t, expected, actual)
}

func TestGetBucketRegion_NoError_InvalidS3Endpoint_Error(t *testing.T) {
	setBucketRegionFromSignedHeadBucketRequest("")
	setS3Endpoint("us-east-1", "", fmt.Errorf("invalid region"))
	setS3FallbackEndpoint("us-east-1", "")
	resp := &http.Response{
		StatusCode: 200,
		Header: http.Header{
			bucketRegionHeader: []string{"us-east-1"},
		},
	}
	var err error = nil
	httpProvider := &MockedHttpProvider{}
	httpProvider.On("Head", "https://bucket-1.s3.us-east-1.amazonaws.com").Return(resp, err)
	actual := getBucketRegion(contextmocks.NewMockDefault(), "us-east-1", "bucket-1", httpProvider)
	assert.Equal(t, "", actual)
}

func TestGetBucketRegion_NoError_NoRegionInResponse_ReturnsEmptyString(t *testing.T) {
	setBucketRegionFromSignedHeadBucketRequest("")
	setS3Endpoint("us-east-1", "s3.us-east-1.amazonaws.com", nil)
	setS3FallbackEndpoint("us-east-1", "s3.amazonaws.com")
	resp := &http.Response{
		StatusCode: 401,
	}
	var err error = nil
	httpProvider := &MockedHttpProvider{}
	httpProvider.On("Head", "https://bucket-1.s3.us-east-1.amazonaws.com").Return(resp, err)
	httpProvider.On("Head", "https://bucket-1.s3.amazonaws.com").Return(resp, err)
	actual := getBucketRegion(contextmocks.NewMockDefault(), "us-east-1", "bucket-1", httpProvider)
	assert.Equal(t, "", actual)
}

func TestGetBucketRegion_NoError_RegionInResponse_ReturnsRegion(t *testing.T) {
	setBucketRegionFromSignedHeadBucketRequest("")
	setS3Endpoint("us-east-1", "s3.us-east-1.amazonaws.com", nil)
	setS3FallbackEndpoint("us-east-1", "s3.amazonaws.com")
	resp := &http.Response{
		StatusCode: 301,
		Header: http.Header{
			bucketRegionHeader: []string{"eu-west-1"},
		},
	}
	var err error = nil
	httpProvider := &MockedHttpProvider{}
	httpProvider.On("Head", "https://bucket-1.s3.us-east-1.amazonaws.com").Return(resp, err)
	actual := getBucketRegion(contextmocks.NewMockDefault(), "us-east-1", "bucket-1", httpProvider)
	assert.Equal(t, "eu-west-1", actual)
}

func TestGetBucketRegion_AllUrlsFail_ReturnsEmptyString(t *testing.T) {
	setBucketRegionFromSignedHeadBucketRequest("")
	setS3Endpoint("us-east-1", "s3.us-east-1.amazonaws.com", nil)
	setS3FallbackEndpoint("us-east-1", "s3.amazonaws.com")
	var resp *http.Response = nil
	err := fmt.Errorf("failed")
	httpProvider := &MockedHttpProvider{}
	httpProvider.On("Head", "https://bucket-1.s3.us-east-1.amazonaws.com").Return(resp, err)
	httpProvider.On("Head", "https://bucket-1.s3.amazonaws.com").Return(resp, err)
	httpProvider.On("Head", "http://bucket-1.s3.us-east-1.amazonaws.com").Return(resp, err)
	httpProvider.On("Head", "http://bucket-1.s3.amazonaws.com").Return(resp, err)
	actual := getBucketRegion(contextmocks.NewMockDefault(), "us-east-1", "bucket-1", httpProvider)
	assert.Equal(t, "", actual)
	httpProvider.AssertExpectations(t)
}

func TestGetS3CrossRegionCapableSession_regionFromHead_noConfigOverrides(t *testing.T) {
	setBucketRegionFromSignedHeadBucketRequest("")
	setupMocksForGetS3CrossRegionCapableSession("us-east-1", "bucket-1", "eu-west-1")
	config, err := GetS3CrossRegionCapableSession(contextmocks.NewMockDefault(), "bucket-1")
	assert.NotNil(t, config)
	assert.Equal(t, "eu-west-1", config.Region)
	assert.Nil(t, config.BaseEndpoint)
	httpClient := config.HTTPClient.(*http.Client)
	assert.NotNil(t, httpClient.Transport)
	_, correctType := httpClient.Transport.(*s3BucketRegionHeaderCapturingTransport)
	assert.True(t, correctType)
	assert.Nil(t, err)
}

func TestGetS3CrossRegionCapableSession_noRegionFromHead_noConfigOverrides(t *testing.T) {
	setBucketRegionFromSignedHeadBucketRequest("")
	identityMock := &identityMocks.IAgentIdentity{}
	identityMock.On("Region").Return("cn-north-1", nil)

	contextMock := new(contextmocks.Mock)
	contextMock.On("Identity").Return(identityMock)
	contextMock.On("Log").Return(logmocks.NewMockLog())
	contextMock.On("AppConfig").Return(appconfig.DefaultConfig())

	setupMocksForGetS3CrossRegionCapableSession("cn-north-1", "bucket-1", "")
	config, err := GetS3CrossRegionCapableSession(contextMock, "bucket-1")
	assert.NotNil(t, config)
	assert.Equal(t, "cn-north-1", config.Region)
	assert.Nil(t, config.BaseEndpoint)
	httpClient := config.HTTPClient.(*http.Client)
	assert.NotNil(t, httpClient)
	assert.NotNil(t, httpClient.Transport)
	_, correctType := httpClient.Transport.(*s3BucketRegionHeaderCapturingTransport)
	assert.True(t, correctType)
	assert.Nil(t, err)
}

func TestGetS3CrossRegionCapableSession_regionFromHead_withConfigOverrides(t *testing.T) {
	setBucketRegionFromSignedHeadBucketRequest("")
	appConfig := appconfig.DefaultConfig()
	appConfig.S3.Endpoint = "https://custom.endpoint.com"

	identityMock := &identityMocks.IAgentIdentity{}
	identityMock.On("Region").Return("us-east-1", nil)

	contextMock := new(contextmocks.Mock)
	contextMock.On("Identity").Return(identityMock)
	contextMock.On("Log").Return(logmocks.NewMockLog())
	contextMock.On("AppConfig").Return(appConfig)

	setupMocksForGetS3CrossRegionCapableSession("us-east-1", "bucket-1", "eu-west-1")
	config, err := GetS3CrossRegionCapableSession(contextMock, "bucket-1")
	assert.NotNil(t, config)
	assert.Equal(t, "eu-west-1", config.Region)
	assert.Equal(t, "https://custom.endpoint.com", *config.BaseEndpoint)
	httpClient := config.HTTPClient.(*http.Client)
	assert.NotNil(t, httpClient.Transport)
	_, correctType := httpClient.Transport.(*s3BucketRegionHeaderCapturingTransport)
	assert.True(t, correctType)
	assert.Nil(t, err)
}

func TestGetS3CrossRegionCapableSession_noRegionFromHead_withConfigOverrides(t *testing.T) {
	setBucketRegionFromSignedHeadBucketRequest("")
	appConfig := appconfig.DefaultConfig()
	appConfig.S3.Endpoint = "https://custom.endpoint.com.cn"

	identityMock := &identityMocks.IAgentIdentity{}
	identityMock.On("Region").Return("cn-north-1", nil)

	contextMock := new(contextmocks.Mock)
	contextMock.On("Identity").Return(identityMock)
	contextMock.On("Log").Return(logmocks.NewMockLog())
	contextMock.On("AppConfig").Return(appConfig)

	setupMocksForGetS3CrossRegionCapableSession("cn-north-1", "bucket-1", "")
	config, err := GetS3CrossRegionCapableSession(contextMock, "bucket-1")
	assert.NotNil(t, config)
	assert.Equal(t, "cn-north-1", config.Region)
	assert.Equal(t, "https://custom.endpoint.com.cn", *config.BaseEndpoint)
	httpClient := config.HTTPClient.(*http.Client)
	assert.NotNil(t, httpClient.Transport)
	_, correctType := httpClient.Transport.(*s3BucketRegionHeaderCapturingTransport)
	assert.True(t, correctType)
	assert.Nil(t, err)
}

func setupMocksForGetS3CrossRegionCapableSession(instanceRegion, bucketName, headBucketResponse string) {
	setBucketRegionFromSignedHeadBucketRequest("")
	setupMockHeadBucketResponse(bucketName, instanceRegion, headBucketResponse)
	makeAwsConfig = func(context context.T, service, region string) aws.Config {
		return aws.Config{
			Region: region,
		}
	}
}

func setupMockHeadBucketResponse(bucketName, instanceRegion, headBucketResponse string) {
	setBucketRegionFromSignedHeadBucketRequest("")
	s3Endpoint := "s3." + instanceRegion + ".amazonaws.com"
	s3FallbackEndpoint := "s3.amazonaws.com"
	if strings.HasPrefix(instanceRegion, "cn-") {
		s3Endpoint += ".cn"
		s3FallbackEndpoint = "s3.cn-north-1.amazonaws.com.cn"
	}
	setS3Endpoint(instanceRegion, s3Endpoint, nil)
	setS3FallbackEndpoint(instanceRegion, s3FallbackEndpoint)

	getHttpProvider = func(log.T, appconfig.SsmagentConfig) HttpProvider {
		provider := &MockedHttpProvider{}
		resp := &http.Response{
			Header: http.Header{},
		}
		var err error = nil
		if headBucketResponse != "" {
			resp.Header.Add(bucketRegionHeader, headBucketResponse)
		}
		provider.On("Head", "https://"+bucketName+"."+s3Endpoint).Return(resp, err)
		provider.On("Head", "https://"+bucketName+"."+s3FallbackEndpoint).Return(resp, err)
		return provider
	}
}

func TestNewS3BucketRegionHeaderCapturingTransport(t *testing.T) {
	transport := newS3BucketRegionHeaderCapturingTransport(logmocks.NewMockLog(), appconfig.SsmagentConfig{})
	_, goodType := transport.delegate.(*http.Transport)
	assert.True(t, goodType)
}

func TestRoundTrip_bucketRegionHeaderPresent(t *testing.T) {
	setBucketRegionFromSignedHeadBucketRequest("")
	requestUrl := "https://test-bucket.s3.cn-northwest-1.amazonaws.com.cn/a/b"
	request := makeRequest("GET", requestUrl)

	responseHeader := http.Header{}
	responseHeader.Add(bucketRegionHeader, "cn-north-1")
	responseHeader.Add("x-amz-request-id", "123")
	responseBodyContents := makeRedirectResponseBodyText("test-bucket.s3.cn-north-1.amazonaws.com.cn", "test-bucket")
	response := makeResponse(301, responseHeader, responseBodyContents)

	delegate := newMockTransport()
	delegate.AddResponse(requestUrl, response)

	transport := newS3BucketRegionHeaderCapturingTransportForTest(delegate)
	actualResponse, err := transport.RoundTrip(request)
	assert.NotNil(t, actualResponse)
	assert.Nil(t, err)

	cachedRegion, ok := getBucketRegionMap().Get("test-bucket")
	assert.True(t, ok)
	assert.Equal(t, "cn-north-1", cachedRegion)

	// Cleanup
	getBucketRegionMap().Remove("test-bucket")
}

func TestRoundTrip_bucketRegionInErrorResponseBody(t *testing.T) {
	setBucketRegionFromSignedHeadBucketRequest("")
	requestUrl := "https://test-bucket.s3.cn-northwest-1.amazonaws.com.cn/a/b"
	request := makeRequest("GET", requestUrl)

	responseHeader := http.Header{}
	responseBodyContents := makeAuthorizationHeaderMalformedErrorResponse("cn-northwest-1", "cn-north-1")
	response := makeResponse(400, responseHeader, responseBodyContents)

	delegate := newMockTransport()
	delegate.AddResponse(requestUrl, response)

	transport := newS3BucketRegionHeaderCapturingTransportForTest(delegate)
	actualResponse, err := transport.RoundTrip(request)
	assert.NotNil(t, actualResponse)
	assert.Nil(t, err)

	cachedRegion, ok := getBucketRegionMap().Get("test-bucket")
	assert.True(t, ok)
	assert.Equal(t, "cn-north-1", cachedRegion)

	// Cleanup
	getBucketRegionMap().Remove("test-bucket")
}

func TestRoundTrip_endpointInErrorResponseBody(t *testing.T) {
	setBucketRegionFromSignedHeadBucketRequest("")
	requestUrl := "https://test-bucket.s3.cn-northwest-1.amazonaws.com.cn/a/b"
	request := makeRequest("GET", requestUrl)

	responseHeader := http.Header{}
	responseBodyContents := makeRedirectResponseBodyText("test-bucket.s3.cn-north-1.amazonaws.com.cn", "test-bucket")
	response := makeResponse(301, responseHeader, responseBodyContents)

	delegate := newMockTransport()
	delegate.AddResponse(requestUrl, response)

	transport := newS3BucketRegionHeaderCapturingTransportForTest(delegate)
	actualResponse, err := transport.RoundTrip(request)
	assert.NotNil(t, actualResponse)
	assert.Nil(t, err)

	cachedRegion, ok := getBucketRegionMap().Get("test-bucket")
	assert.True(t, ok)
	assert.Equal(t, "cn-north-1", cachedRegion)

	// Cleanup
	getBucketRegionMap().Remove("test-bucket")
}

func TestRoundTrip_bucketRegionNotPresent(t *testing.T) {
	setBucketRegionFromSignedHeadBucketRequest("")
	requestUrl := "https://test-bucket.s3.cn-north-1.amazonaws.com.cn/a/b"
	request := makeRequest("GET", requestUrl)
	response := makeResponse(200, http.Header{}, "Success")
	delegate := newMockTransport()
	delegate.AddResponse(requestUrl, response)

	transport := newS3BucketRegionHeaderCapturingTransportForTest(delegate)
	actualResponse, err := transport.RoundTrip(request)
	assert.NotNil(t, actualResponse)
	assert.Nil(t, err)
	assert.Equal(t, actualResponse.StatusCode, 200)

	_, ok := getBucketRegionMap().Get("test-bucket")
	assert.False(t, ok)

	// Cleanup
	getBucketRegionMap().Remove("test-bucket")
}

func TestRoundTrip_error(t *testing.T) {
	setBucketRegionFromSignedHeadBucketRequest("")
	requestUrl := "https://test-bucket.s3.cn-north-1.amazonaws.com.cn/a/b"
	request := makeRequest("GET", requestUrl)
	delegate := newMockTransport()

	transport := newS3BucketRegionHeaderCapturingTransportForTest(delegate)
	actualResponse, err := transport.RoundTrip(request)
	assert.Nil(t, actualResponse)
	assert.NotNil(t, err)
}

func TestBucketRegionCache_keepsNMostRecentItems(t *testing.T) {
	for i := 0; i < 2*bucketRegionCacheItemCountMax; i++ {
		bucketName := fmt.Sprintf("bucket-%d", i)
		getBucketRegionMap().Put(bucketName, "us-east-1")
	}

	// Only the most-recently-added bucketRegionCacheItemCountMax items should be in the cache
	assert.Equal(t, uint64(bucketRegionCacheItemCountMax), getBucketRegionMap().bucketNameCache.Size())
	for i := 0; i < bucketRegionCacheItemCountMax; i++ {
		bucketName := fmt.Sprintf("bucket-%d", i)
		v, ok := getBucketRegionMap().Get(bucketName)
		assert.Equal(t, "", v)
		assert.False(t, ok)
	}
	for i := bucketRegionCacheItemCountMax; i < 2*bucketRegionCacheItemCountMax; i++ {
		bucketName := fmt.Sprintf("bucket-%d", i)
		v, ok := getBucketRegionMap().Get(bucketName)
		assert.Equal(t, "us-east-1", v)
		assert.True(t, ok)
	}

	// Cleanup
	for i := bucketRegionCacheItemCountMax; i < 2*bucketRegionCacheItemCountMax; i++ {
		bucketName := fmt.Sprintf("bucket-%d", i)
		getBucketRegionMap().Remove(bucketName)
	}
}

// Constructor that allows tests to supply a mock Transport
func newS3BucketRegionHeaderCapturingTransportForTest(delegate http.RoundTripper) *s3BucketRegionHeaderCapturingTransport {
	return &s3BucketRegionHeaderCapturingTransport{
		delegate: delegate,
		logger:   logmocks.NewMockLog(),
	}
}

type mockTransportResponse struct {
	resp *http.Response
	err  error
}

// A mock Transport implementation with a map of hard-coded responses
// for a set of URLs.
type mockTransport struct {
	urlToResponseAndError map[string]mockTransportResponse
	requestURLsReceived   []string
}

// Create a new mockTransport with an empty response map
func newMockTransport() *mockTransport {
	return &mockTransport{
		urlToResponseAndError: make(map[string]mockTransportResponse),
		requestURLsReceived:   make([]string, 0),
	}
}

// Register a mock response for the specified url
func (t *mockTransport) AddResponse(url string, response *http.Response) {
	t.urlToResponseAndError[url] = mockTransportResponse{response, nil}
}

// Register a transport error for the specified url
func (t *mockTransport) AddTransportError(url string, err error) {
	t.urlToResponseAndError[url] = mockTransportResponse{nil, err}
}

// Mock RoundTrip implementation.  If the request is for a URL that is in
// the response map, returns the response.  Otherwise, returns a nil response
// and an error.
func (t *mockTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	t.requestURLsReceived = append(t.requestURLsReceived, request.URL.String())
	if response, ok := t.urlToResponseAndError[request.URL.String()]; ok {
		return response.resp, response.err
	}
	return nil, fmt.Errorf("ERROR")
}

func makeRequest(method, rawUrl string) *http.Request {
	parsedUrl, _ := url.Parse(rawUrl)
	return &http.Request{
		Method: method,
		URL:    parsedUrl,
	}
}

func makeResponse(statusCode int, header http.Header, bodyContents string) *http.Response {
	return &http.Response{
		StatusCode:    statusCode,
		Header:        header,
		Body:          ioutil.NopCloser(strings.NewReader(bodyContents)),
		ContentLength: int64(len(bodyContents)),
	}
}

// A credentials.Provider implementation that returns fake credentials
// for testing.
type mockCredentialsProvider struct {
	accessKey string
	secretKey string
}

// Returns fake credentials.
func (c *mockCredentialsProvider) Retrieve() (aws.Credentials, error) {
	return aws.Credentials{
		AccessKeyID:     "FAKEACCESSKEY",
		SecretAccessKey: "FAKESECRETKEY",
		SessionToken:    "FAKESESSIONTOKEN",
		Source:          "mockCredentialsProvider",
	}, nil
}

// Always returns false to indicate the credentials are still valid.
func (c *mockCredentialsProvider) IsExpired() bool {
	return false
}

func makeGetBucketLocationResponseBodyText(region string) string {
	return "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\\n" +
		"<LocationConstraint xmlns=\"http://s3.amazonaws.com/doc/2006-03-01/\">" + region + "</LocationConstraint>"
}

func makeRedirectResponseBodyText(endpoint, bucketName string) string {
	return "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\\n" +
		"<Error><Code>PermanentRedirect</Code>" +
		"<Message>The bucket you are attempting to access must be addressed using the specified endpoint. " +
		"Please send all future requests to this endpoint.</Message>" +
		"<Endpoint>" + endpoint + "</Endpoint>" +
		"<Bucket>" + bucketName + "</Bucket>" +
		"<RequestId>12345</RequestId>" +
		"<HostId>abcde</HostId></Error>"
}

func makeAuthorizationHeaderMalformedErrorResponse(wrongRegion, expRegion string) string {
	return "<?xml version=\"1.0\" encoding=\"UTF-8\"?>" +
		"<Error><Code>AuthorizationHeaderMalformed</Code>" +
		"<Message>The authorization header is malformed; " +
		"the region '" + wrongRegion + "' is wrong; expecting '" + expRegion + "'</Message>" +
		"<Region>" + expRegion + "</Region>" +
		"<RequestId>Request1</RequestId><HostId>Host1</HostId></Error>"
}

func TestExtractRegionFromBody_ErrorXmlWithRegion(t *testing.T) {
	bodyContents := makeAuthorizationHeaderMalformedErrorResponse("us-east-1", "eu-west-1")
	transport := newS3BucketRegionHeaderCapturingTransport(logmocks.NewMockLog(), appconfig.SsmagentConfig{})
	assert.Equal(t, "eu-west-1", transport.extractRegionFromBody([]byte(bodyContents)))
}

func TestExtractRegionFromBody_ErrorXmlWithEndpoint(t *testing.T) {
	bodyContents := makeRedirectResponseBodyText("bucket-1.s3.cn-north-1.amazonaws.com.cn", "cn-north-1")
	transport := newS3BucketRegionHeaderCapturingTransport(logmocks.NewMockLog(), appconfig.SsmagentConfig{})
	assert.Equal(t, "cn-north-1", transport.extractRegionFromBody([]byte(bodyContents)))
}

func TestExtractRegionFromBody_ErrorXmlWithEndpoint_PathStyleEndpointUrl(t *testing.T) {
	bodyContents := makeRedirectResponseBodyText("s3.cn-north-1.amazonaws.com.cn/bucket-1", "cn-north-1")
	transport := newS3BucketRegionHeaderCapturingTransport(logmocks.NewMockLog(), appconfig.SsmagentConfig{})
	assert.Equal(t, "cn-north-1", transport.extractRegionFromBody([]byte(bodyContents)))
}

type mockReaderResponse struct {
	data []byte
	err  error
}
type mockReader struct {
	readResponses     []mockReaderResponse
	readResponseIndex int
}

func (r *mockReader) Read(buf []byte) (int, error) {
	resp := r.readResponses[r.readResponseIndex]
	r.readResponseIndex++

	n := len(resp.data)
	if n > len(buf) {
		n = len(buf)
	}
	for i := 0; i < n; i++ {
		buf[i] = resp.data[i]
	}
	return n, resp.err
}

func (r *mockReader) Close() error {
	return nil
}

func TestGetResponseBody_SingleRead_EOFOnNonemptyRead(t *testing.T) {
	readResponses := []mockReaderResponse{
		{data: []byte("payload"), err: io.EOF},
	}
	getResponseBodyBufsize, getResponseBodyMaxLength = 16, 32
	expResult := []byte("payload")
	expErr := error(nil)
	doGetResponseBodyTest(t, readResponses, expResult, expErr)
}

func TestGetResponseBody_MultipleReads_EOFOnNonemptyRead(t *testing.T) {
	readResponses := []mockReaderResponse{
		{data: []byte("payload"), err: nil},
		{data: []byte("payload"), err: io.EOF},
	}
	getResponseBodyBufsize, getResponseBodyMaxLength = 7, 32
	expResult := []byte("payloadpayload")
	expErr := error(nil)
	doGetResponseBodyTest(t, readResponses, expResult, expErr)
}

func TestGetResponseBody_MultipleReads_EOFOnEmptyRead(t *testing.T) {
	readResponses := []mockReaderResponse{
		{data: []byte("payload"), err: nil},
		{data: []byte("payload"), err: nil},
		{data: []byte(""), err: io.EOF},
	}
	getResponseBodyBufsize, getResponseBodyMaxLength = 7, 32
	expResult := []byte("payloadpayload")
	expErr := error(nil)
	doGetResponseBodyTest(t, readResponses, expResult, expErr)
}

func TestGetResponseBody_MultipleReads_MaxLenExceeded(t *testing.T) {
	readResponses := []mockReaderResponse{
		{data: []byte("payload"), err: nil},
		{data: []byte("payload"), err: nil},
		{data: []byte("payload"), err: io.EOF},
	}
	getResponseBodyBufsize, getResponseBodyMaxLength = 7, 10
	expResult := []byte("payloadpay")
	expErr := fmt.Errorf("getResponseBody(): buffer length exceeded")
	doGetResponseBodyTest(t, readResponses, expResult, expErr)
}

func doGetResponseBodyTest(t *testing.T, mockResponses []mockReaderResponse, expResult []byte, expErr error) {
	body := &mockReader{
		readResponses: mockResponses,
	}
	response := &http.Response{
		Body: body,
	}
	transport := newS3BucketRegionHeaderCapturingTransport(logmocks.NewMockLog(), appconfig.SsmagentConfig{})
	actualBody, actualErr := transport.getResponseBody(response)
	assert.Equal(t, expResult, actualBody)
	assert.Equal(t, expErr, actualErr)
}
