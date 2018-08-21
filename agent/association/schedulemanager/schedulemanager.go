// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package schedulemanager schedules association and submits the association to the task pool
// schedulemanager is a singleton so it can be access at the plugin level
package schedulemanager

import (
	"fmt"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/association/model"
	complianceModel "github.com/aws/amazon-ssm-agent/agent/compliance/model"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/aws/aws-sdk-go/aws"
)

var associations = []*model.InstanceAssociation{}
var lock sync.RWMutex

// Refresh refreshes cached associationRawData
func Refresh(log log.T, assocs []*model.InstanceAssociation) {
	lock.Lock()
	defer lock.Unlock()

	associations = []*model.InstanceAssociation{}
	log.Debugf("Refreshing schedule manager with %v associations", len(assocs))

	for _, newAssoc := range assocs {
		// if association has errors, it will be excluded for the future schedule
		// the next refresh (default to 10 minutes) will retry it
		if len(newAssoc.Errors) == 0 {
			associations = append(associations, newAssoc)
		}
	}

	numberOfNewAssoc := 0
	for _, assoc := range associations {
		assoc.SetNextScheduledDate(log)
		if assoc.NextScheduledDate != nil {
			log.Infof("Scheduling association %v, setting next ScheduledDate to %v", *assoc.Association.AssociationId, times.ToIsoDashUTC(*assoc.NextScheduledDate))
		}

		if assocContent, err := jsonutil.Marshal(assoc); err != nil {
			log.Errorf("Failed to parse scheduled association, %v", err)
		} else {
			log.Debugf("Scheduled Association content is %v", jsonutil.Indent(assocContent))
		}

		if assoc.Association.LastExecutionDate == nil {
			numberOfNewAssoc++
		}
	}

	complianceModel.RefreshAssociationComplianceItems(associations)

	log.Infof("Schedule manager refreshed with %v associations, %v new associations associated", len(associations), numberOfNewAssoc)
}

// LoadNextScheduledAssociation returns next scheduled association
func LoadNextScheduledAssociation(log log.T) (*model.InstanceAssociation, error) {
	lock.Lock()
	defer lock.Unlock()

	if len(associations) == 0 {
		return nil, nil
	}

	for _, assoc := range associations {
		currentTime := time.Now().UTC()
		if assoc.NextScheduledDate == nil {
			continue
		}

		if (*assoc.NextScheduledDate).Before(currentTime) || (*assoc.NextScheduledDate).Equal(currentTime) {
			if assocContent, err := jsonutil.Marshal(assoc); err != nil {
				return nil, fmt.Errorf("failed to parse scheduled association, %v", err)
			} else {
				log.Infof("Next scheduled association is %v", jsonutil.Indent(assocContent))
			}

			return assoc, nil
		}
	}

	return nil, nil
}

// LoadNextScheduledDate returns next scheduled date
func LoadNextScheduledDate(log log.T) *time.Time {
	lock.RLock()
	defer lock.RUnlock()

	var nextScheduleDate *time.Time
	for _, assoc := range associations {
		if assoc.NextScheduledDate == nil {
			continue
		}

		if nextScheduleDate == nil {
			nextScheduleDate = assoc.NextScheduledDate
		}

		if nextScheduleDate.After(*assoc.NextScheduledDate) {
			nextScheduleDate = assoc.NextScheduledDate
		}
	}

	return nextScheduleDate
}

// UpdateNextScheduledDate sets next scheduled date for the given association
func UpdateNextScheduledDate(log log.T, associationID string) {
	lock.Lock()
	defer lock.Unlock()

	for _, assoc := range associations {
		if *assoc.Association.AssociationId == associationID {
			assoc.Association.LastExecutionDate = aws.Time(time.Now().UTC())
			assoc.SetNextScheduledDate(log)
			if assoc.NextScheduledDate != nil {
				log.Infof("Scheduling association %v, setting next ScheduledDate to %v", *assoc.Association.AssociationId, times.ToIsoDashUTC(*assoc.NextScheduledDate))
			}
			break
		}
	}
}

// UpdateAssociationStatus sets detailed status for the given association
func UpdateAssociationStatus(associationID string, status string) {
	lock.Lock()
	defer lock.Unlock()

	for _, assoc := range associations {
		if *assoc.Association.AssociationId == associationID {
			assoc.Association.DetailedStatus = aws.String(status)
			break
		}
	}
}

// IsAssociationInProgress returns if given association has detailed status as InProgress
func IsAssociationInProgress(associationID string) bool {
	lock.Lock()
	defer lock.Unlock()

	for _, assoc := range associations {
		if *assoc.Association.AssociationId == associationID {
			if assoc.Association.DetailedStatus == nil {
				return false
			}

			if *assoc.Association.DetailedStatus == contracts.AssociationStatusInProgress {
				return true
			}

			return false
		}
	}

	return false
}

// Schedules returns all the cached schedules
func Schedules() []*model.InstanceAssociation {
	lock.RLock()
	defer lock.RUnlock()
	return associations
}

func AssociationExists(associationID string) bool {
	for _, assoc := range associations {
		if *assoc.Association.AssociationId == associationID {
			return true
		}
	}
	return false
}
