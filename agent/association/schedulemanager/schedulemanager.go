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
	"github.com/aws/amazon-ssm-agent/agent/association/service"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/gorhill/cronexpr"
)

var associations = []*model.InstanceAssociation{}
var lock sync.RWMutex

// Refresh refreshes cached associationRawData
func Refresh(log log.T, assocs []*model.InstanceAssociation, svc service.T) {
	lock.Lock()
	defer lock.Unlock()

	log.Debugf("Refreshing schedule manager with %v associations", len(assocs))
	unchangedAssociations := 0

	for _, newAssoc := range assocs {
		isNew := true
		for _, oldAssoc := range associations {
			if *newAssoc.Association.AssociationId == *oldAssoc.Association.AssociationId && *newAssoc.Association.Checksum == *oldAssoc.Association.Checksum {
				unchangedAssociations++
				newAssoc.Update(oldAssoc)
				isNew = false
			}
		}

		if isNew || newAssoc.RunNow || newAssoc.NextScheduledDate.IsZero() {
			if err := newAssoc.Initialize(log); err != nil {
				message := "Encountered error while initializing association"
				log.Errorf("%v, %v", message, err)
				svc.UpdateInstanceAssociationStatus(log,
					*newAssoc.Association.AssociationId,
					*newAssoc.Association.Name,
					*newAssoc.Association.InstanceId,
					contracts.AssociationStatusFailed,
					contracts.AssociationErrorCodeInvalidExpression,
					times.ToIso8601UTC(newAssoc.CreateDate),
					message)
				newAssoc.ExcludeFromFutureScheduling = true
			}

			if newAssoc.ExcludeFromFutureScheduling {
				log.Debugf("Association %v is excluded from future scheduling", *newAssoc.Association.AssociationId)
				continue
			}

			log.Infof("Scheduling association %v, set next ScheduledDate to %v", *newAssoc.Association.AssociationId, newAssoc.NextScheduledDate.String())
			if assocContent, err := jsonutil.Marshal(newAssoc); err != nil {
				log.Errorf("Failed to parse scheduled association, %v", err)
			} else {
				log.Debugf("Scheduled Association content is %v", jsonutil.Indent(assocContent))
			}

		}
	}

	associations = assocs
	log.Infof("Schedule manager refreshed, %v new assocations associated", len(assocs)-unchangedAssociations)
}

// LoadNextScheduledAssociation returns next scheduled association
func LoadNextScheduledAssociation(log log.T) (*model.InstanceAssociation, error) {
	lock.Lock()
	defer lock.Unlock()

	if len(associations) == 0 {
		return nil, nil
	}

	for _, assoc := range associations {
		if assoc.ExcludeFromFutureScheduling {
			continue
		}

		currentTime := time.Now().UTC()
		if assoc.NextScheduledDate.Before(currentTime) ||
			assoc.NextScheduledDate.Equal(currentTime) {

			if assocContent, err := jsonutil.Marshal(assoc); err != nil {
				return nil, fmt.Errorf("failed to parse scheduled association, %v", err)
			} else {
				log.Debugf("Next scheduled association is %v", jsonutil.Indent(assocContent))
			}

			return assoc, nil
		}
	}

	return nil, nil
}

// LoadNextScheduledDate returns next scheduled date
func LoadNextScheduledDate(log log.T) time.Time {
	lock.RLock()
	defer lock.RUnlock()

	nextScheduleDate := time.Time{}
	for _, assoc := range associations {
		if assoc.ExcludeFromFutureScheduling {
			continue
		}

		if assoc.NextScheduledDate.IsZero() {
			continue
		}

		if nextScheduleDate.IsZero() {
			nextScheduleDate = assoc.NextScheduledDate
		}

		if nextScheduleDate.After(assoc.NextScheduledDate) {
			nextScheduleDate = assoc.NextScheduledDate
		}
	}

	return nextScheduleDate
}

// UpdateNextScheduledDate sets next scheduled date for the given association
func UpdateNextScheduledDate(log log.T, associationID string) {
	lock.Lock()
	defer lock.Unlock()

	currentTime := time.Now().UTC()
	for _, assoc := range associations {
		if *assoc.Association.AssociationId == associationID {
			if assoc.LegacyAssociation {
				assoc.ExcludeFromFutureScheduling = true
				log.Debugf("Association %v is excluded from future scheduling", *assoc.Association.AssociationId)
			} else {
				assoc.NextScheduledDate = cronexpr.MustParse(assoc.Expression).Next(currentTime).UTC()
				log.Debugf("Association %v next ScheduledDate is updated to %v", *assoc.Association.AssociationId, assoc.NextScheduledDate.String())
			}

			break
		}
	}
}

// MarkAssociationAsCompleted sets exclude from future scheduling to false
func MarkAssociationAsCompleted(log log.T, associationID string) {
	lock.Lock()
	defer lock.Unlock()

	for _, assoc := range associations {
		if *assoc.Association.AssociationId == associationID {
			assoc.ExcludeFromFutureScheduling = true
			log.Debugf("Association %v is excluded from future scheduling", *assoc.Association.AssociationId)
			break
		}
	}
}

// Schedules returns all the cached schedules
func Schedules() []*model.InstanceAssociation {
	lock.RLock()
	defer lock.RUnlock()
	return associations
}
