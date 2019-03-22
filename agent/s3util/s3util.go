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
	"errors"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
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
	IsBucketEncrypted(log log.T, bucketName string) bool
}

type AmazonS3Util struct {
	myUploader *s3manager.Uploader
}

func NewAmazonS3Util(log log.T, bucketName string) *AmazonS3Util {

	httpProvider := HttpProviderImpl{}
	bucketRegion := GetBucketRegion(log, bucketName, httpProvider)

	config := sdkutil.AwsConfig()
	var appConfig appconfig.SsmagentConfig
	appConfig, errConfig := appconfig.Config(false)
	if errConfig != nil {
		log.Error("failed to read appconfig.")
	} else {
		if appConfig.S3.Endpoint != "" {
			config.Endpoint = &appConfig.S3.Endpoint
		} else {
			if region, err := platform.Region(); err == nil {
				if defaultEndpoint := appconfig.GetDefaultEndPoint(region, "s3"); defaultEndpoint != "" {
					config.Endpoint = &defaultEndpoint
				}
			} else {
				log.Errorf("error fetching the region, %v", err)
			}
		}
	}
	config.Region = &bucketRegion

	sess := session.New(config)
	sess.Handlers.Build.PushBack(request.MakeAddToUserAgentHandler(appConfig.Agent.Name, appConfig.Agent.Version))

	return &AmazonS3Util{
		myUploader: s3manager.NewUploader(sess),
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
func GetBucketRegion(log log.T, bucketName string, httpProvider HttpProvider) (region string) {
	instanceRegion, err := getRegion()
	if err != nil {
		log.Error("Cannot get the current instance region information")
		return instanceRegion // Default
	}
	log.Infof("Instance region is %v", instanceRegion)

	bucketRegion := GetS3Header(log, bucketName, instanceRegion, httpProvider)
	if bucketRegion == "" {
		return instanceRegion // Default
	} else {
		return bucketRegion
	}
}

//IsBucketEncrypted checks if the bucket is encrypted
func (u *AmazonS3Util) IsBucketEncrypted(log log.T, bucketName string) bool {
	input := &s3.GetBucketEncryptionInput{
		Bucket: aws.String(bucketName),
	}

	output, err := u.myUploader.S3.GetBucketEncryption(input)
	if err != nil {
		log.Errorf("Encountered an error while calling S3 API GetBucketEncryption %s", err)
		return false
	}

	bucketEncryption := *output.ServerSideEncryptionConfiguration.Rules[0].ApplyServerSideEncryptionByDefault.SSEAlgorithm

	if bucketEncryption == s3.ServerSideEncryptionAwsKms || bucketEncryption == s3.ServerSideEncryptionAes256 {
		return true
	}

	log.Errorf("S3 bucket %s is not encrypted", bucketName)
	return false
}

/*
This function return the S3 bucket region that is returned as a result of a CURL operation on an S3 path.
The starting endpoint does not need to be the same as the returned region.
For example, we might query an endpoint in us-east-1 and get a return of us-west-2, if the bucket is actually in us-west-2.
*/
func GetS3Header(log log.T, bucketName string, instanceRegion string, httpProvider HttpProvider) string {
	var err error
	var region string

	s3Endpoint := GetS3Endpoint(instanceRegion)
	if region, err = getRegionFromS3URLWithExponentialBackoff("http://"+bucketName+"."+s3Endpoint, httpProvider); err == nil {
		return region
	}
	// Try to get the s3 region with https protocol, it's required for S3 upload when https-only enabled
	if region, err = getRegionFromS3URLWithExponentialBackoff("https://"+bucketName+"."+s3Endpoint, httpProvider); err == nil {
		return region
	}

	// Fail over to the generic regional end point, if different from the regional end point
	genericEndPoint := GetS3GenericEndPoint(instanceRegion)
	if genericEndPoint != s3Endpoint {
		log.Infof("Error when querying S3 bucket using address http://%v.%v. Error details: %v. Retrying with the generic regional endpoint %v...",
			bucketName, s3Endpoint, err, genericEndPoint)
		if region, err = getRegionFromS3URLWithExponentialBackoff("http://"+bucketName+"."+genericEndPoint, httpProvider); err == nil {
			return region
		}
		if region, err = getRegionFromS3URLWithExponentialBackoff("https://"+bucketName+"."+genericEndPoint, httpProvider); err == nil {
			return region
		}
	}

	// Could not query the bucket region. Log the error.
	log.Infof("Error when querying S3 bucket using address http://%v.%v. Error details: %v",
		bucketName, genericEndPoint, err)
	return ""
}

func getRegionFromS3URLWithExponentialBackoff(url string, httpProvider HttpProvider) (region string, err error) {
	// Sleep with exponential backoff strategy if response had unexpected error, 502, 503 or 504 http code
	// For any other failed cases, we try it without exponential back off.
	for retryCount := 1; retryCount <= 5; retryCount++ {
		resp, err := httpProvider.Head(url)
		if err != nil || resp == nil {
			continue
		}

		if resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504 {
			time.Sleep(time.Duration(math.Pow(2, float64(retryCount))*100) * time.Millisecond)
		} else if region = resp.Header.Get(s3ResponseRegionHeader); region != "" {
			// Region is fetched correctly at this point
			return region, nil
		}
	}

	err = errors.New(fmt.Sprintf("Failed to fetch region from the header - %s", err))

	return
}
