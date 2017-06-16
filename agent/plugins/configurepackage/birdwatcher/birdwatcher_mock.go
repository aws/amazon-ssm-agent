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

package birdwatcher

import (
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/envdetect"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/stretchr/testify/mock"
)

// facadeMock
type facadeMock struct {
	getManifestInput  *ssm.GetManifestInput
	getManifestOutput *ssm.GetManifestOutput
	getManifestError  error

	putConfigurePackageResultInput  *ssm.PutConfigurePackageResultInput
	putConfigurePackageResultOutput *ssm.PutConfigurePackageResultOutput
	putConfigurePackageResultError  error
}

func (*facadeMock) GetManifestRequest(*ssm.GetManifestInput) (*request.Request, *ssm.GetManifestOutput) {
	panic("not implemented")
}

func (m *facadeMock) GetManifest(input *ssm.GetManifestInput) (*ssm.GetManifestOutput, error) {
	m.getManifestInput = input
	return m.getManifestOutput, m.getManifestError
}

func (*facadeMock) PutConfigurePackageResultRequest(*ssm.PutConfigurePackageResultInput) (*request.Request, *ssm.PutConfigurePackageResultOutput) {
	panic("not implemented")
}

func (m *facadeMock) PutConfigurePackageResult(input *ssm.PutConfigurePackageResultInput) (*ssm.PutConfigurePackageResultOutput, error) {
	m.putConfigurePackageResultInput = input
	return m.putConfigurePackageResultOutput, m.putConfigurePackageResultError
}

// networkMock
type networkMock struct {
	downloadInput  artifact.DownloadInput
	downloadOutput artifact.DownloadOutput
	downloadError  error
}

func (p *networkMock) Download(log log.T, input artifact.DownloadInput) (artifact.DownloadOutput, error) {
	p.downloadInput = input
	return p.downloadOutput, p.downloadError
}

type CollectorMock struct {
	mock.Mock
}

func (cd *CollectorMock) CollectData(log log.T) (*envdetect.Environment, error) {
	args := cd.Called(log)
	return args.Get(0).(*envdetect.Environment), args.Error(1)
}
