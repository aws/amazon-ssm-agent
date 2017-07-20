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
	"os"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

const (
	s3ResponseRegionHeader = "x-amz-bucket-region"
)

var getRegion = platform.Region

type IAmazonS3Util interface {
	S3Upload(log log.T, bucketName string, objectKey string, filePath string) error
	GetBucketRegion(log log.T, bucketName string) string
	GetS3Header(log log.T, bucketName string, instanceRegion string) string
}

type AmazonS3Util struct {
	myUploader *s3manager.Uploader
}

func NewAmazonS3Util(log log.T, bucketName string) *AmazonS3Util {
	bucketRegion := GetBucketRegion(log, bucketName)

	config := sdkutil.AwsConfig()
	var appConfig appconfig.SsmagentConfig
	appConfig, errConfig := appconfig.Config(false)
	if errConfig != nil {
		log.Error("failed to read appconfig.")
	} else {
		if appConfig.S3.Endpoint != "" {
			config.Endpoint = &appConfig.S3.Endpoint
		}
	}
	config.Region = &bucketRegion

	return &AmazonS3Util{
		myUploader: s3manager.NewUploader(session.New(config)),
	}
}

// S3Upload uploads a file to s3.
func (u *AmazonS3Util) S3Upload(log log.T, bucketName string, objectKey string, filePath string) (err error) {
	file, err := os.Open(filePath)
	if err != nil {
		log.Errorf("Failed to open file %v", err)
		return err
	}
	defer file.Close()

	log.Infof("Uploading %v to s3://%v/%v", filePath, bucketName, objectKey)
	params := &s3manager.UploadInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(objectKey),
		Body:        file,
		ContentType: aws.String("text/plain"),
	}
	if result, err := u.myUploader.Upload(params); err == nil {
		log.Infof("Successfully uploaded file to ", result.Location)
		if _, aclErr := u.myUploader.S3.PutObjectAcl(&s3.PutObjectAclInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(objectKey),
			ACL:    aws.String("bucket-owner-full-control"),
		}); aclErr == nil {
			log.Infof("PutAcl: bucket-owner-full-control succeeded.")
		} else {
			// gracefully ignore the error, since the S3 putAcl policy may not be set
			log.Debugf("PutAcl: bucket-owner-full-control failed, error: %v", aclErr)
		}
	} else {
		log.Errorf("Failed uploading %v to s3://%v/%v err:%v", filePath, bucketName, objectKey, err)
	}
	return err
}

// This function returns the Amazon S3 Bucket region based on its name and the EC2 instance region.
// It will return the same instance region if it failed to guess the bucket region.
func GetBucketRegion(log log.T, bucketName string) (region string) {
	instanceRegion, err := getRegion()
	if err != nil {
		log.Error("Cannot get the current instance region information")
		return instanceRegion // Default
	}
	log.Infof("Instance region is %v", instanceRegion)

	bucketRegion := GetS3Header(log, bucketName, instanceRegion)
	if bucketRegion == "" {
		return instanceRegion // Default
	} else {
		return bucketRegion
	}
}

/*
This function return the S3 bucket region that is returned as a result of a CURL operation on an S3 path.
The starting endpoint does not need to be the same as the returned region.
For example, we might query an endpoint in us-east-1 and get a return of us-west-2, if the bucket is actually in us-west-2.
*/
func GetS3Header(log log.T, bucketName string, instanceRegion string) (region string) {
	s3Endpoint := GetS3Endpoint(instanceRegion)
	resp, err := http.Head("http://" + bucketName + "." + s3Endpoint)
	if err == nil {
		return resp.Header.Get(s3ResponseRegionHeader)
	}
	// Fail over to the generic regional end point, if different from the regional end point
	genericEndPoint := GetS3GenericEndPoint(instanceRegion)
	if genericEndPoint != s3Endpoint {
		log.Infof("Error when querying S3 bucket using address http://%v.%v. Error details: %v. Retrying with the generic regional endpoint %v...",
			bucketName, s3Endpoint, err, genericEndPoint)
		resp, err = http.Head("http://" + bucketName + "." + genericEndPoint)
		if err == nil {
			return resp.Header.Get(s3ResponseRegionHeader)
		}
	}
	// Could not query the bucket region. Log the error.
	log.Infof("Error when querying S3 bucket using address http://%v.%v. Error details: %v",
		bucketName, genericEndPoint, err)
	return ""
}
