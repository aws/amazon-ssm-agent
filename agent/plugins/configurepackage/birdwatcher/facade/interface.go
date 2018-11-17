// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

/* This interface is created manally based on the ssmiface.SSMAPI. In order to keep in one place the only APIs needed for birdwatcher. */
package facade

import (
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ssm"
)

// BirdwatcherFacade is the interface type for ssmiface.SSMAPI
type BirdwatcherFacade interface {
	GetManifestRequest(*ssm.GetManifestInput) (*request.Request, *ssm.GetManifestOutput)

	GetManifest(*ssm.GetManifestInput) (*ssm.GetManifestOutput, error)

	PutConfigurePackageResultRequest(*ssm.PutConfigurePackageResultInput) (*request.Request, *ssm.PutConfigurePackageResultOutput)

	PutConfigurePackageResult(*ssm.PutConfigurePackageResultInput) (*ssm.PutConfigurePackageResultOutput, error)

	GetDocumentRequest(*ssm.GetDocumentInput) (*request.Request, *ssm.GetDocumentOutput)

	GetDocument(*ssm.GetDocumentInput) (*ssm.GetDocumentOutput, error)

	DescribeDocumentRequest(*ssm.DescribeDocumentInput) (*request.Request, *ssm.DescribeDocumentOutput)

	DescribeDocument(*ssm.DescribeDocumentInput) (*ssm.DescribeDocumentOutput, error)
}

var _ BirdwatcherFacade = (*ssm.SSM)(nil)
