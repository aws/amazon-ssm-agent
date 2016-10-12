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

var cronExpressionEveryFiveMinutes = "0 0/5 * * * ?"

// Refresh refreshes cached associationRawData
func Refresh(log log.T, assocs []*model.AssociationRawData) {
	lock.Lock()
	defer lock.Unlock()

	log.Debugf("Refresh cached association data with %v associations", len(assocs))

	//TODO: find out if create date can retrieve from service
	currentTime := times.DefaultClock.Now()
	for _, assoc := range assocs {
		assoc.CreateDate = currentTime
		assoc.Association.ScheduleExpression = aws.String(cronExpressionEveryFiveMinutes)
	}

	associations = assocs
}

// LoadNextScheduledAssociation returns next scheduled association
func LoadNextScheduledAssociation(log log.T) (*model.AssociationRawData, error) {
	lock.Lock()
	defer lock.Unlock()

	for _, assoc := range associations {
		var assocContent string
		var err error
		if assocContent, err = jsonutil.Marshal(assoc); err != nil {
			return nil, fmt.Errorf("failed to parse scheduled association, %v", err)
		}
		log.Debugf("Parsed Scheduled Association is ", jsonutil.Indent(assocContent))
	}

	//TODO: check how we going to handle the association re-run for 1.2, 1.0
	currentTime := times.DefaultClock.Now()
	for _, assoc := range associations {
		if assoc.NextScheduledDate.IsZero() {
			if *assoc.Association.ScheduleExpression == cronExpressionEveryFiveMinutes {
				assoc.NextScheduledDate = currentTime
			} else {
				assoc.NextScheduledDate = cronexpr.MustParse(*assoc.Association.ScheduleExpression).Next(currentTime)
			}

			log.Debugf("Next ScheduledDate is ", assoc.NextScheduledDate.String())
		}
	}

	for _, assoc := range associations {
		if assoc.NextScheduledDate.Before(times.DefaultClock.Now()) || assoc.NextScheduledDate.Equal(times.DefaultClock.Now()) {

			var assocContent string
			var err error
			if assocContent, err = jsonutil.Marshal(assoc); err != nil {
				return nil, fmt.Errorf("failed to parse scheduled association, %v", err)
			}
			log.Debugf("Next scheduled association is ", jsonutil.Indent(assocContent))

			return assoc, nil
		}
	}

	return nil, nil
}

// Schedules returns all the cached schedules
func Schedules() []*model.AssociationRawData {
	lock.Lock()
	defer lock.Unlock()
	return associations
}
