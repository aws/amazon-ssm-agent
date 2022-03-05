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
	"math"
	"os"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

var getRegion = platform.Region
var makeAwsConfig = sdkutil.AwsConfigForRegion
var getAppConfig = func() (appconfig.SsmagentConfig, error) {
	return appconfig.Config(false)
}
var getS3Endpoint = GetS3Endpoint
var getFallbackS3EndpointFunc = getFallbackS3Endpoint
var getHttpProvider = func(logger log.T) HttpProvider {
	return HttpProviderImpl{
		logger: logger,
	}
}

type IAmazonS3Util interface {
	S3Upload(log log.T, bucketName string, objectKey string, filePath string) error
	IsBucketEncrypted(log log.T, bucketName string) (bool, error)
}

type AmazonS3Util struct {
	myUploader *s3manager.Uploader
}

func NewAmazonS3Util(log log.T, bucketName string) (res *AmazonS3Util, err error) {
	sess, err := GetS3CrossRegionCapableSession(log, bucketName)
	if err == nil {
		res = &AmazonS3Util{
			myUploader: s3manager.NewUploader(sess),
		}
	} else {
		log.Errorf("failed to create AmazonS3Util: %v", err)
	}
	return
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
		ACL:         aws.String("bucket-owner-full-control"),
	}

	for attempt := 1; attempt <= 4; attempt++ {
		var result *s3manager.UploadOutput
		if result, err = u.myUploader.Upload(params); err == nil {
			log.Infof("Successfully uploaded file to ", result.Location)
			break
		} else {
			log.Errorf("Attempt %s: Failed uploading %v to s3://%v/%v err:%v ", attempt, filePath, bucketName, objectKey, err)
			time.Sleep(time.Duration(math.Pow(2, float64(attempt))*100) * time.Millisecond)
		}
	}

	return err
}

// IsBucketEncrypted checks if the bucket is encrypted
func (u *AmazonS3Util) IsBucketEncrypted(log log.T, bucketName string) (bool, error) {
	input := &s3.GetBucketEncryptionInput{
		Bucket: aws.String(bucketName),
	}

	output, err := u.myUploader.S3.GetBucketEncryption(input)
	if err != nil {
		log.Errorf("Encountered an error while calling S3 API GetBucketEncryption %s", err)
		return false, err
	}

	bucketEncryption := *output.ServerSideEncryptionConfiguration.Rules[0].ApplyServerSideEncryptionByDefault.SSEAlgorithm

	if bucketEncryption == s3.ServerSideEncryptionAwsKms || bucketEncryption == s3.ServerSideEncryptionAes256 {
		return true, nil
	}

	log.Errorf("S3 bucket %s is not encrypted", bucketName)
	return false, nil
}
