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
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/Workiva/go-datastructures/cache"
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/network"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
)

const (
	bucketRegionHeader            = "X-Amz-Bucket-Region"
	retryOnRedirectResponseCode   = 500
	bucketRegionCacheItemCountMax = 128
)

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
func GetS3CrossRegionCapableSession(context context.T, bucketName string) (*session.Session, error) {
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
		config.Endpoint = &appConfig.S3.Endpoint
	}

	config.HTTPClient = &http.Client{
		Transport: newS3BucketRegionHeaderCapturingTransport(log, context.AppConfig()),
	}

	sess := session.New(config)
	sess.Handlers.Build.PushBack(request.MakeAddToUserAgentHandler(agentName, agentVersion))
	sess.Handlers.Validate.PushBackNamed(makeS3RegionCorrectingValidateHandler(log))
	sess.Handlers.Retry.PushFrontNamed(makeS3RegionCorrectingRetryHandler(log))

	return sess, nil
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
func getBucketRegion(context context.T, instanceRegion, bucketName string, httpProvider HttpProvider) (region string) {
	log := context.Log()
	regionalEndpoint := getS3Endpoint(context, instanceRegion)

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
	endpoints := []string{regionalEndpoint}
	fallbackEndpoint := getFallbackS3EndpointFunc(context, instanceRegion)
	if fallbackEndpoint != regionalEndpoint {
		endpoints = append(endpoints, fallbackEndpoint)
	}

	for _, endpoint := range endpoints {
		for _, proto := range protocols {
			url := proto + "://" + bucketName + "." + endpoint
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

// Returns a handler that corrects the region and endpoint.  If a request
// is targeting an S3 bucket which is known to reside in a region different
// from the region specified in the request, the handler will replace the
// request's region and endpoint with the correct region and endpoint for the
// bucket.  Whether or not the request has the correct region for the bucket
// is determined by checking the bucketRegionMap, which will have been populated
// during previous requests by s3BucketRegionHeaderCapturingTransport.
//
// The handler should be added to the handler list for the Validate step.  For
// example:
//
//	sess := session.New(config)
//	sess.Handlers.Validate.PushBackNamed(makeS3RegionCorrectingValidateHandler())
//
// This will ensure that this handler runs before the standard S3 client's Build
// handlers (which make their own modifications to the URL), and before the Sign
// handlers (which calculate the signature based on the region).
func makeS3RegionCorrectingValidateHandler(log log.T) request.NamedHandler {
	return request.NamedHandler{
		Name: "S3RegionCorrectingValidateHandler",
		Fn: func(request *request.Request) {
			if bucketName := getBucketFromParams(request.Params); bucketName != "" {
				if region, ok := getBucketRegionMap().Get(bucketName); ok {
					log.Infof("using cached region %v for bucket %v", region, bucketName)
					fixupRequest(log, request, region)
				}
			} else {
				log.Errorf("could not determine bucket name from params, request.Params=%v", request.Params)
			}
		},
	}
}

// Returns a Handler that prepares the request for retry, in the case where
// S3 has returned a response indicating that the requested S3 bucket is in
// a different region.
//
// This handler should be added to the Retry handler chain, as follows:
//
//	sess = session.New(config)
//	sess.Handlers.Retry.PushFrontNamed(makeS3RegionCorrectingRetryHandler(log))
func makeS3RegionCorrectingRetryHandler(log log.T) request.NamedHandler {
	return request.NamedHandler{
		Name: "S3RegionCorrectingRetryHandler",
		Fn: func(request *request.Request) {
			resp := request.HTTPResponse
			if resp != nil && isRedirectResponseCode(resp.StatusCode) {
				if bucketName := getBucketFromParams(request.Params); bucketName != "" {
					if correctRegion, ok := getBucketRegionMap().Get(bucketName); ok {
						log.Infof("received %v response from S3, sending requests for %v to %v",
							resp.StatusCode, bucketName, correctRegion)
						fixupRequest(log, request, correctRegion)
						request.HTTPResponse.StatusCode = retryOnRedirectResponseCode
					} else {
						log.Debugf("received %v response from S3, but bucket %v not found in bucket-region map",
							resp.StatusCode, bucketName)
					}
				} else {
					log.Errorf("could not determine bucket name from params, request.Params=%v", request.Params)
				}
			}
		},
	}
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

// Sets the request's region to the specified region.  The new region
// will be used for signing and automatic endpoint selection.  If the
// HTTP request URL has already been set, then the request URL will be
// regenerated using the new region and endpoint.
//
// Notes:
// request.Config.Endpoint is the optional custom endpoint from the
// agent appconfig.  We never overwrite this value if it is set, and
// if it is set, it will take precedence over the endpoint resolver
// when selecting the effective endpoint for requests.
//
// request.ClientInfo.Endpoint is the effective endpoint URL that is
// used when building HTTP requests.  We do overwrite this value with
// the selected endpoint URL.
func fixupRequest(log log.T, request *request.Request, newRegion string) {
	if endpointUrl := determineEndpointUrl(log, request, newRegion); endpointUrl != "" {
		request.Config.Region = &newRegion
		request.ClientInfo.SigningRegion = newRegion
		request.ClientInfo.Endpoint = endpointUrl
		if request.HTTPRequest != nil && request.HTTPRequest.URL != nil {
			fixupRequestUrl(log, request, endpointUrl)
		}
	}
}

// Replaces the Host field of the request URL to match endpointUrl
func fixupRequestUrl(log log.T, request *request.Request, endpointUrl string) {
	endpointUrl = removeProtocol(removeTrailingSlash(endpointUrl))
	originalUrl := ParseAmazonS3URL(log, request.HTTPRequest.URL)
	if originalUrl.IsValidS3URI {
		if originalUrl.IsPathStyle {
			request.HTTPRequest.URL.Host = endpointUrl
		} else {
			request.HTTPRequest.URL.Host = originalUrl.Bucket + "." + endpointUrl
		}
	} else {
		log.Errorf("invalid request URL, not fixing up: %v", request.HTTPRequest.URL)
	}
}

// Determines the correct endpoint for the request, given that newRegion is the
// correct region.  If the request has an explicitly configured endpoint, then that
// endpoint will be used.  Otherwise, returns the default S3 endpoint for newRegion.
// If the endpoint resolver fails to find the endpoint for the region, returns "".
func determineEndpointUrl(log log.T, request *request.Request, newRegion string) string {
	var endpoint = ""
	if request.Config.Endpoint != nil && *request.Config.Endpoint != "" {
		endpoint = *request.Config.Endpoint
	} else {
		resolver := request.Config.EndpointResolver
		if resolver == nil {
			log.Warnf("no endpoint resolver in request config, using default resolver. request: %v", request)
			resolver = endpoints.DefaultResolver()
		}
		if resolved, err := resolver.EndpointFor("s3", newRegion); err == nil {
			endpoint = resolved.URL
		} else {
			log.Warnf("failed to resolve S3 endpoint for region %v: %v", newRegion, err)
		}
	}
	return endpoint
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
