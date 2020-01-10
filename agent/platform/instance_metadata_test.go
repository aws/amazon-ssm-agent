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

package platform

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func MakeInstanceIdentityDocument() InstanceIdentityDocument {
	return InstanceIdentityDocument{
		AvailabilityZone:    "us-east-1a",
		Version:             "2010-08-31",
		Region:              "us-east-1",
		InstanceID:          "i-31497ee2",
		AccountID:           "405584108566",
		InstanceType:        "m3.large",
		ImageID:             "ami-15984f7e",
		PendingTimeAsString: "2015-08-06T17:06:28Z",
		BillingProducts:     []string{},
		KernelID:            "null",
		RamdiskID:           "null",
		DevpayProductCodes:  "null",
		Architecture:        "x86_64",
		PrivateIP:           "172.31.32.24",
	}
}

type testHTTPClient struct{}
type testHTTPClientResource struct{}

func ignoreError(v interface{}, _ error) interface{} {
	return v
}

var testClient = EC2MetadataClient{client: testHTTPClient{}}
var testResourceClient = EC2MetadataClient{client: testHTTPClientResource{}}
var expectediid = MakeInstanceIdentityDocument()
var expectedservicedomain = "amazonaws.com"
var testActiveToken = "AQAAAJL52N97Ie16Z4WflqNjLh-OVR_BN2mlEKjzRog13u8E8x2Vrw=="
var testMetaData = "latest-metadata"
var testResponse = map[string]string{
	testClient.resourceServiceURL(InstanceIdentityDocumentResource): string(ignoreError(json.Marshal(expectediid)).([]byte)),
	testClient.resourceServiceURL(ServiceDomainResource):            expectedservicedomain,
	testClient.resourceServiceURL(EC2MetadataTokenURL):              testActiveToken,
	testClient.resourceServiceURL(""):                               testMetaData,
}

var testResourceResponce = map[string]string{
	testClient.resourceServiceURL(ServiceDomainResource): expectedservicedomain,
}

func (t testHTTPClientResource) Get(url string) (*http.Response, error) {
	resp, ok := testResponse[url]
	if ok {
		return &http.Response{
			Status:     "401 Unauthorized",
			StatusCode: 401,
			Proto:      "HTTP/1.0",
			Body:       ioutil.NopCloser(bytes.NewReader([]byte(resp))),
		}, nil
	}
	return nil, errors.New("404")
}

func (t testHTTPClientResource) Do(r *http.Request) (*http.Response, error) {
	endpoint := r.URL.Scheme + "://" + r.Host + r.URL.Path
	resp, ok := testResponse[endpoint]
	if ok {
		switch endpoint {
		case EC2MetadataServiceURL + EC2MetadataTokenURL:
			return &http.Response{
				Status:     "200 OK",
				StatusCode: 200,
				Proto:      "HTTP/1.0",
				Body:       ioutil.NopCloser(bytes.NewReader([]byte(resp))),
			}, nil
		case EC2MetadataServiceURL + ServiceDomainResource:
			return &http.Response{
				Status:     "200 OK",
				StatusCode: 200,
				Proto:      "HTTP/1.0",
				Body:       ioutil.NopCloser(bytes.NewReader([]byte(resp))),
			}, nil
		}
	}
	return nil, errors.New("404")
}

// Get is a mock of the http.Client.Get that reads its responses from the map
// above and defaults to erroring.
func (c testHTTPClient) Get(url string) (*http.Response, error) {
	resp, ok := testResponse[url]
	if ok {
		return &http.Response{
			Status:     "200 OK",
			StatusCode: 200,
			Proto:      "HTTP/1.0",
			Body:       ioutil.NopCloser(bytes.NewReader([]byte(resp))),
		}, nil
	}
	return nil, errors.New("404")
}

