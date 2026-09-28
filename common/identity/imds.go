// Copyright 2026 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package identity

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
)

const imdsTimeout = 2 * time.Second

// IsIMDSAvailableForProvider checks if the IMDS for the given provider is reachable
var IsIMDSAvailableForProvider = func(provider appconfig.Provider) (bool, error) {
	switch provider {
	case appconfig.EC2:
		return checkEC2IMDSAvailable(), nil
	case appconfig.Azure:
		return checkAzureIMDSAvailable(), nil
	default:
		return false, fmt.Errorf("unknown provider: %s", provider)
	}
}

// newEC2IMDSClient creates an IMDS client using aws-sdk-go-v2. Overridable for testing.
var newEC2IMDSClient = func() (*imds.Client, error) {
	client := imds.New(imds.Options{})
	return client, nil
}

// checkEC2IMDSAvailable checks if EC2 IMDS is reachable
func checkEC2IMDSAvailable() bool {
	client, err := newEC2IMDSClient()
	if err != nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), imdsTimeout)
	defer cancel()

	_, err = client.GetMetadata(ctx, &imds.GetMetadataInput{
		Path: "instance-id",
	})
	return err == nil
}

// checkAzureIMDSAvailable checks if Azure IMDS is reachable
func checkAzureIMDSAvailable() bool {
	client := &http.Client{Timeout: imdsTimeout}

	req, err := http.NewRequest("GET", appconfig.AzureIMDSEndpoint, nil)
	if err != nil {
		return false
	}
	req.Header.Set("Metadata", "true")
	q := req.URL.Query()
	q.Set("format", "json")
	q.Set("api-version", appconfig.AzureIMDSAPIVersion)
	req.URL.RawQuery = q.Encode()

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}
