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
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"
)

const (
	// EC2MetadataServiceURL is url for instance metadata.
	EC2MetadataServiceURL = "http://169.254.169.254"
	// SecurityCredentialsResource provides iam credentials
	SecurityCredentialsResource = "/latest/meta-data/iam/security-credentials/"
	// InstanceIdentityDocumentResource provides instance information like instance id, region, availability
	InstanceIdentityDocumentResource = "/latest/dynamic/instance-identity/document"
	// InstanceIdentityDocumentSignatureResource provides instance identity signature
	InstanceIdentityDocumentSignatureResource = "/latest/dynamic/instance-identity/signature"
	// SignedInstanceIdentityDocumentResource provides pkcs7 public key pair value
	SignedInstanceIdentityDocumentResource = "/latest/dynamic/instance-identity/pkcs7"
	// EC2MetadataRequestTimeout specifies the timeout when making web request
	EC2MetadataRequestTimeout = time.Duration(2 * time.Second)
)

// InstanceIdentityDocument stores the values fetched from querying instance metadata
/* Sample Result
{
  "devpayProductCodes" : null,
  "availabilityZone" : "us-east-1c",
  "privateIp" : "172.31.32.24",
  "instanceId" : "i-e0a8424b",
  "billingProducts" : null,
  "version" : "2010-08-31",
  "instanceType" : "m3.medium",
  "accountId" : "099688301723",
  "pendingTime" : "2015-08-06T17:06:28Z",
  "imageId" : "ami-1ecae776",
  "kernelId" : null,
  "ramdiskId" : null,
  "architecture" : "x86_64",
  "region" : "us-east-1"
}
*/
type InstanceIdentityDocument struct {
	InstanceID          string   `json:"instanceId"`
	BillingProducts     []string `json:"billingProducts"`
	ImageID             string   `json:"imageId"`
	Architecture        string   `json:"architecture"`
	PendingTimeAsString string   `json:"pendingTime"`
	InstanceType        string   `json:"instanceType"`
	AccountID           string   `json:"accountId"`
	KernelID            string   `json:"kernelId"`
	RamdiskID           string   `json:"ramdiskId"`
	Region              string   `json:"region"`
	Version             string   `json:"version"`
	PrivateIP           string   `json:"privateIp"`
	DevpayProductCodes  string   `json:"devpayProductCodes"`
	AvailabilityZone    string   `json:"availabilityZone"`
}

// PendingTime parses the PendingTimeAsString field into a time.
func (iid *InstanceIdentityDocument) PendingTime() (time.Time, error) {
	return time.Parse(time.RFC3339, iid.PendingTimeAsString)
}

// SetPendingTime sets the PendingTimeAsString field by formatting a given time.
func (iid *InstanceIdentityDocument) SetPendingTime(pendingTime time.Time) {
	iid.PendingTimeAsString = pendingTime.UTC().Format(time.RFC3339)
}

// httpClient is used to make Get web requests to a url endpoint
type httpClient interface {
	Get(string) (*http.Response, error)
}

// EC2MetadataClient is used to make requests to instance metadata
type EC2MetadataClient struct {
	client httpClient
}

// NewEC2MetadataClient creates new EC2MetadataClient
func NewEC2MetadataClient() *EC2MetadataClient {
	httpClient := &http.Client{Timeout: EC2MetadataRequestTimeout}
	return &EC2MetadataClient{client: httpClient}
}

// InstanceIdentityDocument returns the instance document details querying the metadata
func (c EC2MetadataClient) InstanceIdentityDocument() (*InstanceIdentityDocument, error) {
	rawIidResp, err := c.ReadResource(InstanceIdentityDocumentResource)
	if err != nil {
		return nil, err
	}

	var iid InstanceIdentityDocument

	err = json.Unmarshal(rawIidResp, &iid)
	if err != nil {
		return nil, err
	}

	return &iid, nil
}

func (c EC2MetadataClient) resourceServiceURL(path string) string {
	return EC2MetadataServiceURL + path
}

// ReadResource reads from the url path
func (c EC2MetadataClient) ReadResource(path string) ([]byte, error) {
	endpoint := c.resourceServiceURL(path)

	resp, err := c.client.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return ioutil.ReadAll(resp.Body)
}
