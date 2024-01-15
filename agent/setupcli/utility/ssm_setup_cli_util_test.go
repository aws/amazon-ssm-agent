// Copyright 2023 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
package utility

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHttpDownload_Success(t *testing.T) {
	httpClient := &http.Client{}
	httpClient.Transport = roundTripFunc(func(req *http.Request) *http.Response {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioutil.NopCloser(bytes.NewReader([]byte("3.1.1188.0"))),
			Header:     http.Header{},
		}
	})
	version, err := HttpReadContent("stableUrl", httpClient)
	assert.NoError(t, err)
	assert.Equal(t, "3.1.1188.0", string(version))
}

func TestHttpDownload_Failure(t *testing.T) {
	httpClient := &http.Client{}
	httpClient.Transport = roundTripFunc(func(req *http.Request) *http.Response {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       ioutil.NopCloser(bytes.NewReader([]byte("3.1.1188.0"))),
			Header:     http.Header{},
		}
	})
	version, err := HttpReadContent("stableUrl", httpClient)
	assert.Error(t, err)
	assert.Equal(t, "", string(version))
}

type roundTripFunc func(req *http.Request) *http.Response

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}
