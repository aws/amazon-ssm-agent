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

// Package s3util contains utilities for working with S3
package s3util

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/log"
)

const (
	// Regex for S3 URLs, VPCE interface endpoint
	vpceUrlPattern = "^((.+)\\.)?" + // maybe a bucket name
		"(bucket|accesspoint|control)\\.vpce-[-a-z0-9]+\\." + // VPC endpoint DNS name
		"s3[.-]" + // S3 service name
		"(([-a-z0-9]+)\\.)?" + // region name, optional for us-east-1
		"vpce\\.amazonaws\\.com"
	vpceUrlPatternBucketIdx = 2
	vpceUrlPatternRegionIdx = 5

	// Regex for S3 URLs, public S3 endpoint
	nonVpceUrlPattern = "^((.+)\\.)?" + // maybe a bucket name
		"s3[.-](website[-.])?(accelerate\\.)?(dualstack[-.])?" + // S3 service name with optional features
		"(([-a-z0-9]+)\\.)?" + // region name, optional for us-east-1
		"amazonaws\\.com"
	nonVpceUrlPatternBucketIdx = 2
	nonVpceUrlPatternRegionIdx = 7

	// cn- is a prefix for China region
	ChinaRegionPrefix = "cn-"
)

var (
	vpceUrlRegex    = regexp.MustCompile(vpceUrlPattern)
	nonVpceUrlRegex = regexp.MustCompile(nonVpceUrlPattern)
)

// AmazonS3URL holds interesting pieces after parsing a s3 URL
type AmazonS3URL struct {
	IsValidS3URI bool
	IsPathStyle  bool
	Bucket       string
	Key          string
	Region       string
}

// IsBucketAndKeyPresent checks the AmazonS3URL if it contains both bucket and key
func (output AmazonS3URL) IsBucketAndKeyPresent() bool {
	return output.IsValidS3URI && output.Bucket != "" && output.Key != "" && output.Region != ""
}

// ParseAmazonS3URL parses an HTTP/HTTPS URL for an S3 resource and returns an
// AmazonS3URL object.
//
// S3 URLs come in two flavors: virtual hosted-style URLs and path-style URLs.
// Virtual hosted-style URLs have the bucket name as the first component of the
// hostname, e.g.
//
//	https://mybucket.s3.us-east-1.amazonaws.com/a/b/c
//
// Path-style URLs have the bucket name as the first component of the path, e.g.
//
//	https://s3.us-east-1.amazonaws.com/mybucket/a/b/c
//
// S3 supports a few features that affect how the URL is formed:
//   - Website endpoints - "s3.$REGION" becomes "s3-website[-.]$REGION"
//   - Transfer acceleration - "s3" becomes "s3-accelerate"
//   - Dual-stack (IPv4/IPv6) - "s3" becomes "s3.dualstack"
//   - Can be used with acceleration - "s3-accelerate.dualstack"
//   - VPC endpoints - "s3.$REGION.amazonaws.com" becomes
//     "bucket.$VPC_ENDPOINT_ID.s3.$REGION.vpce.amazonaws.com"
//
// References:
// https://docs.aws.amazon.com/AmazonS3/latest/userguide/VirtualHosting.html
// https://docs.aws.amazon.com/AmazonS3/latest/userguide/WebsiteEndpoints.html
// https://docs.aws.amazon.com/AmazonS3/latest/userguide/transfer-acceleration-getting-started.html
// https://docs.aws.amazon.com/AmazonS3/latest/userguide/dual-stack-endpoints.html
// https://docs.aws.amazon.com/AmazonS3/latest/userguide/privatelink-interface-endpoints.html
func ParseAmazonS3URL(log log.T, s3URL *url.URL) (output AmazonS3URL) {
	output = AmazonS3URL{
		IsValidS3URI: false,
		IsPathStyle:  false,
		Bucket:       "",
		Key:          "",
		Region:       "",
	}

	output, err := parseBucketAndRegionFromHost(s3URL.Host, vpceUrlRegex, vpceUrlPatternBucketIdx, vpceUrlPatternRegionIdx)
	if err != nil {
		output, err = parseBucketAndRegionFromHost(s3URL.Host, nonVpceUrlRegex, nonVpceUrlPatternBucketIdx, nonVpceUrlPatternRegionIdx)
		if err != nil {
			output.IsValidS3URI = false
			return
		}
	}

	output.IsPathStyle = output.Bucket == ""

	path := s3URL.Path

	if output.IsPathStyle {
		// no bucket name in the authority, parse it from the path
		output.IsPathStyle = true

		// grab the encoded path so we don't run afoul of '/'s in the bucket name
		if path == "/" || path == "" {
		} else {
			path = path[1:]
			index := strings.Index(path, "/")
			if index == -1 {
				// https://s3.amazonaws.com/bucket
				output.Bucket = path
				output.Key = ""
			} else if index == (len(path) - 1) {
				// https://s3.amazonaws.com/bucket/
				output.Bucket = strings.TrimRight(path, "/")
				output.Key = ""
			} else {
				// https://s3.amazonaws.com/bucket/key
				output.Bucket = path[:index]
				output.Key = path[index+1:]
			}
		}
	} else {
		// bucket name in the host, path is the object key
		if path == "/" || path == "" {
			output.Key = ""
		} else {
			output.Key = path[1:]
		}
	}

	if strings.EqualFold(output.Region, "external-1") {
		output.Region = "us-east-1"
	} else if output.Region == "" {
		// s3 bucket URL in us-east-1 doesn't include region
		output.Region = "us-east-1"
	}

	return
}

func parseBucketAndRegionFromHost(host string, re *regexp.Regexp, bucketIdx, regionIdx int) (AmazonS3URL, error) {
	result := re.FindStringSubmatch(host)
	if result != nil && len(result) > bucketIdx && len(result) > regionIdx {
		return AmazonS3URL{
			IsValidS3URI: true,
			Bucket:       result[bucketIdx],
			Region:       result[regionIdx],
		}, nil
	} else {
		return AmazonS3URL{}, errors.New("no match")
	}
}

// String returns the string representation of the AmazonS3URL
func (output AmazonS3URL) String() string {
	return fmt.Sprintf("{Region: %s; Bucket: %s; Key: %s; IsValidS3URI: %v; IsPathStyle: %v}",
		output.Region, output.Bucket, output.Key, output.IsValidS3URI, output.IsPathStyle)
}