func (c testHTTPClient) Do(r *http.Request) (*http.Response, error) {
	endpoint := r.URL.Scheme + "://" + r.Host + r.URL.Path
	resp, ok := testResponse[endpoint]
	if ok {
		switch endpoint {
		case EC2MetadataServiceURL:
			return &http.Response{
				Status:     "401 Unauthorized",
				StatusCode: 401,
				Proto:      "HTTP/1.0",
				Body:       ioutil.NopCloser(bytes.NewReader([]byte(resp))),
			}, nil
		case EC2MetadataServiceURL + EC2MetadataTokenURL:
			return &http.Response{
				Status:     "200 OK",
				StatusCode: 200,
				Proto:      "HTTP/1.0",
				Body:       ioutil.NopCloser(bytes.NewReader([]byte(resp))),
			}, nil
		case EC2MetadataServiceURL + ServiceDomainResource:
			return &http.Response{
				Status:     "200 OK",
				StatusCode: 200,
				Proto:      "HTTP/1.0",
				Body:       ioutil.NopCloser(bytes.NewReader([]byte(resp))),
			}, nil
		}
	}
	return nil, errors.New("404")
}

func TestInstanceIdentityDocument(t *testing.T) {
	iid, err := testClient.InstanceIdentityDocument()
	assert.Nil(t, err)
	assert.Equal(t, &expectediid, iid)
}

func TestServiceDomain(t *testing.T) {
	domain, err := testClient.ServiceDomain()
	assert.Nil(t, err)
	assert.Equal(t, expectedservicedomain, domain)
}

func TestMetadataRefreshToken(t *testing.T) {
	err := testClient.refreshToken()
	assert.Nil(t, err)
	assert.Equal(t, metadata_token, testActiveToken)
}

func TestReadResource(t *testing.T) {
	endpoint := ServiceDomainResource
	rsp, err := testResourceClient.ReadResource(endpoint)
	assert.Nil(t, err)
	assert.Equal(t, string(rsp), expectedservicedomain)
	assert.Equal(t, metadata_token, testActiveToken)
}

func TestReadResourceFromMetaDataV1(t *testing.T) {
	endpoint := EC2MetadataServiceURL + ServiceDomainResource
	rsp, err := testClient.readResourceFromMetaDataV1(endpoint)
	assert.Nil(t, err)
	assert.Equal(t, string(rsp), expectedservicedomain)
}

func TestReadResourceFromMetaDataV2(t *testing.T) {
	endpoint := EC2MetadataServiceURL + ServiceDomainResource
	rsp, err := testClient.readResourceFromMetaDataV2(endpoint)
	assert.Nil(t, err)
	assert.Equal(t, string(rsp), expectedservicedomain)
	assert.Equal(t, metadata_token, testActiveToken)
}

// TestPendingTime tests the parsing/formatting of the pending time field.
func TestPendingTime(t *testing.T) {
	mst := time.FixedZone("MST", -7*3600) // seven hours west of UTC

	testCases := []struct {
		timeAsString string
		timeAsTime   time.Time
	}{
		{
			timeAsString: "2015-08-06T17:06:28Z",
			timeAsTime:   time.Date(2015, 8, 6, 17, 6, 28, 0, time.UTC),
		},
		{
			timeAsString: "2015-08-14T15:38:28Z",
			timeAsTime:   time.Date(2015, 8, 14, 8, 38, 28, 0, mst),
		},
	}

	for _, testCase := range testCases {
		testParsePendingTime(t, testCase.timeAsString, testCase.timeAsTime.UTC())
		testFormatPendingTime(t, testCase.timeAsTime, testCase.timeAsString)
	}
}

// testParsePendingTime calls the PendingTime method and checks that the time is parsed correctly.
func testParsePendingTime(t *testing.T, pendingTimeAsString string, pendingTimeAsTime time.Time) {
	iid := InstanceIdentityDocument{PendingTimeAsString: pendingTimeAsString}

	pendingTimeParsed, err := iid.PendingTime()

	assert.Nil(t, err)
	assert.Equal(t, pendingTimeAsTime, pendingTimeParsed)
}

// testFormatPendingTime calls the SetPendingTime method and checks that the time is formatted correctly.
func testFormatPendingTime(t *testing.T, pendingTimeAsTime time.Time, pendingTimeAsString string) {
	iid := InstanceIdentityDocument{}

	iid.SetPendingTime(pendingTimeAsTime)

	assert.Equal(t, pendingTimeAsString, iid.PendingTimeAsString)
}
