// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Amazon Software License (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/asl/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package association

import (
	"io/ioutil"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/poll"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	instanceId    = "i-test"
	testName      = "testDoc"
	sampleDocFile = "./sampleDoc.json"
)

var ssmMock = new(MockedSSM)
var contextMock = context.NewMockDefault()

// Integration test to test the poll loop where association list contains no associations
func TestMockedAssociationPollLoopWhereAssociationIsNull(t *testing.T) {
	associationList := []*ssm.Association{}

	nextToken := ""

	listAssociationsOutput := ssm.ListAssociationsOutput{
		Associations: associationList,
		NextToken:    &nextToken,
	}
	var getListAssociationsInput = func(instanceID string) *ssm.ListAssociationsInput {
		return &ssm.ListAssociationsInput{
			AssociationFilterList: nil,
			MaxResults:            aws.Int64(1),
			NextToken:             aws.String(""),
		}
	}
	ssmMock.On("ListAssociations", getListAssociationsInput(instanceId)).Return(&listAssociationsOutput, nil)
	region := "us-east-1"
	p := NewPollListAssociationsService(contextMock, region, instanceId)
	pollService := poll.PollService{}
	pollService.StartPolling(p.PollListAssociations, 1, contextMock.Log())
	time.Sleep(time.Second)
	assert.Nil(t, p.association)
}

func TestMockedAssociationPoll(t *testing.T) {
	p, getDocumentOutput := prepareTest()
	p.PollListAssociations()
	assert.Equal(t, instanceId, *p.association.InstanceId)
	assert.Equal(t, testName, *p.association.Name)
	assert.Equal(t, getDocumentOutput.Content, p.document)
}

func loadFile(fileName string) (result []byte) {
	result, err := ioutil.ReadFile(fileName)
	if err != nil {
	}
	return
}

// Integration test to test the poll loop.
func TestMockedAssociationPollLoop(t *testing.T) {
	p, getDocumentOutput := prepareTest()
	pollService := poll.PollService{}
	pollService.StartPolling(p.PollListAssociations, 1, contextMock.Log())
	time.Sleep(time.Second)
	assert.Equal(t, instanceId, *p.association.InstanceId)
	assert.Equal(t, testName, *p.association.Name)
	assert.Equal(t, getDocumentOutput.Content, p.document)
}

func prepareTest() (*PollListAssociationsService, ssm.GetDocumentOutput) {

	newSSM = func(region string) ssmiface.SSMAPI {
		return ssmMock
	}

	name := testName
	iid := instanceId
	region := "us-east-1"
	p := NewPollListAssociationsService(contextMock, region, instanceId)

	association := ssm.Association{
		InstanceId: &iid,
		Name:       &name,
	}
	associationList := []*ssm.Association{&association}

	nextToken := ""

	listAssociationsOutput := ssm.ListAssociationsOutput{
		Associations: associationList,
		NextToken:    &nextToken,
	}
	ssmMock.On("ListAssociations", mock.AnythingOfType("*ssm.ListAssociationsInput")).Return(&listAssociationsOutput, nil)

	// mock ListAssociations function to return an association
	content := string(loadFile(sampleDocFile))
	getDocumentOutput := ssm.GetDocumentOutput{
		Content: &content,
		Name:    &name,
	}
	// mock GetDocument function to return the loaded document
	ssmMock.On("GetDocument", mock.AnythingOfType("*ssm.GetDocumentInput")).Return(&getDocumentOutput, nil)
	return p, getDocumentOutput
}
