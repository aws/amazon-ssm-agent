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

package model

import (
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/association/model"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/stretchr/testify/assert"
)

func TestUpdateAssociationComplianceItemReturnCompliant(t *testing.T) {
	association1 := &model.InstanceAssociation{
		Association: &ssm.InstanceAssociationSummary{
			Name:            aws.String("testDoc"),
			AssociationId:   aws.String("association_1"),
			DocumentVersion: aws.String("1"),
		},
	}

	association2 := &model.InstanceAssociation{
		Association: &ssm.InstanceAssociationSummary{
			Name:            aws.String("testDoc2"),
			AssociationId:   aws.String("association_2"),
			DocumentVersion: aws.String("2"),
		},
	}

	association3 := &model.InstanceAssociation{
		Association: &ssm.InstanceAssociationSummary{
			Name:            aws.String("testDoc3"),
			AssociationId:   aws.String("association_3"),
			DocumentVersion: aws.String("3"),
		},
	}

	associationStatus1 := contracts.AssociationStatusSuccess
	executionTime1 := time.Now()

	associationStatus2 := contracts.AssociationStatusTimedOut
	executionTime2 := time.Now()

	associationStatus3 := contracts.AssociationStatusFailed
	executionTime3 := time.Now()

	UpdateAssociationComplianceItem(*association1.Association.AssociationId, *association1.Association.Name, *association1.Association.DocumentVersion, associationStatus1, executionTime1)
	UpdateAssociationComplianceItem(*association2.Association.AssociationId, *association2.Association.Name, *association2.Association.DocumentVersion, associationStatus2, executionTime2)
	UpdateAssociationComplianceItem(*association3.Association.AssociationId, *association3.Association.Name, *association3.Association.DocumentVersion, associationStatus3, executionTime3)

	complianceItems := GetAssociationComplianceEntries()
	assert.Equal(t, 3, len(complianceItems))

	item1 := complianceItems[0]
	assert.Equal(t, item1.AssociationId, *association1.Association.AssociationId)
	assert.Equal(t, item1.DocumentName, *association1.Association.Name)
	assert.Equal(t, item1.DocumentVersion, *association1.Association.DocumentVersion)
	assert.Equal(t, item1.ComplianceSeverity, UNSPECIFIED)
	assert.Equal(t, item1.ComplianceStatus, COMPLIANT)
	assert.Equal(t, item1.Title, ASSOCIATION_COMPLIANCE_TITLE)

	item2 := complianceItems[1]
	assert.Equal(t, item2.AssociationId, *association2.Association.AssociationId)
	assert.Equal(t, item2.DocumentName, *association2.Association.Name)
	assert.Equal(t, item2.DocumentVersion, *association2.Association.DocumentVersion)
	assert.Equal(t, item2.ComplianceSeverity, UNSPECIFIED)
	assert.Equal(t, item2.ComplianceStatus, NON_COMPLIANT)
	assert.Equal(t, item2.Title, ASSOCIATION_COMPLIANCE_TITLE)

	item3 := complianceItems[2]
	assert.Equal(t, item3.AssociationId, *association3.Association.AssociationId)
	assert.Equal(t, item3.DocumentName, *association3.Association.Name)
	assert.Equal(t, item3.DocumentVersion, *association3.Association.DocumentVersion)
	assert.Equal(t, item3.ComplianceSeverity, UNSPECIFIED)
	assert.Equal(t, item3.ComplianceStatus, NON_COMPLIANT)
	assert.Equal(t, item3.Title, ASSOCIATION_COMPLIANCE_TITLE)
}

func TestUpdateAssociationComplianceItemIgnoreStaleUpdate(t *testing.T) {
	RefreshAssociationComplianceItems([]*model.InstanceAssociation{})

	association1 := &model.InstanceAssociation{
		Association: &ssm.InstanceAssociationSummary{
			Name:            aws.String("testDoc"),
			AssociationId:   aws.String("associtionId"),
			DocumentVersion: aws.String("1"),
		},
	}

	associationStatus1 := contracts.AssociationStatusSuccess
	executionTime1 := time.Now()
	executionTime2 := time.Now().Add(-100 * time.Second)

	UpdateAssociationComplianceItem(*association1.Association.AssociationId, *association1.Association.Name, *association1.Association.DocumentVersion, associationStatus1, executionTime1)
	UpdateAssociationComplianceItem(*association1.Association.AssociationId, *association1.Association.Name, *association1.Association.DocumentVersion, associationStatus1, executionTime2)

	complianceItems := GetAssociationComplianceEntries()
	assert.Equal(t, 1, len(complianceItems))

	item1 := complianceItems[0]
	assert.Equal(t, item1.AssociationId, *association1.Association.AssociationId)
	assert.Equal(t, item1.DocumentName, *association1.Association.Name)
	assert.Equal(t, item1.DocumentVersion, *association1.Association.DocumentVersion)
	assert.Equal(t, item1.ComplianceSeverity, UNSPECIFIED)
	assert.Equal(t, item1.ComplianceStatus, COMPLIANT)
	assert.Equal(t, item1.Title, ASSOCIATION_COMPLIANCE_TITLE)
}

func TestRefreshAssociationComplianceItems(t *testing.T) {
	RefreshAssociationComplianceItems([]*model.InstanceAssociation{})
	association1 := &model.InstanceAssociation{
		Association: &ssm.InstanceAssociationSummary{
			Name:            aws.String("testDoc"),
			AssociationId:   aws.String("association_1"),
			DocumentVersion: aws.String("1"),
		},
	}

	association2 := &model.InstanceAssociation{
		Association: &ssm.InstanceAssociationSummary{
			Name:            aws.String("testDoc2"),
			AssociationId:   aws.String("association_2"),
			DocumentVersion: aws.String("2"),
		},
	}

	associationStatus1 := contracts.AssociationStatusSuccess
	executionTime1 := time.Now()

	associationStatus2 := contracts.AssociationStatusTimedOut
	executionTime2 := time.Now()

	UpdateAssociationComplianceItem(*association1.Association.AssociationId, *association1.Association.Name, *association1.Association.DocumentVersion, associationStatus1, executionTime1)
	UpdateAssociationComplianceItem(*association2.Association.AssociationId, *association2.Association.Name, *association2.Association.DocumentVersion, associationStatus2, executionTime2)

	complianceItems := GetAssociationComplianceEntries()
	assert.Equal(t, 2, len(complianceItems))

	changedAssociations := []*model.InstanceAssociation{association1}
	RefreshAssociationComplianceItems(changedAssociations)
	complianceItems = GetAssociationComplianceEntries()
	assert.Equal(t, 1, len(complianceItems))

}
