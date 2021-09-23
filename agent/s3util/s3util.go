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
	"os"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/backoffconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/cenkalti/backoff/v4"
)

var makeAwsConfig = sdkutil.AwsConfigForRegion
var getS3Endpoint = GetS3Endpoint
var getFallbackS3EndpointFunc = getFallbackS3Endpoint
var backoffRetry = backoff.Retry

var getHttpProvider = func(logger log.T, appConfig appconfig.SsmagentConfig) HttpProvider {
	return HttpProviderImpl{
		logger:    logger,
		appConfig: appConfig,
	}
}

type IAmazonS3Util interface {
	S3Upload(log log.T, bucketName string, objectKey string, filePath string) error
	IsBucketEncrypted(log log.T, bucketName string) (bool, error)
}

type AmazonS3Util struct {
	myUploader *s3manager.Uploader
}

func shouldRetryS3Upload(err error) bool {
	// Don't retry if no error
	if err == nil {
		return false
	}

	if awsErr, ok := err.(awserr.Error); ok {
		code := awsErr.Code()
		if _, ok := awsErr.(s3manager.MultiUploadFailure); ok {
			return true
		} else if code == "ChecksumValidationError" || code == "InvalidChecksum" || code == "ReadRequestBody" || code == "BodyHashError" || code == "SerializationError" || code == "ReadError" || code == "ResponseTimeout" || code == "InternalError" || code == "SlowDown" {
			return true
		}
		return false
	}

	// Retry for any non-aws errors
	return true
}

func NewAmazonS3Util(context context.T, bucketName string) (res *AmazonS3Util, err error) {
	log := context.Log()
	sess, err := GetS3CrossRegionCapableSession(context, bucketName)
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

	if bucketEncrypted, sseAlgortihm, encryptionKey := getSSEAlgorithm(log, u, bucketName); bucketEncrypted == true {
		switch sseAlgortihm {
		case s3.ServerSideEncryptionAes256:
			params.ServerSideEncryption = aws.String(sseAlgortihm)
		case s3.ServerSideEncryptionAwsKms:
			params.ServerSideEncryption = aws.String(sseAlgortihm)
			if encryptionKey != "" {
				params.SSEKMSKeyId = aws.String(encryptionKey)
			}
		}
	}

	exponentialBackoff, err := backoffconfig.GetDefaultExponentialBackoff()
	if err != nil {
		log.Warnf("Failed to create backoff config with error: %v", err)
		return err
	}

	var result *s3manager.UploadOutput
	_ = backoffRetry(func() error {
		result, err = u.myUploader.Upload(params)
		if shouldRetryS3Upload(err) {
			log.Warnf("Failed uploading %v to s3://%v/%v err:%v - retrying", filePath, bucketName, objectKey, err)
			return err
		}
		return nil
	}, exponentialBackoff)

	if err != nil {
		log.Errorf("Failed to upload %v to s3://%v/%v err:%v", filePath, bucketName, objectKey, err)
		return err
	}
	log.Infof("Successfully uploaded file to %s", result.Location)

	return nil
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

func getSSEAlgorithm(log log.T, u *AmazonS3Util, bucketName string) (bucketEncrypted bool, sseAlgortihm string, encryptionKey string) {
	defer func() {
		if msg := recover(); msg != nil {
			log.Errorf("S3Upload panic: %v", msg)
		}
	}()

	input := &s3.GetBucketEncryptionInput{
		Bucket: aws.String(bucketName),
	}

	output, err := u.myUploader.S3.GetBucketEncryption(input)
	if err != nil {
		log.Infof("Bucket is not encrypted")
		return false, "", ""
	}

	bucketEncryption := *output.ServerSideEncryptionConfiguration.Rules[0].ApplyServerSideEncryptionByDefault
	switch bucketEncryptionType := *bucketEncryption.SSEAlgorithm; bucketEncryptionType {

	case s3.ServerSideEncryptionAwsKms:
		// If bucket is KMS encrypted
		log.Infof("Bucket %v has been encrypted with KMS", bucketName)
		if bucketEncryption.KMSMasterKeyID != nil {
			return true, s3.ServerSideEncryptionAwsKms, *bucketEncryption.KMSMasterKeyID
		} else {
			return true, s3.ServerSideEncryptionAwsKms, ""
		}

	case s3.ServerSideEncryptionAes256:
		// If bucket is Aes256 encrypted
		log.Infof("Bucket %v has been encrypted with AES256", bucketName)
		return true, s3.ServerSideEncryptionAes256, ""
	default:
		log.Infof("Bucket is not encrypted")
		return false, "", ""
	}
}
