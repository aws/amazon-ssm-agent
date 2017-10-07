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

//Package s3util contains utilities for working with the file system.
package s3util

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/log"
)

const (
	// EndpointPattern is a valid regular expression for s3 url pattern
	EndpointPattern = "^(.+\\.)?s3[.-]([a-z0-9-]+)\\."

	// cn- is a prefix for China region
	ChinaRegionPrefix = "cn-"
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

// ParseAmazonS3URL parses a URL and returns AmazonS3URL object
func ParseAmazonS3URL(log log.T, s3URL *url.URL) (output AmazonS3URL) {
	output = AmazonS3URL{
		IsValidS3URI: false,
		IsPathStyle:  false,
		Bucket:       "",
		Key:          "",
		Region:       "",
	}

	match, _ := regexp.MatchString(EndpointPattern, s3URL.Host)
	if match == false {
		// Invalid S3 URI - hostname does not appear to be a valid S3 endpoint
		output.IsValidS3URI = match
		return
	}
	log.Debugf("%v is valid s3 url", s3URL.String())
	endpointRegEx, err := regexp.Compile(EndpointPattern)
	if err != nil {
		output.IsValidS3URI = false
		return
	}
	output.IsValidS3URI = true
	// for host style urls:
	//   group 0 is bucketname plus 's3' prefix and possible region code
	//   group 1 is bucket name
	//   group 2 will be region or 'amazonaws' if US Classic region
	// for path style urls:
	//   group 0 will be s3 prefix plus possible region code
	//   group 1 will be empty
	//   group 2 will be region or 'amazonaws' if US Classic region

	result := endpointRegEx.FindStringSubmatch(s3URL.Host)
	bucketNameGroup := ""
	if len(result) > 1 {
		bucketNameGroup = result[1]
	}
	path := s3URL.Path
	//log.Debugf("endpointRegEx.FindStringSubmatch =%v, path=%v" , result,path)
	if bucketNameGroup == "" {
		// no bucket name in the authority, parse it from the path
		output.IsPathStyle = true

		// grab the encoded path so we don't run afoul of '/'s in the bucket name
		if path == "/" {
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
		output.IsPathStyle = false
		output.Bucket = strings.TrimRight(bucketNameGroup, ".")
		if path == "/" {
			output.Key = ""
		} else {
			output.Key = path[1:]
		}
	}

	if len(result) > 2 {
		bucketNameGroup = result[1]
		regionGroupValue := result[2]
		if strings.EqualFold(regionGroupValue, "external-1") {
			output.Region = "us-east-1"
		} else if !strings.EqualFold(regionGroupValue, "amazonaws") {
			output.Region = regionGroupValue
		}
	}

	// s3 bucket URL in us-east-1 doesn't include region
	if output.Region == "" {
		output.Region = "us-east-1"
	}

	return
}

// String returns the string representation of the AmazonS3URL
func (output AmazonS3URL) String() string {
	return fmt.Sprintf("{Region: %s; Bucket: %s; Key: %s; IsValidS3URI: %v; IsPathStyle: %v}",
		output.Region, output.Bucket, output.Key, output.IsValidS3URI, output.IsPathStyle)
}
