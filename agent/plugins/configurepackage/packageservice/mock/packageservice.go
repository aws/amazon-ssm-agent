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

package packageservice_mock

import (
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/packageservice"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/trace"
	"github.com/stretchr/testify/mock"
)

type Mock struct {
	mock.Mock
}

func (ds *Mock) PackageServiceName() string {
	args := ds.Called()
	return args.String(0)
}

func (ds *Mock) GetPackageArnAndVersion(packageName string, packageVersion string) (name string, version string) {
	args := ds.Called(packageName, packageVersion)
	return args.String(0), args.String(1)
}

func (ds *Mock) DownloadManifest(tracer trace.Tracer, packageName string, version string) (string, string, bool, error) {
	args := ds.Called(tracer, packageName, version)
	return args.String(0), args.String(1), args.Bool(2), args.Error(3)
}

func (ds *Mock) DownloadArtifact(tracer trace.Tracer, packageName string, version string) (string, error) {
	args := ds.Called(tracer, packageName, version)
	return args.String(0), args.Error(1)
}

func (ds *Mock) ReportResult(tracer trace.Tracer, result packageservice.PackageResult) error {
	args := ds.Called(tracer, result)
	return args.Error(0)
}
