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
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/association/model"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/aws-sdk-go/service/ssm"
)

const UNSPECIFIED string = ssm.ComplianceSeverityUnspecified
const COMPLIANT string = ssm.ComplianceStatusCompliant
const NON_COMPLIANT string = ssm.ComplianceStatusNonCompliant

var ASSOCIATION_COMPLIANCE_TITLE string

type AssociationComplianceItem struct {
	// AssociationId stores the key of compliance status
	AssociationId      string
	ExecutionTime      time.Time
	DocumentName       string
	DocumentVersion    string
	Title              string
	ComplianceSeverity string
	ComplianceStatus   string
}

// Association compliance status is Unspecified by default
var associationComplianceItems = []*AssociationComplianceItem{}
var lock = sync.RWMutex{}

/**
 * Update compliance item based on the executed instance association and update timestamp.
 */
func UpdateAssociationComplianceItem(associationId string, documentName string, documentVersion string, associationStatus string, executionTime time.Time) {
	if contracts.AssociationStatusTimedOut != associationStatus &&
		contracts.AssociationStatusSuccess != associationStatus &&
		contracts.AssociationStatusFailed != associationStatus {
		return
	}

	lock.Lock()
	defer lock.Unlock()

	var compliantStatus = COMPLIANT
	if contracts.AssociationStatusSuccess != associationStatus {
		compliantStatus = NON_COMPLIANT
	}

	var statusFound = false
	for i, status := range associationComplianceItems {
		if status.AssociationId == associationId {
			if status.ExecutionTime.Before(executionTime) {
				associationComplianceItems[i] = &AssociationComplianceItem{
					associationId,
					executionTime,
					documentName,
					documentVersion,
					ASSOCIATION_COMPLIANCE_TITLE,
					UNSPECIFIED,
					compliantStatus,
				}
			}

			statusFound = true
			break
		}
	}

	if !statusFound {
		var newStatus = &AssociationComplianceItem{
			associationId,
			executionTime,
			documentName,
			documentVersion,
			ASSOCIATION_COMPLIANCE_TITLE,
			UNSPECIFIED,
			compliantStatus,
		}

		associationComplianceItems = append(associationComplianceItems, newStatus)
	}
}

/**
 * Refresh association compliance items so legacy association compliance items will be refreshed
 */
func RefreshAssociationComplianceItems(associations []*model.InstanceAssociation) {
	lock.Lock()
	defer lock.Unlock()

	var newComplianceItems = []*AssociationComplianceItem{}
	var associationMap = map[string]*model.InstanceAssociation{}

	for _, assoc := range associations {
		associationMap[*assoc.Association.AssociationId] = assoc
	}

	for _, item := range associationComplianceItems {
		if _, exist := associationMap[item.AssociationId]; exist {
			newComplianceItems = append(newComplianceItems, item)
		}
	}

	associationComplianceItems = newComplianceItems
}

func GetAssociationComplianceEntries() []*AssociationComplianceItem {
	lock.RLock()
	defer lock.RUnlock()

	return associationComplianceItems
}
