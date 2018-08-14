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

package s3util

import (
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/mock"
)

// Note: This code is used in the test files. However, this code is not in a _test.go file
// because then we would have to copy it in every test package that needs the mock.

// MockS3Uploader mocks an s3 uploader.
type MockS3Uploader struct {
	mock.Mock
}

var logger = log.NewMockLog()

// S3Upload mocks the method with the same name.
func (uploader *MockS3Uploader) S3Upload(log log.T, bucketName string, bucketKey string, contentPath string) error {
	args := uploader.Called(bucketName, bucketKey, contentPath)
	logger.Debugf("===========MockS3Upload Uploading %v to s3://%v/%v returns %v", contentPath, bucketName, bucketKey, args.Error(0))

	return args.Error(0)
}

// GetS3BucketRegionFromErrorMsg mocks the method with the same name.
func (uploader *MockS3Uploader) GetS3BucketRegionFromErrorMsg(log log.T, errMsg string) string {
	args := uploader.Called(log, errMsg)
	logger.Debugf("===========MockGetS3BucketRegionFromErrorMsg Getting S3 bucketRegion from error message - %v returns %v", errMsg, args.String(0))

	return args.String(0)
}

// IsS3ErrorRelatedToAccessDenied mocks the method with the same name.
func (uploader *MockS3Uploader) IsS3ErrorRelatedToAccessDenied(errMsg string) bool {
	args := uploader.Called(errMsg)
	logger.Debugf("===========MockIsS3ErrorRelatedToAccessDenied Determining if the given error message is because of accessdenied - %v returns %v", errMsg, args.Bool(0))

	return args.Bool(0)
}

// IsS3ErrorRelatedToWrongBucketRegion mocks the method with the same name.
func (uploader *MockS3Uploader) IsS3ErrorRelatedToWrongBucketRegion(errMsg string) bool {
	args := uploader.Called(errMsg)
	logger.Debugf("===========MockIsS3ErrorRelatedToWrongBucketRegion Determining if the given error message is because of wrong bucket region - %v returns %v", errMsg, args.Bool(0))

	return args.Bool(0)
}

// GetS3ClientRegion mocks the method with the same name.
func (uploader *MockS3Uploader) GetS3ClientRegion() string {
	args := uploader.Called()
	logger.Debugf("===========MockGetS3ClientRegion Getting S3 client region returns %v", args.String(0))

	return args.String(0)
}

// SetS3ClientRegion mocks the method with the same name.
func (uploader *MockS3Uploader) SetS3ClientRegion(region string) {
	//args := uploader.Called(region)
	logger.Debugf("===========MockGetS3ClientRegion Setting S3 client region to %v", region)
}

// UploadS3TestFile mocks the method with the same name.
func (uploader *MockS3Uploader) UploadS3TestFile(log log.T, bucketName, key string) error {
	args := uploader.Called(log, bucketName, key)
	logger.Debugf("===========MockUploadS3TestFile Uploading a test file to bucket - %v, key - %v returns %v", bucketName, key, args.Error(0))

	return args.Error(0)
}

// IsBucketEncrypted mocks the method with the same name.
func (uploader *MockS3Uploader) IsBucketEncrypted(log log.T, bucketName string) bool {
	args := uploader.Called(log, bucketName)
	logger.Debugf("===========MockIsBucketEncrypted Determining if the given s3 bucket has been encrypted - %v returns %v", bucketName, args.Bool(0))

	return args.Bool(0)
}
