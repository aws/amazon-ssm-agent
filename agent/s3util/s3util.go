// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Amazon Software License (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/asl/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package s3util contains methods for interracting with S3.
package s3util

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

const (
	accessDeniedErrMsg    = "AccessDenied: Access Denied status code: 403"
	diffRegionErrMsgRegex = "AuthorizationHeaderMalformed: The authorization header is malformed; the region '.+' is wrong; expecting '.+'"
)

// Manager is an object that can interact with s3.
type Manager struct {
	S3 *s3.S3
}

// NewManager creates a new Manager object.
func NewManager(s3 *s3.S3) *Manager {
	return &Manager{S3: s3}
}

// GetS3ClientRegion returns the S3 client's region
func (m Manager) GetS3ClientRegion() string {
	return *m.S3.Config.Region
}

// SetS3ClientRegion returns the S3 client's region
func (m *Manager) SetS3ClientRegion(region string) {
	*m.S3.Config.Region = region
}

// S3Upload uploads a file to s3.
func (m *Manager) S3Upload(bucketName string, objectKey string, filePath string) (err error) {
	file, err := os.Open(filePath)
	if err != nil {
		return
	}
	return m.S3UploadFromReader(bucketName, objectKey, file)
}

// S3UploadFromReader uploads data to s3 from an io.ReadSeeker.
func (m *Manager) S3UploadFromReader(bucketName string, objectKey string, content io.ReadSeeker) (err error) {
	params := &s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(objectKey),
		Body:        content,
		ContentType: aws.String("text/plain"),
	}
	_, err = m.S3.PutObject(params)
	return
}

// S3Download downloads an s3 object in memory.
func (m *Manager) S3Download(bucketName string, objectKey string) (data []byte, err error) {
	params := &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
	}
	resp, err := m.S3.GetObject(params)
	if err != nil {
		return
	}
	data, err = ioutil.ReadAll(resp.Body)
	return
}

// S3DeleteKey deletes an s3 object.
func (m *Manager) S3DeleteKey(bucketName string, objectKey string) (err error) {
	params := &s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
	}
	_, err = m.S3.DeleteObject(params)
	return
}

// UploadS3TestFile uploads a test S3 file (with current datetime) to given s3 bucket and key
func (m *Manager) UploadS3TestFile(log log.T, bucketName, key string) error {
	var err error
	//create a test content
	testData := time.Now().String()
	log.Debugf("Data being written in S3 - %v", testData)
	content := bytes.NewReader([]byte(testData))

	//objectName to be uploaded in S3
	var objectKey = path.Join(key, "ssmaccesstext.txt")

	params := &s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(objectKey),
		Body:        content,
		ContentType: aws.String("text/plain"),
	}
	_, err = m.S3.PutObject(params)

	return err
}

// GetS3BucketRegionFromErrorMsg gets the expected region from the error message
func (m *Manager) GetS3BucketRegionFromErrorMsg(log log.T, errMsg string) string {
	var expectedBucketRegion = ""
	if errMsg != "" && m.IsS3ErrorRelatedToWrongBucketRegion(errMsg) {
		//Sample error message:
		//AuthorizationHeaderMalformed: The authorization header is malformed; the region 'us-east-1' is wrong; expecting 'us-west-2' status code: 400, request id: []
		splitResult := strings.Split(errMsg, ";")
		furtherSplitResult := strings.Split(splitResult[len(splitResult)-1], "'")
		expectedBucketRegion = furtherSplitResult[1]
		log.Debugf("expected region according to error message = %v", expectedBucketRegion)

		if expectedBucketRegion == "" {
			log.Debugf("Setting expected region = us-east-1 as per http://docs.aws.amazon.com/AmazonS3/latest/API/RESTBucketGETlocation.html")
			expectedBucketRegion = "us-east-1"
		}
	}
	return expectedBucketRegion
}

// IsS3ErrorRelatedToAccessDenied determines if the given error message regarding AccessDenied
func (m *Manager) IsS3ErrorRelatedToAccessDenied(errMsg string) bool {
	return strings.Contains(errMsg, accessDeniedErrMsg)
}

// IsS3ErrorRelatedToWrongBucketRegion determines if the given error message is related to S3 bucket being in a different region than the one mentioned in S3client.
func (m *Manager) IsS3ErrorRelatedToWrongBucketRegion(errMsg string) bool {
	//If the bucket region is not correct then we get an error like following:
	//AuthorizationHeaderMalformed: The authorization header is malformed; the region 'us-east-1' is wrong; expecting 'us-west-2' status code: 400, request id: []
	match, _ := regexp.MatchString(diffRegionErrMsgRegex, errMsg)
	return match
}
