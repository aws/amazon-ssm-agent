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
	"bytes"
	cont "context"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	"github.com/Workiva/go-datastructures/cache"
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/network"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/amazon-ssm-agent/common/identity/endpoint"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	bucketRegionHeader            = "X-Amz-Bucket-Region"
	retryOnRedirectResponseCode   = 500
	bucketRegionCacheItemCountMax = 128
)

// Custom middleware to intercept HTTP response and extract bucket region
type bucketRegionExtractor struct {
	bucketRegion *string
}

func (b *bucketRegionExtractor) ID() string {
	return "BucketRegionExtractor"
}

func (b *bucketRegionExtractor) HandleDeserialize(
	ctx cont.Context,
	in middleware.DeserializeInput,
	next middleware.DeserializeHandler,
) (middleware.DeserializeOutput, middleware.Metadata, error) {

	// Call the next handler first to get the HTTP response
	out, metadata, err := next.HandleDeserialize(ctx, in)

	// Extract HTTP response from the transport layer
	if httpResp, ok := out.RawResponse.(*smithyhttp.Response); ok {
		if region := httpResp.Header.Get(bucketRegionHeader); region != "" {
			*b.bucketRegion = region
		}

		// Only suppress errors for expected redirect/access-denied responses when region was extracted
		// These are expected when doing cross-region HeadBucket requests
		if *b.bucketRegion != "" && err != nil {
			statusCode := httpResp.StatusCode
			// Suppress only redirect (301, 307) and access-related (400, 403) errors
			// that are expected when probing bucket region
			if statusCode == 301 || statusCode == 307 || statusCode == 400 || statusCode == 403 {
				err = nil
			}
		}
	}

	return out, metadata, err
}

// Returns a Session capable of performing cross-region S3 bucket accesses
// (i.e. the bucket region may be different from the instance's home region).
// The session is initialized to work with the specified bucket, and should
// not be used to access other buckets.
//
// When initializing the session, we make a best-effort attempt to determine
// the region in which the bucket resides.  The session is initialized with
// the correct region for the bucket if the region was successfully determined,
// or with the instance region.
//
// The session also has a Handler chain and custom HTTP RoundTripper that follow
// cross-region redirect responses from S3.  These work as follows:
//  1. The custom RoundTripper (s3BucketRegionHeaderCapturingTransport) extracts
//     the bucket region information from S3 redirect responses and stores them
//     in a cache.
//  2. The Retry Handler, which is invoked before each retry, checks to see whether
//     a bucket -> region mapping exists for the request's bucket, and if so, fixes
//     up the request to point to the correct region.
//  3. The Validation Handler, which is invoked before the first attempt, similarly
//     checks for a bucket -> region mapping for the request's bucket, and if one
//     is found, fixes up the request to point to the correct region.
//
// In most cases, the best-effort attempt will initialize the session with the correct
// region, and the custom Transport and Handler chain will not need to make any changes.
func GetS3CrossRegionCapableSession(context context.T, bucketName string) (*aws.Config, error) {
	log := context.Log()

	initialRegion, err := context.Identity().Region()
	if err != nil {
		log.Errorf("failed to get instance region: %v", err)
		return nil, err
	}

	guessedBucketRegion := getBucketRegion(context, initialRegion, bucketName, getHttpProvider(log, context.AppConfig()))
	if guessedBucketRegion != "" {
		initialRegion = guessedBucketRegion
	} else {
		log.Infof("using instance region %v for bucket %v", initialRegion, bucketName)
	}

	config := makeAwsConfig(context, "s3", initialRegion)

	appConfig := context.AppConfig()

	var agentName, agentVersion string
	agentName = appConfig.Agent.Name
	agentVersion = appConfig.Agent.Version

	if appConfig.S3.Endpoint != "" {
		ep := sdkutil.NormalizeEndpointScheme(appConfig.S3.Endpoint)
		config.BaseEndpoint = &ep
	}

	config.HTTPClient = &http.Client{
		Transport: newS3BucketRegionHeaderCapturingTransport(log, context.AppConfig()),
	}

	// User Agent handler
	config.APIOptions = append(config.APIOptions, awsmiddleware.AddUserAgentKeyValue(agentName, agentVersion))

	// S3 cross-region correction middleware: on each retry attempt, check if
	// the bucket region cache has a corrected region from a previous redirect
	// response, and rewrite the request URL to the correct regional endpoint.
	// This must be in Finalize (after "Retry") so it runs on every attempt.
	config.APIOptions = append(config.APIOptions, func(stack *middleware.Stack) error {
		return stack.Finalize.Insert(
			middleware.FinalizeMiddlewareFunc("S3RegionCorrecting",
				func(ctx cont.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
					req, ok := in.Request.(*smithyhttp.Request)
					if !ok {
						return next.HandleFinalize(ctx, in)
					}

					bucket := getBucketFromRequest(log, req)
					if bucket == "" {
						return next.HandleFinalize(ctx, in)
					}

					if cachedRegion, found := getBucketRegionMap().Get(bucket); found {
						endpointHelper := endpoint.NewEndpointHelper(log, appConfig)
						correctEndpoint := endpointHelper.GetServiceEndpoint("s3", cachedRegion)
						if correctEndpoint != "" {
							log.Infof("Correcting S3 request for bucket %s to region %s", bucket, cachedRegion)
							fixupRequestUrl(log, req, correctEndpoint)
						}
					}

					return next.HandleFinalize(ctx, in)
				}),
			"Retry",
			middleware.After,
		)
	})

	return &config, nil
}

