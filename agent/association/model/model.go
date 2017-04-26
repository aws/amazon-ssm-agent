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
	"fmt"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/association/scheduleexpression"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
)

// InstanceAssociation represents detail information of an association
type InstanceAssociation struct {
	DocumentID        string
	CreateDate        time.Time
	NextScheduledDate *time.Time
	Association       *ssm.InstanceAssociationSummary
	ParsedExpression  scheduleexpression.ScheduleExpression
	Document          *string
	Errors            []error
}

// ParseExpression parses the expression with the given association
func (newAssoc *InstanceAssociation) ParseExpression(log log.T) error {

	parsedScheduleExpression, err := scheduleexpression.CreateScheduleExpression(log, *newAssoc.Association.ScheduleExpression)

	if err != nil {
		return fmt.Errorf("Failed to parse schedule expression %v, %v", *newAssoc.Association.ScheduleExpression, err)
	}

	newAssoc.ParsedExpression = parsedScheduleExpression
	return nil
}

// IsRunOnceAssociation return true for the association that doesn't have schedule expression and will run only once
func (assoc *InstanceAssociation) IsRunOnceAssociation() bool {
	return assoc.Association.ScheduleExpression == nil || *assoc.Association.ScheduleExpression == ""
}

// RunNow sets the NextScheduledDate to current time
func (newAssoc *InstanceAssociation) RunNow() {
	newAssoc.NextScheduledDate = aws.Time(time.Now().UTC())
}

// SetNextScheduledDate sets next scheduled date for the given association
func (newAssoc *InstanceAssociation) SetNextScheduledDate(log log.T) {
	// Run association immediately if DetailedStatus is Pending
	if newAssoc.Association.DetailedStatus != nil &&
		*newAssoc.Association.DetailedStatus == contracts.AssociationStatusPending {
		newAssoc.RunNow()
		return
	}

	if newAssoc.IsRunOnceAssociation() {
		if newAssoc.Association.DetailedStatus != nil &&
			*newAssoc.Association.DetailedStatus == contracts.AssociationStatusAssociated {
			// Run association immediately if RunOnceAssociation has not been run before
			newAssoc.RunNow()
		} else {
			log.Infof("Skipping association %v as it has been processed", *newAssoc.Association.Name)
			newAssoc.NextScheduledDate = nil
		}
		return
	}

	// Run association immediately if association has not been run before
	if newAssoc.Association.LastExecutionDate == nil {
		newAssoc.RunNow()
		return
	}

	if newAssoc.ParsedExpression == nil {
		if err := newAssoc.ParseExpression(log); err != nil {
			log.Errorf("Skipping association %v as there was an error parsing schedule expression %v."+
				" Error received while parsing was %v.",
				*newAssoc.Association.AssociationId, *newAssoc.Association.ScheduleExpression, err)

			newAssoc.NextScheduledDate = nil
			return
		}
	}

	// Set next schedule date of association according to it's schedule
	newAssoc.NextScheduledDate = aws.Time(
		newAssoc.ParsedExpression.Next(newAssoc.Association.LastExecutionDate.UTC()).UTC())
	log.Infof("Based upon expression %v and last execution date %v, next scheduled date for association %v is %v",
		*newAssoc.Association.ScheduleExpression, times.ToIsoDashUTC(*newAssoc.Association.LastExecutionDate),
		*newAssoc.Association.AssociationId, times.ToIsoDashUTC(*newAssoc.NextScheduledDate))
}
