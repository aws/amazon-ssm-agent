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
	"fmt"
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
	// DomainForMetadataService
	ServiceDomainResource = "/latest/meta-data/services/domain"
	// EC2MetadataRequestTimeout specifies the timeout when making web request
	EC2MetadataRequestTimeout = time.Duration(2 * time.Second)
	// EC2MetadataTokenURL provides the token resource for metadata v2
	EC2MetadataTokenURL = "/latest/api/token"
	// EC2MetadataTokenExpireHeader provides the token expire header
	EC2MetadataTokenExpireHeader = "X-aws-ec2-metadata-token-ttl-seconds"
	// Token expiration time is 6 hour
	EC2MetadataTokenExpireTime = "21600"
	// Token header for metadata v2
	EC2MetadataTokenHeader = "X-aws-ec2-metadata-token"
	// Status code for success http request
	EC2MetadataSuccessStatus = 200
	// Unauthorized status code for token expire
	EC2MetadataUnauthorizedStatus = 401
)

// InstanceIdentityDocument stores the values fetched from querying instance metadata
/* Sample Result
{
  "devpayProductCodes" : null,
  "availabilityZone" : "us-east-1c",
  "privateIp" : "2.152.66.15",
  "instanceId" : "i-e0a8424b",
  "billingProducts" : null,
  "version" : "2010-08-31",
  "instanceType" : "m3.medium",
  "accountId" : "999999999999",
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
	Do(r *http.Request) (*http.Response, error)
}

// EC2MetadataClient is used to make requests to instance metadata
type EC2MetadataClient struct {
	client httpClient
}

var metadata_token = ""

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

	var resp []byte
	var err error

	if resp, err = c.readResourceFromMetaDataV2(endpoint); err == nil {
		return resp, err
	}

	if resp, err = c.readResourceFromMetaDataV1(endpoint); err == nil {
		return resp, err
	}

	return nil, err
}

func (c EC2MetadataClient) readResourceFromMetaDataV1(endpoint string) (rst []byte, err error) {
	resp, err := c.client.Get(endpoint)
	if err != nil {
		return nil, err
	}

	if resp != nil && resp.StatusCode != EC2MetadataSuccessStatus {
		return nil, fmt.Errorf("Error to get resource from MetaDataV1, %v", resp)
	}

	defer resp.Body.Close()

	return ioutil.ReadAll(resp.Body)
}

func (c *EC2MetadataClient) readResourceFromMetaDataV2(endpoint string) (rst []byte, err error) {

	// Get the token when in the first time
	if len(metadata_token) == 0 {
		if err := c.refreshToken(); err != nil {
			return nil, fmt.Errorf("Failed to refresh token for MetadataV2, %v", err)
		}
	}

	// Send request with current token
	if resp, err := c.retrieveInfoWithToken(endpoint); err == nil {
		// Token is active, return result
		return resp, err
	} else {
		if err = c.refreshToken(); err == nil {
			return c.retrieveInfoWithToken(endpoint)
		} else {
			return nil, fmt.Errorf("Failed to refresh token for MetadataV2, %v", err)
		}
	}
}

func (c *EC2MetadataClient) retrieveInfoWithToken(endpoint string) (rst []byte, err error) {
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(EC2MetadataTokenHeader, metadata_token)
	rsp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer rsp.Body.Close()
	if rsp != nil && rsp.StatusCode != EC2MetadataSuccessStatus {
		return nil, fmt.Errorf("Error to get from imdb v2, response %v", rsp)
	}

	return ioutil.ReadAll(rsp.Body)
}

// ServiceDomain from ec2 metadata client
func (c EC2MetadataClient) ServiceDomain() (string, error) {
	domain, err := c.ReadResource(ServiceDomainResource)
	if err != nil {
		return "", err
	}
	return string(domain), nil
}

// Validate whether token is expired, and refresh it if expired
func (c *EC2MetadataClient) refreshToken() (err error) {

	var req *http.Request
	var rsp *http.Response

	url := c.resourceServiceURL(EC2MetadataTokenURL)
	req, _ = http.NewRequest("PUT", url, nil)
	req.Header.Set(EC2MetadataTokenExpireHeader, EC2MetadataTokenExpireTime)

	rsp, err = c.client.Do(req)

	if err != nil {
		// failed to get the new token from metadata service
		return err
	}
	defer rsp.Body.Close()
	token, err := ioutil.ReadAll(rsp.Body)

	if err != nil {
		return err
	}

	metadata_token = string(token)
	return nil
}
