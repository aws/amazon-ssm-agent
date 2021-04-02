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

package endpoint

import (
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
)

var awsRegionServiceDomainMap = map[string]string{
	"ap-east-1":      "amazonaws.com",
	"ap-northeast-1": "amazonaws.com",
	"ap-northeast-2": "amazonaws.com",
	"ap-south-1":     "amazonaws.com",
	"ap-southeast-1": "amazonaws.com",
	"ap-southeast-2": "amazonaws.com",
	"ca-central-1":   "amazonaws.com",
	"cn-north-1":     "amazonaws.com.cn",
	"cn-northwest-1": "amazonaws.com.cn",
	"eu-central-1":   "amazonaws.com",
	"eu-north-1":     "amazonaws.com",
	"eu-west-1":      "amazonaws.com",
	"eu-west-2":      "amazonaws.com",
	"eu-west-3":      "amazonaws.com",
	"me-south-1":     "amazonaws.com",
	"sa-east-1":      "amazonaws.com",
	"us-east-1":      "amazonaws.com",
	"us-east-2":      "amazonaws.com",
	"us-gov-east-1":  "amazonaws.com",
	"us-gov-west-1":  "amazonaws.com",
	"us-west-1":      "amazonaws.com",
	"us-west-2":      "amazonaws.com",
}

const (
	ec2ServiceDomainResource = "services/domain"
	defaultServiceDomain     = "amazonaws.com"
	defaultCNServiceDomain   = "amazonaws.com.cn"
	maxRetries               = 3
)

// iEC2MdsSdkClient defines the functions that ec2_identity depends on from the aws sdk
type iEC2MdsSdkClient interface {
	GetMetadata(string) (string, error)
}

// NewEC2MetadataClient creates new ec2 metadata client
func newEC2MetadataClient() iEC2MdsSdkClient {
	awsConfig := &aws.Config{}
	awsConfig = awsConfig.WithMaxRetries(3)
	awsConfig = awsConfig.WithEC2MetadataDisableTimeoutOverride(true)
	sess, _ := session.NewSession(awsConfig)

	return ec2metadata.New(sess)
}

var ec2Metadata = newEC2MetadataClient()

// getDefaultEndpoint returns the default endpoint for a service
func GetDefaultEndpoint(log log.T, service, region, serviceDomain string) string {
	if region == "" {
		log.Error("Cannot get default endpoint for service due to unspecified region.")
		return ""
	}

	var endpoint string
	if serviceDomain != "" {
		endpoint = serviceDomain
	} else if val, ok := awsRegionServiceDomainMap[region]; ok {
		endpoint = val
	} else {
		dynamicServiceDomain, err := ec2Metadata.GetMetadata(ec2ServiceDomainResource)
		if err == nil {
			endpoint = dynamicServiceDomain
		} else {
			log.Warnf("Failed to get service domain from ec2 metadata: %v", err)
		}
	}

	if endpoint == "" {
		if strings.HasPrefix(region, "cn-") {
			endpoint = defaultCNServiceDomain
		} else {
			endpoint = defaultServiceDomain
		}

		log.Warnf("Service domain not found in region-domain map and could not be retrieved from instance metadata. Using %s as default", endpoint)
	}

	return getServiceEndpoint(region, service, endpoint)
}

func getServiceEndpoint(region string, service string, endpoint string) string {
	return service + "." + region + "." + endpoint
}
