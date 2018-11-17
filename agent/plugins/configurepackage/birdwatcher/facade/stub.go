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
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ssm"
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

func (m *FacadeStub) GetManifestRequest(*ssm.GetManifestInput) (*request.Request, *ssm.GetManifestOutput) {
	panic("not implemented")
}

func (m *FacadeStub) GetManifest(input *ssm.GetManifestInput) (*ssm.GetManifestOutput, error) {
	m.GetManifestInput = input
	return m.GetManifestOutput, m.GetManifestError
}

func (m *FacadeStub) PutConfigurePackageResultRequest(*ssm.PutConfigurePackageResultInput) (*request.Request, *ssm.PutConfigurePackageResultOutput) {
	panic("not implemented")
}

func (m *FacadeStub) PutConfigurePackageResult(input *ssm.PutConfigurePackageResultInput) (*ssm.PutConfigurePackageResultOutput, error) {
	m.PutConfigurePackageResultInput = input
	return m.PutConfigurePackageResultOutput, m.PutConfigurePackageResultError
}

func (m *FacadeStub) GetDocumentRequest(*ssm.GetDocumentInput) (*request.Request, *ssm.GetDocumentOutput) {
	panic("not implemented")
}

func (m *FacadeStub) GetDocument(input *ssm.GetDocumentInput) (*ssm.GetDocumentOutput, error) {
	m.GetDocumentInput = input
	return m.GetDocumentOutput, m.GetDocumentError
}

func (m *FacadeStub) DescribeDocumentRequest(*ssm.DescribeDocumentInput) (*request.Request, *ssm.DescribeDocumentOutput) {
	panic("not implemented")
}

func (m *FacadeStub) DescribeDocument(input *ssm.DescribeDocumentInput) (*ssm.DescribeDocumentOutput, error) {
	m.DescribeDocumentInput = input
	return m.DescribeDocumentOutput, m.DescribeDocumentError
}
