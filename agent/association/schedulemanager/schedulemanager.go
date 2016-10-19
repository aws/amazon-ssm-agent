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
package schedulemanager

import (
	"fmt"
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/association/model"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/gorhill/cronexpr"
)

var associations = []*model.AssociationRawData{}
var lock sync.RWMutex

var cronExpressionEveryFiveMinutes = "*/5 * * * *"

// Refresh refreshes cached associationRawData
func Refresh(log log.T, assocs []*model.AssociationRawData) {
	lock.Lock()
	defer lock.Unlock()

	log.Debugf("Refresh cached association data with %v associations", len(assocs))

	currentTime := times.DefaultClock.Now()
	for _, assoc := range assocs {
		assoc.CreateDate = currentTime
		if assoc.Association.ScheduleExpression == nil {
			assoc.Association.ScheduleExpression = aws.String("")
		}

		if _, err := cronexpr.Parse(*assoc.Association.ScheduleExpression); err != nil {
			log.Infof("Failed to parse schedule expression %v, %v", *(assoc.Association.ScheduleExpression), err)
			log.Infof("Set schedule expression to default %v", cronExpressionEveryFiveMinutes)
			assoc.Association.ScheduleExpression = aws.String(cronExpressionEveryFiveMinutes)
		}

		log.Debugf("Loaded association %v with schedule expression %v", *(assoc.Association.AssociationId), *(assoc.Association.ScheduleExpression))
	}

	for _, assoc := range assocs {
		var assocContent string
		var err error
		if assocContent, err = jsonutil.Marshal(assoc); err != nil {
			log.Errorf("failed to parse scheduled association, %v", err)
		}
		log.Debugf("Parsed Scheduled Association is %v", jsonutil.Indent(assocContent))
	}

	//TODO: check how we going to handle the association re-run for 1.2, 1.0
	for _, assoc := range assocs {
		if assoc.NextScheduledDate.IsZero() {
			if *assoc.Association.ScheduleExpression == cronExpressionEveryFiveMinutes {
				// run association immediately
				assoc.NextScheduledDate = currentTime
			} else {
				assoc.NextScheduledDate = cronexpr.MustParse(*assoc.Association.ScheduleExpression).Next(currentTime)
			}

			log.Debugf("Update Association %v next ScheduledDate to %v", *assoc.Association.AssociationId, assoc.NextScheduledDate.String())
		}
	}

	associations = assocs
}

// LoadNextScheduledAssociation returns next scheduled association
func LoadNextScheduledAssociation(log log.T) (*model.AssociationRawData, error) {
	lock.Lock()
	defer lock.Unlock()

	if len(associations) == 0 {
		return nil, nil
	}

	for _, assoc := range associations {
		if assoc.NextScheduledDate.Before(times.DefaultClock.Now()) || assoc.NextScheduledDate.Equal(times.DefaultClock.Now()) {

			var assocContent string
			var err error
			if assocContent, err = jsonutil.Marshal(assoc); err != nil {
				return nil, fmt.Errorf("failed to parse scheduled association, %v", err)
			}
			log.Debugf("Next scheduled association is %v", jsonutil.Indent(assocContent))

			return assoc, nil
		}
	}

	return nil, nil
}

// MarkScheduledAssociationAsCompleted sets next scheduled date for the given association id
func MarkScheduledAssociationAsCompleted(log log.T, associationID string) {
	lock.Lock()
	defer lock.Unlock()

	currentTime := times.DefaultClock.Now()
	for _, assoc := range associations {
		if *assoc.Association.AssociationId == associationID {
			assoc.NextScheduledDate = cronexpr.MustParse(*assoc.Association.ScheduleExpression).Next(currentTime)
			log.Debugf("Update Association %v next ScheduledDate to %v", *assoc.Association.AssociationId, assoc.NextScheduledDate.String())
			break
		}
	}
}

// Schedules returns all the cached schedules
func Schedules() []*model.AssociationRawData {
	lock.Lock()
	defer lock.Unlock()
	return associations
}
