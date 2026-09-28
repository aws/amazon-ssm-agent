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
	cont "context"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// BirdwatcherFacade is the interface type for ssmiface.SSMAPI
type BirdwatcherFacade interface {
	GetManifest(ctx cont.Context, params *ssm.GetManifestInput, optFns ...func(*ssm.Options)) (*ssm.GetManifestOutput, error)

	PutConfigurePackageResult(ctx cont.Context, params *ssm.PutConfigurePackageResultInput, optFns ...func(*ssm.Options)) (*ssm.PutConfigurePackageResultOutput, error)

	GetDocument(ctx cont.Context, params *ssm.GetDocumentInput, optFns ...func(*ssm.Options)) (*ssm.GetDocumentOutput, error)

	DescribeDocument(ctx cont.Context, params *ssm.DescribeDocumentInput, optFns ...func(*ssm.Options)) (*ssm.DescribeDocumentOutput, error)
}

var _ BirdwatcherFacade = (*ssm.Client)(nil)
