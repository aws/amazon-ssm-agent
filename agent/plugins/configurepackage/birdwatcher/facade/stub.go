// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

// This package returns the means of creating an object of type facade
package facade

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// FacadeStub
type FacadeStub struct {
	GetManifestInput  *ssm.GetManifestInput
	GetManifestOutput *ssm.GetManifestOutput
	GetManifestError  error

	PutConfigurePackageResultInput  *ssm.PutConfigurePackageResultInput
	PutConfigurePackageResultOutput *ssm.PutConfigurePackageResultOutput
	PutConfigurePackageResultError  error

	GetDocumentInput  *ssm.GetDocumentInput
	GetDocumentOutput *ssm.GetDocumentOutput
	GetDocumentError  error

	DescribeDocumentInput  *ssm.DescribeDocumentInput
	DescribeDocumentOutput *ssm.DescribeDocumentOutput
	DescribeDocumentError  error
}

func (m *FacadeStub) GetManifest(ctx context.Context, input *ssm.GetManifestInput, optFns ...func(*ssm.Options)) (*ssm.GetManifestOutput, error) {
	m.GetManifestInput = input
	return m.GetManifestOutput, m.GetManifestError
}

func (m *FacadeStub) PutConfigurePackageResult(ctx context.Context, input *ssm.PutConfigurePackageResultInput, optFns ...func(*ssm.Options)) (*ssm.PutConfigurePackageResultOutput, error) {
	m.PutConfigurePackageResultInput = input
	return m.PutConfigurePackageResultOutput, m.PutConfigurePackageResultError
}

func (m *FacadeStub) GetDocument(ctx context.Context, input *ssm.GetDocumentInput, optFns ...func(*ssm.Options)) (*ssm.GetDocumentOutput, error) {
	m.GetDocumentInput = input
	return m.GetDocumentOutput, m.GetDocumentError
}

func (m *FacadeStub) DescribeDocument(ctx context.Context, input *ssm.DescribeDocumentInput, optFns ...func(*ssm.Options)) (*ssm.DescribeDocumentOutput, error) {
	m.DescribeDocumentInput = input
	return m.DescribeDocumentOutput, m.DescribeDocumentError
}
