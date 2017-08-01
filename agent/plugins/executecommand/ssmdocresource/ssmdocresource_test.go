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

package ssmdocresource

import (
	"github.com/aws/amazon-ssm-agent/agent/log"
	filemock "github.com/aws/amazon-ssm-agent/agent/plugins/executecommand/filemanager/mock"
	"github.com/aws/amazon-ssm-agent/agent/plugins/executecommand/remoteresource"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"path/filepath"
	"testing"
)

var logMock = log.NewMockLog()

func TestSSMDocResource_ValidateLocationInfo(t *testing.T) {

	locationInfo := `{
		"Name": "AWS-ExecuteCommand",
		"Version": ""
	}`

	ssmresource, _ := NewSSMDocResource(locationInfo)
	_, err := ssmresource.ValidateLocationInfo()

	assert.NoError(t, err)
}

func TestSSMDocResource_ValidateLocationInfoNoName(t *testing.T) {

	locationInfo := `{
		"Name": "",
		"Version": "10"
	}`

	ssmresource, _ := NewSSMDocResource(locationInfo)
	_, err := ssmresource.ValidateLocationInfo()

	assert.Error(t, err)
	assert.Equal(t, "SSM Document name in LocationType must be specified", err.Error())
}

func TestSSMDocResource_PopulateResourceInfo(t *testing.T) {
	locationInfo := `{
		"Name": "AWS-ExecuteCommand",
		"Version": "10"
	}`

	ssmresource, _ := NewSSMDocResource(locationInfo)

	resource := ssmresource.PopulateResourceInfo(logMock, "destination", false)

	assert.False(t, resource.EntireDir)
	assert.Equal(t, remoteresource.Document, resource.TypeOfResource)
	assert.Equal(t, "AWS-ExecuteCommand.json", resource.StarterFile)
	assert.Equal(t, filepath.Join("destination", ssmresource.Info.DocName, ssmresource.Info.DocName+".json"), resource.LocalDestinationPath)
}

func TestSSMDocResource_Download(t *testing.T) {
	depMock := new(ssmDocDepMock)
	fileMock := filemock.FileSystemMock{}

	locationInfo := `{
		"Name": "AWS-ExecuteCommand",
		"Version": "10"
	}`
	content := "content"
	docOutput := ssm.GetDocumentOutput{
		Content: &content,
	}
	ssmresource, _ := NewSSMDocResource(locationInfo)
	dir := filepath.Join("destination", ssmresource.Info.DocName)
	depMock.On("GetDocument", logMock, ssmresource.Info.DocName, ssmresource.Info.DocVersion).Return(&docOutput, nil)
	fileMock.On("MakeDirs", dir).Return(nil)
	fileMock.On("WriteFile", filepath.Join(dir, ssmresource.Info.DocName+".json"), content).Return(nil)

	ssmdocdep = depMock

	err := ssmresource.Download(logMock, fileMock, false, "destination")

	assert.NoError(t, err)
	depMock.AssertExpectations(t)
	fileMock.AssertExpectations(t)
}

func TestSSMDocResource_DownloadEntireDirTrue(t *testing.T) {
	fileMock := filemock.FileSystemMock{}

	locationInfo := `{
		"Name": "AWS-ExecuteCommand",
		"Version": "10"
	}`

	ssmresource, _ := NewSSMDocResource(locationInfo)

	err := ssmresource.Download(logMock, fileMock, true, "destination")

	assert.Error(t, err)
	assert.Equal(t, "EntireDirectory option is not supported for SSMDocument location type.", err.Error())

}

type ssmDocDepMock struct {
	mock.Mock
}

func (s ssmDocDepMock) GetDocument(log log.T, docName string, docVersion string) (response *ssm.GetDocumentOutput, err error) {
	args := s.Called(log, docName, docVersion)
	return args.Get(0).(*ssm.GetDocumentOutput), args.Error(1)
}
