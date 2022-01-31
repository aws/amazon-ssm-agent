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
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil/retryer"
	identityMocks "github.com/aws/amazon-ssm-agent/common/identity/mocks"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client/metadata"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
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

func setS3Endpoint(region, endpoint string) {
	getS3Endpoint = func(context context.T, region string) string {
		return endpoint
	}
}

func setS3FallbackEndpoint(region, endpoint string) {
	getFallbackS3EndpointFunc = func(context context.T, region string) string {
		return endpoint
	}
}

func TestGetBucketRegion_NoError_NoRegionInResponse_ReturnsEmptyString(t *testing.T) {
	setS3Endpoint("us-east-1", "s3.us-east-1.amazonaws.com")
	setS3FallbackEndpoint("us-east-1", "s3.amazonaws.com")
	resp := &http.Response{
		StatusCode: 401,
	}
	var err error = nil
	httpProvider := &MockedHttpProvider{}
	httpProvider.On("Head", "https://bucket-1.s3.us-east-1.amazonaws.com").Return(resp, err)
	httpProvider.On("Head", "https://bucket-1.s3.amazonaws.com").Return(resp, err)
	actual := getBucketRegion(context.NewMockDefault(), "us-east-1", "bucket-1", httpProvider)
	assert.Equal(t, "", actual)
}

func TestGetBucketRegion_NoError_RegionInResponse_ReturnsRegion(t *testing.T) {
	setS3Endpoint("us-east-1", "s3.us-east-1.amazonaws.com")
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
	actual := getBucketRegion(context.NewMockDefault(), "us-east-1", "bucket-1", httpProvider)
	assert.Equal(t, "eu-west-1", actual)
}

func TestGetBucketRegion_AllUrlsFail_ReturnsEmptyString(t *testing.T) {
	setS3Endpoint("us-east-1", "s3.us-east-1.amazonaws.com")
	setS3FallbackEndpoint("us-east-1", "s3.amazonaws.com")
	var resp *http.Response = nil
	err := fmt.Errorf("failed")
	httpProvider := &MockedHttpProvider{}
	httpProvider.On("Head", "https://bucket-1.s3.us-east-1.amazonaws.com").Return(resp, err)
	httpProvider.On("Head", "https://bucket-1.s3.amazonaws.com").Return(resp, err)
	httpProvider.On("Head", "http://bucket-1.s3.us-east-1.amazonaws.com").Return(resp, err)
	httpProvider.On("Head", "http://bucket-1.s3.amazonaws.com").Return(resp, err)
	actual := getBucketRegion(context.NewMockDefault(), "us-east-1", "bucket-1", httpProvider)
	assert.Equal(t, "", actual)
	httpProvider.AssertExpectations(t)
}

func TestGetS3CrossRegionCapableSession_regionFromHead_noConfigOverrides(t *testing.T) {
	setupMocksForGetS3CrossRegionCapableSession("us-east-1", "bucket-1", "eu-west-1")
	sess, err := GetS3CrossRegionCapableSession(context.NewMockDefault(), "bucket-1")
	assert.NotNil(t, sess)
	assert.Equal(t, *sess.Config.Region, "eu-west-1")
	assert.Nil(t, sess.Config.Endpoint)
	assert.NotNil(t, sess.Config.HTTPClient.Transport)
	_, correctType := sess.Config.HTTPClient.Transport.(*s3BucketRegionHeaderCapturingTransport)
	assert.True(t, correctType)
	assert.Nil(t, err)
}

