// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package s3util contains methods for interacting with S3.
package s3util

import (
	"net/http"

	"github.com/aws/amazon-ssm-agent/agent/backoffconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/network"
	"github.com/cenkalti/backoff"
)

type HttpProvider interface {
	Head(url string) (*http.Response, error)
}

// HttpProviderImpl provides http capabilities
type HttpProviderImpl struct {
	logger log.T
}

var getHeadBucketTransportDelegate = func(log log.T) http.RoundTripper {
	return network.GetDefaultTransport(log)
}

func (p HttpProviderImpl) Head(url string) (resp *http.Response, err error) {
	exponentialBackoff, err := backoffconfig.GetDefaultExponentialBackoff()
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{
		Transport: makeHeadBucketTransport(p.logger, getHeadBucketTransportDelegate(p.logger)),
	}

	op := func() error {
		resp, err = httpClient.Head(url)
		if err != nil {
			p.logger.Debugf("attempt failed for HTTP HEAD request: url=%v, error=%v", url, err)
		}
		return err
	}

	backoff.Retry(op, exponentialBackoff)

	if err != nil {
		p.logger.Errorf("HTTP HEAD request failed: url=%v, error=%v", url, err)
	}
	return resp, err
}

// RoundTripper with special handling for the locationless redirects that S3 returns.
type headBucketTransport struct {
	logger   log.T
	delegate http.RoundTripper
}

// Creates a new headBucketTransport using the supplied logger and delegate.
func makeHeadBucketTransport(logger log.T, delegate http.RoundTripper) headBucketTransport {
	return headBucketTransport{
		logger:   logger,
		delegate: delegate,
	}
}

// Sends an HTTP request an returns the result.  In most cases, returns the delegate's
// response without modification.  The only exception is when the delegate returns a redirect
// response with no Location header.  In that case, we change the response code to 200 keep
// the Go http.Client from swallowing the response and returning an error.
func (trans headBucketTransport) RoundTrip(request *http.Request) (resp *http.Response, err error) {
	resp, err = trans.delegate.RoundTrip(request)
	if err == nil && resp != nil && goHttpClientWillFollowRedirect(resp.StatusCode) {
		if resp.Header != nil && resp.Header.Get("Location") == "" && resp.Header.Get(bucketRegionHeader) != "" {
			logger.Debugf("redirect response missing Location header, overriding status code")
			resp.StatusCode = 200
		}
	}
	return
}

// See redirectBehavior() in http.Client
func goHttpClientWillFollowRedirect(statusCode int) bool {
	return statusCode == 301 || statusCode == 302 || statusCode == 303 || statusCode == 307 || statusCode == 308
}
