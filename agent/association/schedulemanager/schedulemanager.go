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
	"github.com/aws/aws-sdk-go/aws"
)

var associations = []*model.InstanceAssociation{}
var lock sync.RWMutex

// Refresh refreshes cached associationRawData
func Refresh(log log.T, assocs []*model.InstanceAssociation, svc service.T) {
	lock.Lock()
	defer lock.Unlock()

	log.Debugf("Refreshing schedule manager with %v associations", len(assocs))

	for _, newAssoc := range assocs {
		for _, oldAssoc := range associations {
			if *newAssoc.Association.AssociationId == *oldAssoc.Association.AssociationId && *newAssoc.Association.Checksum == *oldAssoc.Association.Checksum {
				newAssoc.Copy(oldAssoc)
			}
		}

		// validate association expression, fail association if expression cannot be passed
		if err := newAssoc.ParseExpression(log); err != nil {
			message := fmt.Sprintf("Encountered error while parsing expression for association %v", *newAssoc.Association.AssociationId)
			log.Errorf("%v, %v", message, err)
			svc.UpdateInstanceAssociationStatus(
				log,
				*newAssoc.Association.AssociationId,
				*newAssoc.Association.Name,
				*newAssoc.Association.InstanceId,
				contracts.AssociationStatusFailed,
				contracts.AssociationErrorCodeInvalidExpression,
				times.ToIso8601UTC(time.Now()),
				message,
				service.NoOutputUrl)
			newAssoc.ExcludeFromFutureScheduling = true
		}

		if newAssoc.ExcludeFromFutureScheduling {
			log.Debugf("Association %v has been excluded from future scheduling", *newAssoc.Association.AssociationId)
			continue
		}

		// set next scheduled date if association need to run now or next scheduled date is empty
		if newAssoc.RunNow || newAssoc.NextScheduledDate.IsZero() {
			newAssoc.SetNextScheduledDate()

			log.Infof("Scheduling association %v, setting next ScheduledDate to %v", *newAssoc.Association.AssociationId, newAssoc.NextScheduledDate.String())
			if assocContent, err := jsonutil.Marshal(newAssoc); err != nil {
				log.Errorf("Failed to parse scheduled association, %v", err)
			} else {
				log.Debugf("Scheduled Association content is %v", jsonutil.Indent(assocContent))
			}

		}
	}

	associations = assocs

	numberOfNewAssoc := 0
	for _, assoc := range associations {
		if assoc.Association.LastExecutionDate.IsZero() {
			numberOfNewAssoc++
		}
	}

	log.Infof("Schedule manager refreshed, %v new assocations associated", numberOfNewAssoc)
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
				log.Infof("Next scheduled association is %v", jsonutil.Indent(assocContent))
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

	for _, assoc := range associations {
		if *assoc.Association.AssociationId == associationID {
			assoc.Association.LastExecutionDate = aws.Time(time.Now().UTC())

			if assoc.LegacyAssociation {
				assoc.ExcludeFromFutureScheduling = true
				log.Infof("Association %v has been executed, excluding from future scheduling", *assoc.Association.Name)
			} else {
				assoc.SetNextScheduledDate()
				log.Infof("Association %v next ScheduledDate is updated to %v", *assoc.Association.Name, assoc.NextScheduledDate.String())
			}

			break
		}
	}
}

// ExcludeAssocFromFutureScheduling sets exclude from future scheduling to true
func ExcludeAssocFromFutureScheduling(log log.T, associationID string) {
	lock.Lock()
	defer lock.Unlock()

	for _, assoc := range associations {
		if *assoc.Association.AssociationId == associationID {
			assoc.ExcludeFromFutureScheduling = true
			log.Debugf("Association %v is excluded from future scheduling", *assoc.Association.Name)
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