func TestGetS3CrossRegionCapableSession_noRegionFromHead_noConfigOverrides(t *testing.T) {
	identityMock := &identityMocks.IAgentIdentity{}
	identityMock.On("Region").Return("cn-north-1", nil)

	contextMock := new(context.Mock)
	contextMock.On("Identity").Return(identityMock)
	contextMock.On("Log").Return(log.NewMockLog())
	contextMock.On("AppConfig").Return(appconfig.DefaultConfig())

	setupMocksForGetS3CrossRegionCapableSession("cn-north-1", "bucket-1", "")
	sess, err := GetS3CrossRegionCapableSession(contextMock, "bucket-1")
	assert.NotNil(t, sess)
	assert.Equal(t, "cn-north-1", *sess.Config.Region)
	assert.Nil(t, sess.Config.Endpoint)
	assert.NotNil(t, sess.Config.HTTPClient.Transport)
	_, correctType := sess.Config.HTTPClient.Transport.(*s3BucketRegionHeaderCapturingTransport)
	assert.True(t, correctType)
	assert.Nil(t, err)
}

func TestGetS3CrossRegionCapableSession_regionFromHead_withConfigOverrides(t *testing.T) {
	appConfig := appconfig.DefaultConfig()
	appConfig.S3.Endpoint = "https://custom.endpoint.com"

	identityMock := &identityMocks.IAgentIdentity{}
	identityMock.On("Region").Return("us-east-1", nil)

	contextMock := new(context.Mock)
	contextMock.On("Identity").Return(identityMock)
	contextMock.On("Log").Return(log.NewMockLog())
	contextMock.On("AppConfig").Return(appConfig)

	setupMocksForGetS3CrossRegionCapableSession("us-east-1", "bucket-1", "eu-west-1")
	sess, err := GetS3CrossRegionCapableSession(contextMock, "bucket-1")
	assert.NotNil(t, sess)
	assert.Equal(t, "eu-west-1", *sess.Config.Region)
	assert.Equal(t, "https://custom.endpoint.com", *sess.Config.Endpoint)
	assert.NotNil(t, sess.Config.HTTPClient.Transport)
	_, correctType := sess.Config.HTTPClient.Transport.(*s3BucketRegionHeaderCapturingTransport)
	assert.True(t, correctType)
	assert.Nil(t, err)
}

func TestGetS3CrossRegionCapableSession_noRegionFromHead_withConfigOverrides(t *testing.T) {
	appConfig := appconfig.DefaultConfig()
	appConfig.S3.Endpoint = "https://custom.endpoint.com.cn"

	identityMock := &identityMocks.IAgentIdentity{}
	identityMock.On("Region").Return("cn-north-1", nil)

	contextMock := new(context.Mock)
	contextMock.On("Identity").Return(identityMock)
	contextMock.On("Log").Return(log.NewMockLog())
	contextMock.On("AppConfig").Return(appConfig)

	setupMocksForGetS3CrossRegionCapableSession("cn-north-1", "bucket-1", "")
	sess, err := GetS3CrossRegionCapableSession(contextMock, "bucket-1")
	assert.NotNil(t, sess)
	assert.Equal(t, "cn-north-1", *sess.Config.Region)
	assert.Equal(t, "https://custom.endpoint.com.cn", *sess.Config.Endpoint)
	assert.NotNil(t, sess.Config.HTTPClient.Transport)
	_, correctType := sess.Config.HTTPClient.Transport.(*s3BucketRegionHeaderCapturingTransport)
	assert.True(t, correctType)
	assert.Nil(t, err)
}

func setupMocksForGetS3CrossRegionCapableSession(instanceRegion, bucketName, headBucketResponse string) {
	setupMockHeadBucketResponse(bucketName, instanceRegion, headBucketResponse)
	makeAwsConfig = func(context context.T, service, region string) *aws.Config {
		result := aws.NewConfig()
		result.Region = aws.String(region)
		result.Credentials = credentials.NewCredentials(&mockCredentialsProvider{})
		return result
	}
}

