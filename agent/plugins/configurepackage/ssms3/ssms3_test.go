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

package ssms3

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/stretchr/testify/assert"
)

func TestGetLatestVersion_NumericSort(t *testing.T) {
	versions := [3]string{"1.0.0", "2.0.0", "10.0.0"}
	latest := getLatestVersion(versions[:], "")
	assert.Equal(t, "10.0.0", latest)
}

func TestGetLatestVersion_OnlyOneValid(t *testing.T) {
	versions := [3]string{"0.0.0", "1.0", "1.0.0.0"}
	latest := getLatestVersion(versions[:], "")
	assert.Equal(t, "0.0.0", latest)
}

func TestGetLatestVersion_NoneValid(t *testing.T) {
	versions := [3]string{"Foo", "1.0", "1.0.0.0"}
	latest := getLatestVersion(versions[:], "")
	assert.Equal(t, "", latest)
}

func TestGetLatestVersion_None(t *testing.T) {
	versions := make([]string, 0)
	latest := getLatestVersion(versions[:], "")
	assert.Equal(t, "", latest)
}

func TestEndpointEuCentral1(t *testing.T) {
	service := New("", "eu-central-1")
	assert.Equal(t, fmt.Sprintf("https://s3.eu-central-1.amazonaws.com/amazon-ssm-packages-eu-central-1/Packages/{PackageName}/%v/%v", appconfig.PackagePlatform, runtime.GOARCH), service.packageURL)
}

func TestEndpointEuCentral1Beta(t *testing.T) {
	service := New("beta", "eu-central-1")
	assert.Equal(t, fmt.Sprintf("https://s3.amazonaws.com/amazon-ssm-packages-beta/Packages/{PackageName}/%v/%v", appconfig.PackagePlatform, runtime.GOARCH), service.packageURL)
}

func TestEndpointEuCentral1Gamma(t *testing.T) {
	service := New("gamma", "eu-central-1")
	assert.Equal(t, fmt.Sprintf("https://s3.amazonaws.com/amazon-ssm-packages-us-east-1-gamma/Packages/{PackageName}/%v/%v", appconfig.PackagePlatform, runtime.GOARCH), service.packageURL)
}

func TestEndpointCnNorth1(t *testing.T) {
	service := New("", "cn-north-1")
	assert.Equal(t, fmt.Sprintf("https://s3.cn-north-1.amazonaws.com.cn/amazon-ssm-packages-cn-north-1/Packages/{PackageName}/%v/%v", appconfig.PackagePlatform, runtime.GOARCH), service.packageURL)
}

func TestEndpointCnNorth1Beta(t *testing.T) {
	service := New("beta", "cn-north-1")
	assert.Equal(t, fmt.Sprintf("https://s3.amazonaws.com/amazon-ssm-packages-beta/Packages/{PackageName}/%v/%v", appconfig.PackagePlatform, runtime.GOARCH), service.packageURL)
}

func TestEndpointCnNorth1Gamma(t *testing.T) {
	service := New("gamma", "cn-north-1")
	assert.Equal(t, fmt.Sprintf("https://s3.amazonaws.com/amazon-ssm-packages-us-east-1-gamma/Packages/{PackageName}/%v/%v", appconfig.PackagePlatform, runtime.GOARCH), service.packageURL)
}

func TestEndpointUsEast1(t *testing.T) {
	service := New("", "us-east-1")
	assert.Equal(t, fmt.Sprintf("https://s3.amazonaws.com/amazon-ssm-packages-us-east-1/Packages/{PackageName}/%v/%v", appconfig.PackagePlatform, runtime.GOARCH), service.packageURL)
}

func TestEndpointUsEast1Beta(t *testing.T) {
	service := New("beta", "us-east-1")
	assert.Equal(t, fmt.Sprintf("https://s3.amazonaws.com/amazon-ssm-packages-beta/Packages/{PackageName}/%v/%v", appconfig.PackagePlatform, runtime.GOARCH), service.packageURL)
}

func TestEndpointUsEast1Gamma(t *testing.T) {
	service := New("gamma", "us-east-1")
	assert.Equal(t, fmt.Sprintf("https://s3.amazonaws.com/amazon-ssm-packages-us-east-1-gamma/Packages/{PackageName}/%v/%v", appconfig.PackagePlatform, runtime.GOARCH), service.packageURL)
}
