// Copyright 2026 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	s3manager "github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type uploadRequest struct {
	operation  string
	contentMD5 string
	bodyMD5    string
	trailer    string
}

// stubS3 answers just enough of PutObject and the multipart upload API for
// the upload manager, recording each upload request's Content-MD5 header.
type stubS3 struct {
	mu       sync.Mutex
	requests []uploadRequest
}

func (s *stubS3) Do(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
	}
	sum := md5.Sum(body)
	query := req.URL.Query()

	operation, responseBody, header := "PutObject", "", http.Header{}
	switch {
	case req.Method == http.MethodPost && query.Has("uploads"):
		operation = "CreateMultipartUpload"
		responseBody = `<InitiateMultipartUploadResult><Bucket>bucket</Bucket><Key>key</Key><UploadId>upload-1</UploadId></InitiateMultipartUploadResult>`
	case req.Method == http.MethodPost && query.Has("uploadId"):
		operation = "CompleteMultipartUpload"
		responseBody = `<CompleteMultipartUploadResult><Bucket>bucket</Bucket><Key>key</Key><ETag>"etag"</ETag></CompleteMultipartUploadResult>`
	case req.Method == http.MethodPut && query.Has("partNumber"):
		operation = "UploadPart"
		header.Set("ETag", `"etag"`)
	}

	s.mu.Lock()
	s.requests = append(s.requests, uploadRequest{
		operation:  operation,
		contentMD5: req.Header.Get("Content-MD5"),
		bodyMD5:    base64.StdEncoding.EncodeToString(sum[:]),
		trailer:    req.Header.Get("X-Amz-Trailer"),
	})
	s.mu.Unlock()

	return &http.Response{StatusCode: http.StatusOK, Header: header, Body: io.NopCloser(strings.NewReader(responseBody))}, nil
}

func uploadFile(t *testing.T, size int) []uploadRequest {
	path := filepath.Join(t.TempDir(), "session.log")
	require.NoError(t, os.WriteFile(path, bytes.Repeat([]byte("a"), size), 0600))
	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()

	stub := &stubS3{}
	cfg := aws.Config{
		Region:       "us-east-1",
		Credentials:  credentials.NewStaticCredentialsProvider("AKID", "SECRET", ""),
		HTTPClient:   stub,
		BaseEndpoint: aws.String("https://s3.us-east-1.amazonaws.com"),
	}

	_, err = s3manager.NewUploader(newS3Client(cfg)).Upload(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String("bucket"),
		Key:    aws.String("key"),
		Body:   file,
	})
	require.NoError(t, err)
	return stub.requests
}

func uploadsOf(requests []uploadRequest, operation string) []uploadRequest {
	var matching []uploadRequest
	for _, r := range requests {
		if r.operation == operation {
			matching = append(matching, r)
		}
	}
	return matching
}

// S3 rejects uploads without Content-MD5 or an x-amz-checksum-* header on
// buckets with a default Object Lock retention.
func TestNewS3Client_SinglePartUploadSendsContentMD5(t *testing.T) {
	uploads := uploadsOf(uploadFile(t, 1024), "PutObject")

	require.Len(t, uploads, 1)
	assert.Equal(t, uploads[0].bodyMD5, uploads[0].contentMD5)
}

// The upload manager sends a CRC32 trailing checksum on each part, so parts get
// no Content-MD5 on top of it.
func TestNewS3Client_MultipartUploadKeepsManagerChecksum(t *testing.T) {
	uploads := uploadsOf(uploadFile(t, int(s3manager.DefaultUploadPartSize)+1024), "UploadPart")

	require.Len(t, uploads, 2)
	for _, part := range uploads {
		assert.Equal(t, "x-amz-checksum-crc32", strings.ToLower(part.trailer))
		assert.Empty(t, part.contentMD5)
	}
}
