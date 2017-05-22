// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package compliance

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"

	associationModel "github.com/aws/amazon-ssm-agent/agent/association/model"
	"github.com/aws/amazon-ssm-agent/agent/compliance/model"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/datauploader"
	ssmSvc "github.com/aws/amazon-ssm-agent/agent/ssm"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func MockComplianceUploader() *ComplianceUploader {
	var uploader ComplianceUploader

	optimizer := datauploader.NewMockDefault()
	optimizer.On("GetContentHash", mock.AnythingOfType("string")).Return("RandomComplianceItem")
	optimizer.On("UpdateContentHash", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)

	uploader.optimizer = optimizer
	uploader.ssmSvc = ssmSvc.NewMockDefault()
	uploader.context = context.NewMockDefault()
	return &uploader
}

func AssociationComplianceItem() *model.AssociationComplianceItem {
	item := &model.AssociationComplianceItem{
		AssociationId:      "associationId",
		ExecutionTime:      time.Now(),
		DocumentName:       "testDoc",
		DocumentVersion:    "1",
		Title:              "testTitle",
		ComplianceSeverity: "UNSPECIFIED",
		ComplianceStatus:   "Compliant",
	}
	return item
}

func FakeComplianceItems(count int) (items []*model.AssociationComplianceItem) {
	i := 0

	for i < count {
		var compliantStatus string
		if i%2 == 0 {
			compliantStatus = "Compliant"
		} else {
			compliantStatus = "NonCompliant"
		}

		items = append(items, &model.AssociationComplianceItem{
			AssociationId:      "fakeAssociation" + strconv.Itoa(i),
			ExecutionTime:      time.Now(),
			DocumentName:       "fakeDoc" + strconv.Itoa(i),
			DocumentVersion:    strconv.Itoa(i),
			Title:              "title " + strconv.Itoa(i),
			ComplianceSeverity: "UNSPECIFIED",
			ComplianceStatus:   compliantStatus,
		})
		i++
	}

	return
}

func TestConvertToSsmComplianceItemAreEqual(t *testing.T) {

	var items []*model.AssociationComplianceItem
	var complianceItems []*ssm.ComplianceItemEntry
	var err error

	c := context.NewMockDefault()
	u := MockComplianceUploader()

	items = append(items, AssociationComplianceItem())
	complianceItems, _, err = u.ConvertToSsmAssociationComplianceItems(c.Log(), items, "RandomHash")

	assert.Nil(t, err)
	assert.Equal(t, len(items), len(complianceItems))

	ssmComplianceItem := complianceItems[0]
	assert.Equal(t, "associationId", *ssmComplianceItem.Id)
	assert.Equal(t, "testTitle", *ssmComplianceItem.Title)
	assert.Equal(t, "UNSPECIFIED", *ssmComplianceItem.Severity)
	assert.Equal(t, "Compliant", *ssmComplianceItem.Status)
	assert.Equal(t, "testDoc", *ssmComplianceItem.Details["DocumentName"])
	assert.Equal(t, "1", *ssmComplianceItem.Details["DocumentVersion"])
}

func TestConvertToSsmComplianceItems(t *testing.T) {

	var items []*model.AssociationComplianceItem
	var complianceItems []*ssm.ComplianceItemEntry
	var err error

	c := context.NewMockDefault()
	u := MockComplianceUploader()

	for _, newItem := range FakeComplianceItems(2) {
		items = append(items, newItem)
	}
	complianceItems, _, err = u.ConvertToSsmAssociationComplianceItems(c.Log(), items, "RandomHash")

	assert.Nil(t, err)
	assert.Equal(t, len(items), len(complianceItems))
}

func TestUpdateAssociationCompliance(t *testing.T) {
	u := MockComplianceUploader()

	association1 := &associationModel.InstanceAssociation{
		Association: &ssm.InstanceAssociationSummary{
			Name:            aws.String("testDoc"),
			AssociationId:   aws.String("association_1"),
			DocumentVersion: aws.String("1"),
			InstanceId:      aws.String("i-123"),
		},
	}

	serviceMock := ssmSvc.NewMockDefault()
	u.ssmSvc = serviceMock

	optimizer := datauploader.NewMockDefault()
	optimizer.On("GetContentHash", mock.AnythingOfType("string")).Return("RandomComplianceItem")
	optimizer.On("UpdateContentHash", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)
	u.optimizer = optimizer

	u.context = context.NewMockDefault()

	mockPutComplianceItemOutput := &ssm.PutComplianceItemsOutput{}
	serviceMock.On(
		"PutComplianceItems",
		mock.AnythingOfType("*log.Mock"),
		mock.AnythingOfType("*time.Time"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("[]*ssm.ComplianceItemEntry")).Return(mockPutComplianceItemOutput, nil)

	executionTime := time.Now()
	u.UpdateAssociationCompliance(
		*association1.Association.AssociationId,
		*association1.Association.InstanceId,
		*association1.Association.Name,
		*association1.Association.DocumentVersion,
		"Success",
		executionTime)

	assert.True(t, serviceMock.AssertNumberOfCalls(t, "PutComplianceItems", 1))

	arguments := serviceMock.Calls[0].Arguments
	assert.Equal(t, &executionTime, arguments.Get(1))
	assert.Equal(t, "", arguments.String(2))
	assert.Equal(t, "", arguments.String(3))
	assert.Equal(t, "i-123", arguments.String(4))
	assert.Equal(t, "Association", arguments.String(5))
	assert.NotNil(t, arguments.String(6))

	assert.Equal(t, 1, len(arguments.Get(7).([]*ssm.ComplianceItemEntry)))

	optimizer.AssertCalled(t, "GetContentHash", mock.AnythingOfType("string"))
	optimizer.AssertCalled(t, "UpdateContentHash", mock.AnythingOfType("string"), mock.AnythingOfType("string"))

}

func TestConvertReturnEmptyForHashMatch(t *testing.T) {

	var items []*model.AssociationComplianceItem
	var complianceItems []*ssm.ComplianceItemEntry

	c := context.NewMockDefault()
	u := MockComplianceUploader()

	items = append(items, AssociationComplianceItem())
	dataB, _ := json.Marshal(items)
	hash := calculateCheckSum(dataB)

	optimizer := datauploader.NewMockDefault()
	optimizer.On("GetContentHash", mock.AnythingOfType("string")).Return(hash)
	u.optimizer = optimizer

	var newHash string
	complianceItems, newHash, _ = u.ConvertToSsmAssociationComplianceItems(c.Log(), items, hash)

	assert.Equal(t, 0, len(complianceItems))
	assert.Equal(t, hash, newHash)
}

func TestChecksumSameForSameContent(t *testing.T) {
	var items []*model.AssociationComplianceItem
	var items2 []*model.AssociationComplianceItem

	items = append(items, AssociationComplianceItem())
	items2 = append(items2, AssociationComplianceItem())

	dataB1, _ := json.Marshal(items)
	dataB2, _ := json.Marshal(items)

	assert.Equal(t, calculateCheckSum(dataB1), calculateCheckSum(dataB2))
}
