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
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go/middleware"
)

func newS3Client(cfg aws.Config) *s3.Client {
	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.APIOptions = append(o.APIOptions, addContentMD5)
	})
}

// addContentMD5 sets Content-MD5 on PutObject and UploadPart requests that carry
// no other checksum, as aws-sdk-go v1 did for every upload. S3 rejects uploads to
// a bucket with a default Object Lock retention unless they carry Content-MD5 or
// an x-amz-checksum-* header.
func addContentMD5(stack *middleware.Stack) error {
	return stack.Initialize.Add(middleware.InitializeMiddlewareFunc("ContentMD5", setContentMD5), middleware.After)
}

func setContentMD5(ctx context.Context, in middleware.InitializeInput, next middleware.InitializeHandler) (middleware.InitializeOutput, middleware.Metadata, error) {
	var err error
	switch input := in.Parameters.(type) {
	case *s3.PutObjectInput:
		if input.ContentMD5 == nil && input.ChecksumAlgorithm == "" {
			input.ContentMD5, err = bodyMD5(input.Body)
		}
	case *s3.UploadPartInput:
		if input.ContentMD5 == nil && input.ChecksumAlgorithm == "" {
			input.ContentMD5, err = bodyMD5(input.Body)
		}
	}
	if err != nil {
		return middleware.InitializeOutput{}, middleware.Metadata{}, fmt.Errorf("compute Content-MD5: %w", err)
	}
	return next.HandleInitialize(ctx, in)
}

// bodyMD5 returns the base64 MD5 of a seekable body from its current position,
// leaving it at that position. It returns nil for a body it can't rewind.
func bodyMD5(body io.Reader) (*string, error) {
	seeker, ok := body.(io.ReadSeeker)
	if !ok {
		return nil, nil
	}
	start, err := seeker.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, err
	}
	hash := md5.New()
	if _, err := io.Copy(hash, seeker); err != nil {
		return nil, err
	}
	if _, err := seeker.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}
	return aws.String(base64.StdEncoding.EncodeToString(hash.Sum(nil))), nil
}
