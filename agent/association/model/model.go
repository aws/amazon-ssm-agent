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

// Package model provides model definition for association
package model

import (
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/gorhill/cronexpr"
)

var cronExpressionEveryFiveMinutes = "*/5 * * * *"

// AssociationRawData represents detail information of association
type AssociationRawData struct {
	CreateDate                  time.Time
	NextScheduledDate           time.Time
	Association                 *ssm.InstanceAssociationSummary
	Document                    *string
	RunOnce                     bool
	ExcludeFromFutureScheduling bool
}

// Update updates new association with old association details
func (newAssoc *AssociationRawData) Update(oldAssoc *AssociationRawData) {
	newAssoc.CreateDate = oldAssoc.CreateDate
	newAssoc.NextScheduledDate = oldAssoc.NextScheduledDate
	newAssoc.Association.ScheduleExpression = oldAssoc.Association.ScheduleExpression
}

// Initialize initializes default values for the given new association
func (newAssoc *AssociationRawData) Initialize(log log.T, currentTime time.Time) {
	newAssoc.CreateDate = currentTime

	if newAssoc.Association.ScheduleExpression == nil || *newAssoc.Association.ScheduleExpression == "" {
		newAssoc.Association.ScheduleExpression = aws.String(cronExpressionEveryFiveMinutes)
		// legacy association, run only once
		newAssoc.RunOnce = true
	}

	if _, err := cronexpr.Parse(*newAssoc.Association.ScheduleExpression); err != nil {
		log.Errorf("Failed to parse schedule expression %v, %v", *(newAssoc.Association.ScheduleExpression), err)

		//this line is needed due to a service bug, we can remove it once it's addressed
		newAssoc.Association.ScheduleExpression = aws.String(cronExpressionEveryFiveMinutes)
		//newAssoc.ExcludeFromFutureScheduling = true
	}

	newAssoc.NextScheduledDate = cronexpr.MustParse(*newAssoc.Association.ScheduleExpression).Next(currentTime)
}
