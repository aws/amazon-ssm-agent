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

// Package archive has the interface for the various archives used by birdwatcher service
package archive

import (
	"github.com/stretchr/testify/mock"
)

type Mock struct {
	mock.Mock
}

func (pa Mock) GetResourceVersion(packageName string, version string) (names []string, versions []string) {
	args := pa.Called(packageName, version)
	return args.Get(0).([]string), args.Get(1).([]string)
}
func (pa Mock) DownloadArchiveInfo(packageName string, version string) (string, error) {
	args := pa.Called(packageName, version)
	return args.String(0), args.Error(1)
}