func setupMockHeadBucketResponse(bucketName, instanceRegion, headBucketResponse string) {
	s3Endpoint := "s3." + instanceRegion + ".amazonaws.com"
	s3FallbackEndpoint := "s3.amazonaws.com"
	if strings.HasPrefix(instanceRegion, "cn-") {
		s3Endpoint += ".cn"
		s3FallbackEndpoint = "s3.cn-north-1.amazonaws.com.cn"
	}
	setS3Endpoint(instanceRegion, s3Endpoint)
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

func TestRedirect_RedirectResponse_RetryWithCorrectRegion(t *testing.T) {
	appConfig := appconfig.DefaultConfig()
	identityMock := &identityMocks.IAgentIdentity{}
	identityMock.On("Region").Return("cn-northwest-1", nil)

	contextMock := new(context.Mock)
	contextMock.On("Identity").Return(identityMock)
	contextMock.On("Log").Return(log.NewMockLog())
	contextMock.On("AppConfig").Return(appConfig)

	setupMocksForGetS3CrossRegionCapableSession("cn-northwest-1", "bucket-1", "")
	sess, err := GetS3CrossRegionCapableSession(contextMock, "bucket-1")
	assert.Nil(t, err)

	trans, transTypeOk := sess.Config.HTTPClient.Transport.(*s3BucketRegionHeaderCapturingTransport)
	assert.True(t, transTypeOk)

	delegate := newMockTransport()
	trans.delegate = delegate

	svc := s3.New(sess)
	input := &s3.HeadBucketInput{
		Bucket: aws.String("bucket-1"),
	}

	// First attempt goes to the instance's home region.  S3 returns a 301 PermanentRedirect
	// response with header indicating the correct region for the bucket.
	req1Url := "https://bucket-1.s3.cn-northwest-1.amazonaws.com.cn/"
	resp1Header := http.Header{}
	resp1Header.Add(bucketRegionHeader, "cn-north-1")
	resp1 := &http.Response{
		Status:     "PermanentRedirect",
		StatusCode: 301,
		Header:     resp1Header,
		Body:       ioutil.NopCloser(strings.NewReader("body contents")),
	}

	// The retry goes to the correct endpoint for the bucket, which is cn-north-1.
	req2Url := "https://bucket-1.s3.cn-north-1.amazonaws.com.cn/"
	resp2Header := http.Header{}
	resp2 := &http.Response{
		Status:     "Success",
		StatusCode: 200,
		Header:     resp2Header,
		Body:       ioutil.NopCloser(strings.NewReader("")),
	}
	delegate.AddResponse(req1Url, resp1)
	delegate.AddResponse(req2Url, resp2)

	_, err = svc.HeadBucket(input)

	assert.Nil(t, err)
	assert.Equal(t, 2, len(delegate.requestURLsReceived))
	assert.Equal(t, "https://bucket-1.s3.cn-northwest-1.amazonaws.com.cn/", delegate.requestURLsReceived[0])
	assert.Equal(t, "https://bucket-1.s3.cn-north-1.amazonaws.com.cn/", delegate.requestURLsReceived[1])

	// Cleanup
	getBucketRegionMap().Remove("bucket-1")
}

func TestRedirect_BadSigningRegionResponse_RetryWithCorrectRegion(t *testing.T) {
	setupMocksForGetS3CrossRegionCapableSession("us-east-1", "bucket-1", "")
	sess, err := GetS3CrossRegionCapableSession(context.NewMockDefault(), "bucket-1")
	assert.Nil(t, err)

	trans, transTypeOk := sess.Config.HTTPClient.Transport.(*s3BucketRegionHeaderCapturingTransport)
	assert.True(t, transTypeOk)

	delegate := newMockTransport()
	trans.delegate = delegate

	svc := s3.New(sess)
	input := &s3.HeadBucketInput{
		Bucket: aws.String("bucket-1"),
	}

	// For the first attempt, the client is initialized for us-east-1.
	// However, DNS is able to resolve the virtual hosted bucket URL
	// to the correct regional endpoint in eu-west-1.  The eu-west-1
	// endpoint returns an HTTP 400 "wrong signing region" error, with
	// the bucket region set in the response body.
	req1Url := "https://bucket-1.s3.amazonaws.com/"
	resp1Header := http.Header{}
	resp1Body := makeAuthorizationHeaderMalformedErrorResponse("us-east-1", "eu-west-1")
	resp1 := &http.Response{
		Status:     "",
		StatusCode: 400,
		Header:     resp1Header,
		Body:       ioutil.NopCloser(strings.NewReader(resp1Body)),
	}

	// The retry should have the correct regional endpoint in the request URL
	req2Url := "https://bucket-1.s3.eu-west-1.amazonaws.com/"
	resp2Header := http.Header{}
	resp2 := &http.Response{
		Status:     "Success",
		StatusCode: 200,
		Header:     resp2Header,
		Body:       ioutil.NopCloser(strings.NewReader("")),
	}
	delegate.AddResponse(req1Url, resp1)
	delegate.AddResponse(req2Url, resp2)

	_, err = svc.HeadBucket(input)

	assert.Nil(t, err)
	assert.Equal(t, 2, len(delegate.requestURLsReceived))
	assert.Equal(t, "https://bucket-1.s3.amazonaws.com/", delegate.requestURLsReceived[0])
	assert.Equal(t, "https://bucket-1.s3.eu-west-1.amazonaws.com/", delegate.requestURLsReceived[1])

	// Cleanup
	getBucketRegionMap().Remove("bucket-1")
}

func TestRedirect_CachedBucketRegion_FirstRequestGoesToCorrectRegion(t *testing.T) {
	// The correct region for the bucket is already cached.  The first
	// attempt should go to the correct region.
	getBucketRegionMap().Put("bucket-1", "cn-north-1")

	setupMocksForGetS3CrossRegionCapableSession("cn-northwest-1", "bucket-1", "")
	sess, err := GetS3CrossRegionCapableSession(context.NewMockDefault(), "bucket-1")
	assert.Nil(t, err)

	trans, transTypeOk := sess.Config.HTTPClient.Transport.(*s3BucketRegionHeaderCapturingTransport)
	assert.True(t, transTypeOk)

	delegate := newMockTransport()
	trans.delegate = delegate

	svc := s3.New(sess)
	input := &s3.GetBucketLocationInput{
		Bucket: aws.String("bucket-1"),
	}

	// The first attempt goes to the correct endpoint for the bucket, which is cn-north-1.
	reqUrl := "https://s3.cn-north-1.amazonaws.com.cn/bucket-1?location="
	respHeader := http.Header{}
	resp := &http.Response{
		Status:     "Success",
		StatusCode: 200,
		Header:     respHeader,
		Body:       ioutil.NopCloser(strings.NewReader(makeGetBucketLocationResponseBodyText("cn-north-1"))),
	}
	delegate.AddResponse(reqUrl, resp)

	output, err := svc.GetBucketLocation(input)

	assert.Nil(t, err)
	assert.Equal(t, "cn-north-1", *output.LocationConstraint)
	assert.Equal(t, 1, len(delegate.requestURLsReceived))
	assert.Equal(t, "https://s3.cn-north-1.amazonaws.com.cn/bucket-1?location=", delegate.requestURLsReceived[0])

	// Cleanup
	getBucketRegionMap().Remove("bucket-1")
}

type handlerTestCaseData struct {
	bucketName string
	op         *request.Operation
	input      interface{}
	output     interface{}
}

var handlerTestCases = []handlerTestCaseData{
	{
		bucketName: "bucket-1",
		op: &request.Operation{
			Name:       "PutObject",
			HTTPMethod: "PUT",
			HTTPPath:   "/{Bucket}/{Key+}",
		},
		input: &s3.PutObjectInput{
			Body:   strings.NewReader("body contents"),
			Key:    aws.String("a/b"),
			Bucket: aws.String("bucket-1"),
		},
		output: &s3.PutObjectOutput{},
	},
	{
		bucketName: "bucket-1",
		op: &request.Operation{
			Name:       "CreateMultipartUpload",
			HTTPMethod: "POST",
			HTTPPath:   "/{Bucket}/{Key+}?uploads",
		},
		input: &s3.CreateMultipartUploadInput{
			Bucket:      aws.String("bucket-1"),
			Key:         aws.String("a/b"),
			ContentType: aws.String("text/plain"),
			ACL:         aws.String("bucket-owner-full-control"),
		},
		output: &s3.CreateMultipartUploadOutput{},
	},
	{
		bucketName: "bucket-1",
		op: &request.Operation{
			Name:       "UploadPart",
			HTTPMethod: "PUT",
			HTTPPath:   "/{Bucket}/{Key+}",
		},
		input: &s3.UploadPartInput{
			Bucket:     aws.String("bucket-1"),
			Key:        aws.String("a/b"),
			Body:       strings.NewReader("body contents"),
			UploadId:   aws.String("1324"),
			PartNumber: aws.Int64(1),
		},
		output: &s3.UploadPartOutput{},
	},
}

func TestHandlerAllCases(t *testing.T) {
	for _, d := range handlerTestCases {
		validationHandlerTestCase(t, d.bucketName, "cn-northwest-1", "cn-north-1", d.op, d.input, d.output)
		validationHandlerTestCase(t, d.bucketName, "us-east-1", "us-west-1", d.op, d.input, d.output)
		validationHandlerTestCase(t, d.bucketName, "us-gov-east-1", "us-gov-west-1", d.op, d.input, d.output)

		retryHandlerTestCase(t, d.bucketName, "cn-northwest-1", "cn-north-1", d.op, d.input, d.output)
		retryHandlerTestCase(t, d.bucketName, "us-east-1", "us-west-1", d.op, d.input, d.output)
		retryHandlerTestCase(t, d.bucketName, "us-gov-east-1", "us-gov-west-1", d.op, d.input, d.output)
	}
}

func validationHandlerTestCase(t *testing.T, bucketName, oldRegion, newRegion string,
	op *request.Operation, input, output interface{}) {

	// The request initially targets the old region
	retryer := retryer.SsmRetryer{}
	retryer.NumMaxRetries = 3
	config := &aws.Config{
		Retryer: retryer,
		SleepDelay: func(d time.Duration) {
			time.Sleep(d)
		},
		Region: &oldRegion,
	}

	// The correct region for the bucket has been discovered by s3BucketRegionHeaderCapturingTransport
	getBucketRegionMap().Put(bucketName, newRegion)

	sess := session.New(config)
	sess.Handlers.Validate.PushBackNamed(makeS3RegionCorrectingValidateHandler(log.NewMockLog()))
	svc := s3.New(sess)

	request := svc.NewRequest(op, input, output)
	assert.Equal(t, oldRegion, *request.Config.Region)

	request.Build()

	var newRegionS3EndpointHostname string
	if strings.HasPrefix(newRegion, "cn-") {
		newRegionS3EndpointHostname = "s3." + newRegion + ".amazonaws.com.cn"
	} else {
		newRegionS3EndpointHostname = "s3." + newRegion + ".amazonaws.com"
	}

	assert.Equal(t, newRegion, *request.Config.Region)
	assert.Equal(t, "https://"+newRegionS3EndpointHostname, request.ClientInfo.Endpoint)
	assert.Equal(t, newRegion, request.ClientInfo.SigningRegion)
	assert.Equal(t, bucketName+"."+newRegionS3EndpointHostname, request.HTTPRequest.URL.Host)

	// Cleanup
	getBucketRegionMap().Remove(bucketName)
}

func retryHandlerTestCase(t *testing.T, bucketName, oldRegion, newRegion string,
	op *request.Operation, input, output interface{}) {

	// The request initially targets the old region
	retryer := retryer.SsmRetryer{}
	retryer.NumMaxRetries = 3
	config := &aws.Config{
		Retryer: retryer,
		SleepDelay: func(d time.Duration) {
			time.Sleep(d)
		},
		Region: &oldRegion,
	}

	sess := session.New(config)
	sess.Handlers.Retry.PushFrontNamed(makeS3RegionCorrectingRetryHandler(log.NewMockLog()))
	svc := s3.New(sess)

	request := svc.NewRequest(op, input, output)
	assert.Equal(t, oldRegion, *request.Config.Region)

	request.Build()

	// Simulate sending the request.  S3 returns a 301, and the Transport
	// captures the bucket region from the response headers.
	getBucketRegionMap().Put(bucketName, newRegion)
	request.HTTPResponse = &http.Response{
		StatusCode: 301,
	}

	// Invoke the handler
	request.Handlers.Retry.Run(request)

	var newRegionS3EndpointHostname string
	if strings.HasPrefix(newRegion, "cn-") {
		newRegionS3EndpointHostname = "s3." + newRegion + ".amazonaws.com.cn"
	} else {
		newRegionS3EndpointHostname = "s3." + newRegion + ".amazonaws.com"
	}

	assert.Equal(t, newRegion, *request.Config.Region)
	assert.Equal(t, "https://"+newRegionS3EndpointHostname, request.ClientInfo.Endpoint)
	assert.Equal(t, newRegion, request.ClientInfo.SigningRegion)
	assert.Equal(t, bucketName+"."+newRegionS3EndpointHostname, request.HTTPRequest.URL.Host)

	// Cleanup
	getBucketRegionMap().Remove(bucketName)
}

func TestValidateHandler_EndpointLookupFailure_NoChangeToRequest(t *testing.T) {
	// bucket-1 somehow got mapped to an unknown region
	getBucketRegionMap().Put("bucket-1", "unknown-region-1")
	config := &aws.Config{
		Region: aws.String("us-east-1"),

		// This simulates an endpoint lookup failure
		EndpointResolver: mockEndpointResolver{
			endpoints.ResolvedEndpoint{},
			fmt.Errorf("ERROR"),
		},
	}

	op := &request.Operation{
		Name:       "PutObject",
		HTTPMethod: "PUT",
		HTTPPath:   "/{Bucket}/{Key+}",
	}
	input := &s3.PutObjectInput{
		Body:   strings.NewReader("body contents"),
		Key:    aws.String("a/b"),
		Bucket: aws.String("bucket-1"),
	}
	output := &s3.PutObjectOutput{}

	sess := session.New(config)
	svc := s3.New(sess)
	request := svc.NewRequest(op, input, output)

	handler := makeS3RegionCorrectingValidateHandler(log.NewMockLog())
	handler.Fn(request)

	assert.Equal(t, "us-east-1", *request.Config.Region)

	// Cleanup
	getBucketRegionMap().Remove("bucket-1")
}

func TestRetryHandler_EndpointLookupFailure_NoChangeToRequest(t *testing.T) {
	// bucket-1 somehow got mapped to an unknown region
	getBucketRegionMap().Put("bucket-1", "unknown-region-1")
	config := &aws.Config{
		Region: aws.String("us-east-1"),

		// This simulates an endpoint lookup failure
		EndpointResolver: mockEndpointResolver{
			endpoints.ResolvedEndpoint{},
			fmt.Errorf("ERROR"),
		},
	}

	op := &request.Operation{
		Name:       "PutObject",
		HTTPMethod: "PUT",
		HTTPPath:   "/{Bucket}/{Key+}",
	}
	input := &s3.PutObjectInput{
		Body:   strings.NewReader("body contents"),
		Key:    aws.String("a/b"),
		Bucket: aws.String("bucket-1"),
	}
	output := &s3.PutObjectOutput{}

	sess := session.New(config)
	svc := s3.New(sess)
	request := svc.NewRequest(op, input, output)
	request.HTTPResponse = &http.Response{
		StatusCode: 301,
	}

	handler := makeS3RegionCorrectingRetryHandler(log.NewMockLog())
	handler.Fn(request)

	assert.Equal(t, "us-east-1", *request.Config.Region)

	// Cleanup
	getBucketRegionMap().Remove("bucket-1")
}

func TestFixupRequest_NoHttpRequestUrl_NoCustomEndpoint_SetsRegionAndEndpoint(t *testing.T) {
	request := &request.Request{
		Config: aws.Config{
			Region: aws.String("us-east-1"),
		},
		ClientInfo:  metadata.ClientInfo{},
		HTTPRequest: &http.Request{},
	}
	fixupRequest(log.NewMockLog(), request, "eu-west-1")
	assert.Equal(t, "eu-west-1", *request.Config.Region)
	assert.Nil(t, request.Config.Endpoint)
	assert.Equal(t, "https://s3.eu-west-1.amazonaws.com", request.ClientInfo.Endpoint)
}

func TestFixupRequest_NoHttpRequestUrl_CustomEndpoint_SetsRegionAndEndpoint(t *testing.T) {
	request := &request.Request{
		Config: aws.Config{
			Region:   aws.String("us-east-1"),
			Endpoint: aws.String("https://my-custom-endpoint.com"),
		},
		ClientInfo:  metadata.ClientInfo{},
		HTTPRequest: &http.Request{},
	}
	fixupRequest(log.NewMockLog(), request, "eu-west-1")
	assert.Equal(t, "eu-west-1", *request.Config.Region)
	assert.Equal(t, "https://my-custom-endpoint.com", *request.Config.Endpoint)
	assert.Equal(t, "https://my-custom-endpoint.com", request.ClientInfo.Endpoint)
}

func TestFixupRequest_HttpRequestUrl_NoCustomEndpoint_SetsRegionAndHttpRequestUrl(t *testing.T) {
	request := &request.Request{
		Config: aws.Config{
			Region:           aws.String("us-east-1"),
			EndpointResolver: endpoints.DefaultResolver(),
		},
		ClientInfo: metadata.ClientInfo{
			Endpoint: "http://s3.amazonaws.com",
		},
		HTTPRequest: &http.Request{
			URL: &url.URL{
				Scheme: "https",
				Host:   "s3.amazonaws.com",
			},
		},
	}
	fixupRequest(log.NewMockLog(), request, "eu-west-1")
	assert.Equal(t, "eu-west-1", *request.Config.Region)
	assert.Equal(t, "https://s3.eu-west-1.amazonaws.com", request.HTTPRequest.URL.String())
	assert.Nil(t, request.Config.Endpoint)
	assert.Equal(t, "https://s3.eu-west-1.amazonaws.com", request.ClientInfo.Endpoint)
}

func TestFixupRequest_HttpRequestUrlPresent_RespectsCustomEndpoint(t *testing.T) {
	request := &request.Request{
		Config: aws.Config{
			Region:           aws.String("us-east-1"),
			Endpoint:         aws.String("https://my-custom-endpoint.com"),
			EndpointResolver: endpoints.DefaultResolver(),
		},
		ClientInfo: metadata.ClientInfo{
			Endpoint: "https://my-custom-endpoint.com",
		},
		HTTPRequest: &http.Request{
			URL: &url.URL{
				Scheme: "https",
				Host:   "s3.amazonaws.com",
			},
		},
	}
	fixupRequest(log.NewMockLog(), request, "eu-west-1")
	assert.Equal(t, "eu-west-1", *request.Config.Region)
	assert.Equal(t, "https://my-custom-endpoint.com", request.HTTPRequest.URL.String())
	assert.Equal(t, "https://my-custom-endpoint.com", *request.Config.Endpoint)
	assert.Equal(t, "https://my-custom-endpoint.com", request.ClientInfo.Endpoint)
}

func TestFixupRequest_HttpRequestUrlPresent_VirtualHostedUrlWithKey(t *testing.T) {
	request := &request.Request{
		Config: aws.Config{
			Region:           aws.String("us-east-1"),
			EndpointResolver: endpoints.DefaultResolver(),
		},
		ClientInfo: metadata.ClientInfo{
			Endpoint: "https://s3.amazonaws.com",
		},
		HTTPRequest: &http.Request{
			URL: &url.URL{
				Scheme: "https",
				Host:   "bucket-1.s3.amazonaws.com",
				Path:   "/key",
			},
		},
	}
	fixupRequest(log.NewMockLog(), request, "eu-west-1")
	assert.Equal(t, "eu-west-1", *request.Config.Region)
	assert.Equal(t, "https://bucket-1.s3.eu-west-1.amazonaws.com/key", request.HTTPRequest.URL.String())
	assert.Nil(t, request.Config.Endpoint)
	assert.Equal(t, "https://s3.eu-west-1.amazonaws.com", request.ClientInfo.Endpoint)
}

func TestFixupRequest_HttpRequestUrlPresent_PathStyleUrlWithKey(t *testing.T) {
	request := &request.Request{
		Config: aws.Config{
			Region:           aws.String("us-east-1"),
			EndpointResolver: endpoints.DefaultResolver(),
		},
		ClientInfo: metadata.ClientInfo{
			Endpoint: "https://s3.amazonaws.com",
		},
		HTTPRequest: &http.Request{
			URL: &url.URL{
				Scheme: "https",
				Host:   "s3.amazonaws.com",
				Path:   "/bucket-1/key",
			},
		},
	}
	fixupRequest(log.NewMockLog(), request, "eu-west-1")
	assert.Equal(t, "eu-west-1", *request.Config.Region)
	assert.Equal(t, "https://s3.eu-west-1.amazonaws.com/bucket-1/key", request.HTTPRequest.URL.String())
	assert.Nil(t, request.Config.Endpoint)
	assert.Equal(t, "https://s3.eu-west-1.amazonaws.com", request.ClientInfo.Endpoint)
}

func TestNewS3BucketRegionHeaderCapturingTransport(t *testing.T) {
	transport := newS3BucketRegionHeaderCapturingTransport(log.NewMockLog(), appconfig.SsmagentConfig{})
	_, goodType := transport.delegate.(*http.Transport)
	assert.True(t, goodType)
}

func TestRoundTrip_bucketRegionHeaderPresent(t *testing.T) {
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
		logger:   log.NewMockLog(),
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
func (c *mockCredentialsProvider) Retrieve() (credentials.Value, error) {
	return credentials.Value{
		AccessKeyID:     "FAKEACCESSKEY",
		SecretAccessKey: "FAKESECRETKEY",
		SessionToken:    "FAKESESSIONTOKEN",
		ProviderName:    "mockCredentialsProvider",
	}, nil
}

// Always returns false to indicate the credentials are still valid.
func (c *mockCredentialsProvider) IsExpired() bool {
	return false
}

// A Resolver implementation that returns a hard-coded endpoint
type mockEndpointResolver struct {
	resolvedEndpoint endpoints.ResolvedEndpoint
	err              error
}

// Returns the hard-coded endpoint lookup response
func (r mockEndpointResolver) EndpointFor(service, region string, opts ...func(*endpoints.Options)) (endpoints.ResolvedEndpoint, error) {
	return r.resolvedEndpoint, r.err
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
	transport := newS3BucketRegionHeaderCapturingTransport(log.NewMockLog(), appconfig.SsmagentConfig{})
	assert.Equal(t, "eu-west-1", transport.extractRegionFromBody([]byte(bodyContents)))
}

func TestExtractRegionFromBody_ErrorXmlWithEndpoint(t *testing.T) {
	bodyContents := makeRedirectResponseBodyText("bucket-1.s3.cn-north-1.amazonaws.com.cn", "cn-north-1")
	transport := newS3BucketRegionHeaderCapturingTransport(log.NewMockLog(), appconfig.SsmagentConfig{})
	assert.Equal(t, "cn-north-1", transport.extractRegionFromBody([]byte(bodyContents)))
}

func TestExtractRegionFromBody_ErrorXmlWithEndpoint_PathStyleEndpointUrl(t *testing.T) {
	bodyContents := makeRedirectResponseBodyText("s3.cn-north-1.amazonaws.com.cn/bucket-1", "cn-north-1")
	transport := newS3BucketRegionHeaderCapturingTransport(log.NewMockLog(), appconfig.SsmagentConfig{})
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
	transport := newS3BucketRegionHeaderCapturingTransport(log.NewMockLog(), appconfig.SsmagentConfig{})
	actualBody, actualErr := transport.getResponseBody(response)
	assert.Equal(t, expResult, actualBody)
	assert.Equal(t, expErr, actualErr)
}
