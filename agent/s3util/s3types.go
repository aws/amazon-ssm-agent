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

package s3util

import (
	"github.com/aws/aws-sdk-go/service/s3"
)

// All of the *Input types have getBucket() functions, but sadly these are not
// exported. The SDK defines a bucketGetter interface, but this isn't exported either.
func getBucketFromParams(params interface{}) string {
	var result *string = nil
	switch p := params.(type) {
	case *s3.AbortMultipartUploadInput:
		result = p.Bucket
	case *s3.CompleteMultipartUploadInput:
		result = p.Bucket
	case *s3.CopyObjectInput:
		result = p.Bucket
	case *s3.CreateBucketInput:
		result = p.Bucket
	case *s3.CreateMultipartUploadInput:
		result = p.Bucket
	case *s3.DeleteBucketAnalyticsConfigurationInput:
		result = p.Bucket
	case *s3.DeleteBucketCorsInput:
		result = p.Bucket
	case *s3.DeleteBucketEncryptionInput:
		result = p.Bucket
	case *s3.DeleteBucketInput:
		result = p.Bucket
	case *s3.DeleteBucketInventoryConfigurationInput:
		result = p.Bucket
	case *s3.DeleteBucketLifecycleInput:
		result = p.Bucket
	case *s3.DeleteBucketMetricsConfigurationInput:
		result = p.Bucket
	case *s3.DeleteBucketPolicyInput:
		result = p.Bucket
	case *s3.DeleteBucketReplicationInput:
		result = p.Bucket
	case *s3.DeleteBucketTaggingInput:
		result = p.Bucket
	case *s3.DeleteBucketWebsiteInput:
		result = p.Bucket
	case *s3.DeleteObjectInput:
		result = p.Bucket
	case *s3.DeleteObjectTaggingInput:
		result = p.Bucket
	case *s3.DeleteObjectsInput:
		result = p.Bucket
	case *s3.DeletePublicAccessBlockInput:
		result = p.Bucket
	case *s3.GetBucketAccelerateConfigurationInput:
		result = p.Bucket
	case *s3.GetBucketAclInput:
		result = p.Bucket
	case *s3.GetBucketAnalyticsConfigurationInput:
		result = p.Bucket
	case *s3.GetBucketCorsInput:
		result = p.Bucket
	case *s3.GetBucketEncryptionInput:
		result = p.Bucket
	case *s3.GetBucketInventoryConfigurationInput:
		result = p.Bucket
	case *s3.GetBucketLifecycleConfigurationInput:
		result = p.Bucket
	case *s3.GetBucketLifecycleInput:
		result = p.Bucket
	case *s3.GetBucketLocationInput:
		result = p.Bucket
	case *s3.GetBucketLoggingInput:
		result = p.Bucket
	case *s3.GetBucketMetricsConfigurationInput:
		result = p.Bucket
	case *s3.GetBucketPolicyInput:
		result = p.Bucket
	case *s3.GetBucketPolicyStatusInput:
		result = p.Bucket
	case *s3.GetBucketReplicationInput:
		result = p.Bucket
	case *s3.GetBucketRequestPaymentInput:
		result = p.Bucket
	case *s3.GetBucketTaggingInput:
		result = p.Bucket
	case *s3.GetBucketVersioningInput:
		result = p.Bucket
	case *s3.GetBucketWebsiteInput:
		result = p.Bucket
	case *s3.GetObjectAclInput:
		result = p.Bucket
	case *s3.GetObjectInput:
		result = p.Bucket
	case *s3.GetObjectLegalHoldInput:
		result = p.Bucket
	case *s3.GetObjectLockConfigurationInput:
		result = p.Bucket
	case *s3.GetObjectRetentionInput:
		result = p.Bucket
	case *s3.GetObjectTaggingInput:
		result = p.Bucket
	case *s3.GetObjectTorrentInput:
		result = p.Bucket
	case *s3.GetPublicAccessBlockInput:
		result = p.Bucket
	case *s3.HeadBucketInput:
		result = p.Bucket
	case *s3.HeadObjectInput:
		result = p.Bucket
	case *s3.ListBucketAnalyticsConfigurationsInput:
		result = p.Bucket
	case *s3.ListBucketInventoryConfigurationsInput:
		result = p.Bucket
	case *s3.ListBucketMetricsConfigurationsInput:
		result = p.Bucket
	case *s3.ListMultipartUploadsInput:
		result = p.Bucket
	case *s3.ListObjectVersionsInput:
		result = p.Bucket
	case *s3.ListObjectsInput:
		result = p.Bucket
	case *s3.ListObjectsV2Input:
		result = p.Bucket
	case *s3.ListPartsInput:
		result = p.Bucket
	case *s3.PutBucketAccelerateConfigurationInput:
		result = p.Bucket
	case *s3.PutBucketAclInput:
		result = p.Bucket
	case *s3.PutBucketAnalyticsConfigurationInput:
		result = p.Bucket
	case *s3.PutBucketCorsInput:
		result = p.Bucket
	case *s3.PutBucketEncryptionInput:
		result = p.Bucket
	case *s3.PutBucketInventoryConfigurationInput:
		result = p.Bucket
	case *s3.PutBucketLifecycleConfigurationInput:
		result = p.Bucket
	case *s3.PutBucketLifecycleInput:
		result = p.Bucket
	case *s3.PutBucketLoggingInput:
		result = p.Bucket
	case *s3.PutBucketMetricsConfigurationInput:
		result = p.Bucket
	case *s3.PutBucketNotificationConfigurationInput:
		result = p.Bucket
	case *s3.PutBucketNotificationInput:
		result = p.Bucket
	case *s3.PutBucketPolicyInput:
		result = p.Bucket
	case *s3.PutBucketReplicationInput:
		result = p.Bucket
	case *s3.PutBucketRequestPaymentInput:
		result = p.Bucket
	case *s3.PutBucketTaggingInput:
		result = p.Bucket
	case *s3.PutBucketVersioningInput:
		result = p.Bucket
	case *s3.PutBucketWebsiteInput:
		result = p.Bucket
	case *s3.PutObjectAclInput:
		result = p.Bucket
	case *s3.PutObjectInput:
		result = p.Bucket
	case *s3.PutObjectLegalHoldInput:
		result = p.Bucket
	case *s3.PutObjectLockConfigurationInput:
		result = p.Bucket
	case *s3.PutObjectRetentionInput:
		result = p.Bucket
	case *s3.PutObjectTaggingInput:
		result = p.Bucket
	case *s3.PutPublicAccessBlockInput:
		result = p.Bucket
	case *s3.RestoreObjectInput:
		result = p.Bucket
	case *s3.SelectObjectContentInput:
		result = p.Bucket
	case *s3.UploadPartCopyInput:
		result = p.Bucket
	case *s3.UploadPartInput:
		result = p.Bucket
	default:
		result = nil
	}

	if result != nil {
		return *result
	} else {
		return ""
	}
}