// Tries to determine the correct region for the specified bucket by doing
// an anonymous HTTP HEAD request for the bucket URL and checking for the
// x-amz-bucket-region header in the response.  If the region cannot be
// determined in this way, returns "".
//
// In some cases, but not all cases, the S3 endpoint response to the HEAD
// request will contain the x-amz-bucket-region header indicating the correct
// region for the bucket.  S3 endpoints in the "aws" partition generally include
// this header in the response, so this method works well for those regions.
// S3 endpoints in the "aws-cn" partition may return a 401 or 403 response without
// the header.

func getBucketRegionFromSignedHeadBucketRequest(context context.T, instanceRegion, regionalEndpoint, bucketName string) (region string) {
	var bucketRegion = ""
	log := context.Log()

	credentials := context.Identity().CredentialsProvider()
	ctx := cont.Background()

	// Normalize the endpoint scheme — regionalEndpoint may come from the raw
	// appconfig override (S3.Endpoint), which can be scheme-less. SDK v2's
	// URI validator requires a scheme on BaseEndpoint, so add https:// when
	// missing.
	normalizedEndpoint := sdkutil.NormalizeEndpointScheme(regionalEndpoint)

	config := aws.Config{
		Credentials: credentials,
		Region:      instanceRegion,
	}
	// Only pin BaseEndpoint when the helper actually produced one. Leaving
	// it nil lets the SDK ruleset resolver produce the partition-appropriate
	// default; setting it to aws.String("") would short-circuit the resolver
	// and produce an empty request URI.
	if normalizedEndpoint != "" {
		config.BaseEndpoint = aws.String(normalizedEndpoint)
	}

	client := s3.NewFromConfig(config, func(o *s3.Options) {
		o.HTTPClient = &http.Client{
			// Use the agent's default transport so proxy, TLS, and CA settings
			// are honored instead of falling back to http.DefaultTransport.
			Transport: network.GetDefaultTransport(log, context.AppConfig()),
			Timeout:   10 * time.Second,
			// Disable following redirects — we want the 301/307 response to extract
			// the x-amz-bucket-region header, not follow it and amplify requests.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		o.RetryMaxAttempts = 1

		// Add our custom middleware to extract bucket region
		o.APIOptions = append(o.APIOptions, func(stack *middleware.Stack) error {
			return stack.Deserialize.Add(&bucketRegionExtractor{
				bucketRegion: &bucketRegion,
			}, middleware.After)
		})
	})

	headBucketInput := &s3.HeadBucketInput{
		Bucket: aws.String(bucketName),
	}

	_, err := client.HeadBucket(ctx, headBucketInput)
	if err != nil {
		log.Warnf("Signed HeadBucket request failed, continuing to fallback logic")
	}

	return bucketRegion
}

func getBucketFromRequest(log log.T, req *smithyhttp.Request) string {
	if req == nil || req.URL == nil {
		return ""
	}

	parseOutput := ParseAmazonS3URL(log, req.URL)
	if parseOutput.IsValidS3URI && parseOutput.Bucket != "" {
		return parseOutput.Bucket
	}

	return ""
}

func getBucketRegion(context context.T, instanceRegion, bucketName string, httpProvider HttpProvider) (region string) {
	log := context.Log()
	regionalEndpoint, _ := getS3Endpoint(context, instanceRegion)

	// if we can get the region from a Signed HeadBucket request, then we will return that region
	region = getBucketRegionFromSignedHeadBucketRequestFunc(context, instanceRegion, regionalEndpoint, bucketName)
	if region != "" {
		return region
	}

	// When using virtual hosted–style buckets with SSL, the SSL wild-card certificate
	// only matches buckets that do not contain dots (".").  To work around this, try
	// to connect using HTTP in the case that the HTTPS connection attempt fails.
	protocols := []string{"https", "http"}

	// In CN regions, if the HEAD request is sent to the correct regional endpoint but the
	// bucket does not allow public access, then the request will fail with a 401 status code
	// and no bucket region information will be included in the header.  For this reason,
	// always try both the regional endpoint for the instance region as well as one other
	// endpoint.  This should enable the HEAD request to successfully discover the bucket
	// region in CN regions, and may be helpful in other partitions as well.
	endpoints := []string{}
	if regionalEndpoint != "" {
		endpoints = append(endpoints, regionalEndpoint)
	}
	fallbackEndpoint := getFallbackS3EndpointFunc(context, instanceRegion)
	if fallbackEndpoint != regionalEndpoint && fallbackEndpoint != "" {
		endpoints = append(endpoints, fallbackEndpoint)
	}

	for _, endpoint := range endpoints {
		// Strip scheme — endpoints may now be scheme-prefixed from GetServiceEndpoint
		bareEndpoint := strings.TrimPrefix(strings.TrimPrefix(endpoint, "https://"), "http://")
		for _, proto := range protocols {
			url := proto + "://" + bucketName + "." + bareEndpoint
			resp, err := httpProvider.Head(url)
			if err == nil && resp != nil {
				if resp.Header != nil {
					region = resp.Header.Get(bucketRegionHeader)
					if region != "" {
						log.Infof("HEAD response from endpoint %v indicates bucket %v is in region %v",
							endpoint, bucketName, region)
						return region
					}
				}
				// Got a response, no need to try other protocols for this endpoint
				break
			}
		}
	}

	log.Infof("no region in HEAD response for bucket %v", bucketName)
	return
}

// Maps bucket name to the AWS region where the bucket is hosted.
// This is a singleton and is thread-safe.
type bucketRegionMap struct {
	bucketNameCache cache.Cache
	mutex           sync.RWMutex
}

type bucketRegionMapItem struct {
	value string
}

func (i bucketRegionMapItem) Size() uint64 {
	return 1 // max cache size = max item count
}

var bucketRegionMapInstance *bucketRegionMap
var once sync.Once

// Returns the singleton instance, creating it if necessary
func getBucketRegionMap() *bucketRegionMap {
	once.Do(func() {
		bucketRegionMapInstance = &bucketRegionMap{
			bucketNameCache: cache.New(bucketRegionCacheItemCountMax,
				cache.EvictionPolicy(cache.LeastRecentlyUsed)),
		}
	})
	return bucketRegionMapInstance
}

// Get the region for the specified bucket name.  If bucketName exists in the
// map, returns the bucket region and true.  Otherwise, returns "" and false.
func (m *bucketRegionMap) Get(bucketName string) (region string, ok bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	items := m.bucketNameCache.Get(bucketName)
	if len(items) > 0 && items[0] != nil {
		if item, ok := items[0].(bucketRegionMapItem); ok {
			return item.value, true
		}
	}
	return "", false
}

// Add an entry mapping bucketName to the specified region.
func (m *bucketRegionMap) Put(bucketName, region string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.bucketNameCache.Put(bucketName, bucketRegionMapItem{region})
}

// Remove the entry for the specified bucket name, if present
func (m *bucketRegionMap) Remove(bucketName string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.bucketNameCache.Remove(bucketName)
}

// Indicates whether the HTTP response code indicates that the response
// may contain information about the bucket region.
// References:
//
//	https://docs.aws.amazon.com/AmazonS3/latest/dev/Redirects.html
//	https://docs.aws.amazon.com/AmazonS3/latest/API/ErrorResponses.html
func isRedirectResponseCode(responseCode int) bool {
	return responseCode == 301 || responseCode == 307 || responseCode == 400
}

// Replaces the Host field of the request URL to match endpointUrl
func fixupRequestUrl(log log.T, request *smithyhttp.Request, endpointUrl string) {
	endpointUrl = removeProtocol(removeTrailingSlash(endpointUrl))
	originalUrl := ParseAmazonS3URL(log, request.Request.URL)
	if originalUrl.IsValidS3URI {
		if originalUrl.IsPathStyle {
			request.Request.URL.Host = endpointUrl
		} else {
			request.Request.URL.Host = originalUrl.Bucket + "." + endpointUrl
		}
	} else {
		log.Errorf("invalid request URL, not fixing up: %v", request.Request.URL)
	}
}

// Trims the protocol prefix (e.g. "https://") from the given URL string
func removeProtocol(url string) string {
	idx := strings.Index(url, "://")
	if idx >= 0 {
		if idx+3 < len(url) {
			return url[idx+3:]
		} else {
			return ""
		}
	} else {
		return url
	}
}

// Removes trailing slashes from the given URL string
func removeTrailingSlash(url string) string {
	return strings.TrimRight(url, "/")
}

// A http.RoundTripper implementation that captures the bucket region that is
// included in certain responses from S3.
//
// The bucket name -> region mapping is stored in the RegionBucketMap, a shared
// data structure.  This makes it available for use in the SDK request.Handler chains.
type s3BucketRegionHeaderCapturingTransport struct {
	delegate http.RoundTripper
	logger   log.T
}

// Create a new s3BucketRegionHeaderCapturingTransport
func newS3BucketRegionHeaderCapturingTransport(log log.T, appConfig appconfig.SsmagentConfig) *s3BucketRegionHeaderCapturingTransport {
	return &s3BucketRegionHeaderCapturingTransport{
		delegate: network.GetDefaultTransport(log, appConfig),
		logger:   log,
	}
}

// Process the request and return the response
func (t *s3BucketRegionHeaderCapturingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := t.delegate.RoundTrip(request)
	if err == nil && response != nil && isRedirectResponseCode(response.StatusCode) {
		if bucketRegion := t.getBucketRegionFromResponse(response); bucketRegion != "" {
			parseOutput := ParseAmazonS3URL(t.logger, request.URL)
			if parseOutput.IsValidS3URI && parseOutput.Bucket != "" {
				t.logger.Infof("caching region %v for bucket %v from S3 response header", bucketRegion, parseOutput.Bucket)
				getBucketRegionMap().Put(parseOutput.Bucket, bucketRegion)
				// Return a 500 status code to trigger SDK retry.
				// On retry, the Build middleware will rewrite the endpoint
				// to the correct region from the cache.
				response.StatusCode = retryOnRedirectResponseCode
			} else {
				t.logger.Errorf("failed to parse request URL %v", request.URL)
			}
		}
	}
	return response, err
}

// Tries to determine the correct bucket region from the given response.
// If the region could not be determined, returns "".
func (t *s3BucketRegionHeaderCapturingTransport) getBucketRegionFromResponse(response *http.Response) string {
	region := t.getBucketRegionFromResponseHeader(response)
	if region == "" {
		region = t.getBucketRegionFromResponseBody(response)
	}
	return region
}

// Tries to determine the correct bucket region from the given response header.
// If the region could not be determined, returns "".
func (t *s3BucketRegionHeaderCapturingTransport) getBucketRegionFromResponseHeader(response *http.Response) string {
	region := ""
	if response.Header != nil {
		region = response.Header.Get(bucketRegionHeader)
	}
	return region
}

var getResponseBodyBufsize = 1024
var getResponseBodyMaxLength = 1024 * 1024

// Tries to determine the correct bucket region from the body of the given
// response.  If the region cannot be determined, returns "".
func (t *s3BucketRegionHeaderCapturingTransport) getBucketRegionFromResponseBody(response *http.Response) string {
	region := ""
	body, err := t.getResponseBody(response)
	if err == nil {
		region = t.extractRegionFromBody(body)
	}
	return region
}

// Returns a []byte containing the response body.
// Also sets response.Body to a new Reader backed by the []byte,
// so that the caller also has access to the body contents, and closes
// the original response.Body.
func (t *s3BucketRegionHeaderCapturingTransport) getResponseBody(response *http.Response) ([]byte, error) {
	resultBuf := make([]byte, 0, getResponseBodyBufsize)
	readBuf := make([]byte, getResponseBodyBufsize)
	var resultErr error
	for len(resultBuf) < getResponseBodyMaxLength {
		n, readErr := response.Body.Read(readBuf)
		if n > 0 {
			toCopy := n
			toCopyMax := getResponseBodyMaxLength - len(resultBuf)
			if toCopy > toCopyMax {
				toCopy = toCopyMax
				resultErr = fmt.Errorf("getResponseBody(): buffer length exceeded")
			}
			resultBuf = append(resultBuf, readBuf[:toCopy]...)
		}
		if readErr != nil || resultErr != nil {
			if resultErr == nil && readErr != io.EOF {
				resultErr = readErr
			}
			break
		}
	}
	response.Body.Close()
	response.Body = ioutil.NopCloser(bytes.NewReader(resultBuf))
	return resultBuf, resultErr
}

// S3 REST API error response structure used for XML unmarshalling
type xmlResponseError struct {
	XMLName  xml.Name `xml:"Error"`
	Code     string
	Message  string
	Region   string
	Endpoint string
}

// Tries to extract the correct bucket region from the given response body XML.
// If successful, returns the region name (e.g. "eu-west-1").  If not successful,
// returns "".
//
// The following paths are checked:
//   - Error/Region - if present, contains the region name (e.g. "us-east-1")
//   - Error/Endpoint - if present, contains an endpoint url from which the
//     region can be determined (e.g. "bucket-1.eu-west-1.amazonaws.com")
func (t *s3BucketRegionHeaderCapturingTransport) extractRegionFromBody(bodyContents []byte) (region string) {
	resp := xmlResponseError{}
	err := xml.Unmarshal(bodyContents, &resp)
	if err == nil {
		if resp.Region != "" {
			region = resp.Region
		} else {
			rawUrl := &url.URL{
				Scheme: "https",
				Host:   resp.Endpoint,
			}
			parsedUrl := ParseAmazonS3URL(t.logger, rawUrl)
			if parsedUrl.IsValidS3URI {
				region = parsedUrl.Region
			}
		}
	}
	return
}
