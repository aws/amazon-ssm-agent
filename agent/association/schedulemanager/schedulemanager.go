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
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/gorhill/cronexpr"
)

var associations = []*model.AssociationRawData{}
var lock sync.RWMutex

// Refresh refreshes cached associationRawData
func Refresh(log log.T, assocs []*model.AssociationRawData) {
	lock.Lock()
	defer lock.Unlock()

	log.Debugf("Refresh cached association data with %v associations", len(assocs))
	currentTime := times.DefaultClock.Now()
	unchangedAssociation := 0

	for _, newAssoc := range assocs {
		foundMatch := false
		for _, oldAssoc := range associations {
			if *newAssoc.Association.AssociationId == *oldAssoc.Association.AssociationId {
				if *newAssoc.Association.Checksum == *oldAssoc.Association.Checksum {
					unchangedAssociation++
					newAssoc.Update(oldAssoc)
					foundMatch = true
				}
				break
			}
		}

		if !foundMatch {
			newAssoc.Initialize(log, currentTime)

			//todo: call service to update association status
			if newAssoc.ExcludeFromFutureScheduling {
				log.Infof("Exclude association %v from future scheduling", *newAssoc.Association.AssociationId)
			} else {
				log.Infof("Scheduling association %v, set next ScheduledDate to %v", *newAssoc.Association.AssociationId, newAssoc.NextScheduledDate.String())
			}

			if assocContent, err := jsonutil.Marshal(newAssoc); err != nil {
				log.Errorf("Failed to parse scheduled association, %v", err)
			} else {
				log.Debugf("Scheduled Association content is %v", jsonutil.Indent(assocContent))
			}

		}
	}

	associations = assocs
	log.Debugf("Refresh cached association data completed, %v new assocations associated", len(assocs)-unchangedAssociation)
}

// LoadNextScheduledAssociation returns next scheduled association
func LoadNextScheduledAssociation(log log.T) (*model.AssociationRawData, error) {
	lock.Lock()
	defer lock.Unlock()

	if len(associations) == 0 {
		return nil, nil
	}

	for _, assoc := range associations {
		if assoc.ExcludeFromFutureScheduling {
			continue
		}

		if assoc.NextScheduledDate.Before(times.DefaultClock.Now()) || assoc.NextScheduledDate.Equal(times.DefaultClock.Now()) {
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

	currentTime := times.DefaultClock.Now()
	for _, assoc := range associations {
		if *assoc.Association.AssociationId == associationID {
			assoc.NextScheduledDate = cronexpr.MustParse(assoc.Expression).Next(currentTime)
			log.Debugf("Update Association %v next ScheduledDate to %v", *assoc.Association.AssociationId, assoc.NextScheduledDate.String())
			break
		}
	}

	log.Debugf("Association %v no longer associated", associationID)
}

// MarkAssociationAsCompleted sets exclude from future scheduling to false
func MarkAssociationAsCompleted(log log.T, associationID string) {
	lock.Lock()
	defer lock.Unlock()

	for _, assoc := range associations {
		if *assoc.Association.AssociationId == associationID {
			assoc.ExcludeFromFutureScheduling = true
			log.Debugf("Exclude Association %v from future scheduling", *assoc.Association.AssociationId)
			break
		}
	}

	log.Debugf("Association %v no longer associated", associationID)
}

// Schedules returns all the cached schedules
func Schedules() []*model.AssociationRawData {
	lock.RLock()
	defer lock.RUnlock()
	return associations
}
