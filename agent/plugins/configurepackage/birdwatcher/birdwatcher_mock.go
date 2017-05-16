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
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/birdwatcherstationservice"
)

// birdwatcherStationServiceMock
type birdwatcherStationServiceMock struct {
	getManifestInput  *birdwatcherstationservice.GetManifestInput
	getManifestOutput *birdwatcherstationservice.GetManifestOutput
	getManifestError  error

	putConfigurePackageResultInput  *birdwatcherstationservice.PutConfigurePackageResultInput
	putConfigurePackageResultOutput *birdwatcherstationservice.PutConfigurePackageResultOutput
	putConfigurePackageResultError  error
}

func (*birdwatcherStationServiceMock) GetManifestRequest(*birdwatcherstationservice.GetManifestInput) (*request.Request, *birdwatcherstationservice.GetManifestOutput) {
	panic("not implemented")
}

func (m *birdwatcherStationServiceMock) GetManifest(input *birdwatcherstationservice.GetManifestInput) (*birdwatcherstationservice.GetManifestOutput, error) {
	m.getManifestInput = input
	return m.getManifestOutput, m.getManifestError
}

func (*birdwatcherStationServiceMock) PutConfigurePackageResultRequest(*birdwatcherstationservice.PutConfigurePackageResultInput) (*request.Request, *birdwatcherstationservice.PutConfigurePackageResultOutput) {
	panic("not implemented")
}

func (m *birdwatcherStationServiceMock) PutConfigurePackageResult(input *birdwatcherstationservice.PutConfigurePackageResultInput) (*birdwatcherstationservice.PutConfigurePackageResultOutput, error) {
	m.putConfigurePackageResultInput = input
	return m.putConfigurePackageResultOutput, m.putConfigurePackageResultError
}

// platformProviderMock
type platformProviderMock struct {
	name                string
	nameerr             error
	version             string
	versionerr          error
	architecture        string
	architectureerr     error
	instanceID          string
	instaceIDerr        error
	instanceType        string
	instanceTypeerr     error
	availabilityZone    string
	availabilityZoneerr error
	region              string
	regionerr           error
}

func (p *platformProviderMock) Name(log log.T) (string, error) {
	return p.name, p.nameerr
}

func (p *platformProviderMock) Version(log log.T) (string, error) {
	return p.version, p.versionerr
}

func (p *platformProviderMock) Architecture(log log.T) (string, error) {
	return p.architecture, p.architectureerr
}

func (p *platformProviderMock) InstanceID(log log.T) (string, error) {
	return p.instanceID, p.instaceIDerr
}

func (p *platformProviderMock) InstanceType(log log.T) (string, error) {
	return p.instanceType, p.instanceTypeerr
}

func (p *platformProviderMock) AvailabilityZone(log log.T) (string, error) {
	return p.availabilityZone, p.availabilityZoneerr
}

func (p *platformProviderMock) Region(log log.T) (string, error) {
	return p.region, p.regionerr
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
