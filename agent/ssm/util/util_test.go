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

// Package util contains helper function common for ssm service
package util

import (
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/url"
	"testing"
)

type MockClient struct {
	DoFunc func(req *http.Request) (*http.Response, error)
}

func (m *MockClient) Do(req *http.Request) (*http.Response, error) {
	if m.DoFunc != nil {
		return m.DoFunc(req)
	}
	// just in case you want default correct return value
	return &http.Response{}, nil
}
func TestHTTPSRedirect(t *testing.T) {
	origRequests := []*http.Request{
		{
			Method: "POST",
			URL: &url.URL{
				Scheme: "https",
			},
		},
	}
	redirectRequest := &http.Request{
		Method: "POST",
		URL: &url.URL{
			Scheme: "https",
		},
	}
	client := &MockClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			//This function mocks a redirect. Do gets called and then gets sent to the CheckRedirect handler.
			//via is requests already made and req is the upcoming request
			err := disableHTTPDowngrade(req, origRequests)
			return &http.Response{
				StatusCode: http.StatusOK,
			}, err
		},
	}
	resp, err := client.Do(redirectRequest)
	assert.Nil(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestHTTPDowngrade(t *testing.T) {
	origRequests := []*http.Request{
		{
			Method: "POST",
			URL: &url.URL{
				Scheme: "https",
			},
		},
	}
	redirectRequest := &http.Request{
		Method: "POST",
		URL: &url.URL{
			Scheme: "http",
		},
	}
	client := &MockClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			//This function mocks a redirect. Do gets called and then gets sent to the CheckRedirect handler.
			//via is requests already made and req is the upcoming request
			err := disableHTTPDowngrade(req, origRequests)
			return &http.Response{
				StatusCode: http.StatusPermanentRedirect,
			}, err
		},
	}
	resp, err := client.Do(redirectRequest)
	assert.NotNil(t, err)
	assert.Equal(t, http.StatusPermanentRedirect, resp.StatusCode)
}
