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
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/gorhill/cronexpr"
)

const (
	expressionTypeCron = "cron"
)

// InstanceAssociation represents detail information of an association
type InstanceAssociation struct {
	DocumentID                  string
	CreateDate                  time.Time
	NextScheduledDate           time.Time
	Association                 *ssm.InstanceAssociationSummary
	Expression                  string
	ExpressionType              string
	Document                    *string
	LegacyAssociation           bool
	RunNow                      bool
	ExcludeFromFutureScheduling bool
}

// Copy copies new association with old association details
func (newAssoc *InstanceAssociation) Copy(oldAssoc *InstanceAssociation) {
	// It'd be ideal to make associations immutable
	// However, NextScheduledDate will be lost if refresh association happens during apply now
	// The apply now will fail, we will keep the mutation associations as is and keep it minimum
	// This logic will be cleaned during document execution.
	newAssoc.CreateDate = oldAssoc.CreateDate
	newAssoc.NextScheduledDate = oldAssoc.NextScheduledDate
	newAssoc.ExcludeFromFutureScheduling = oldAssoc.ExcludeFromFutureScheduling
	newAssoc.LegacyAssociation = oldAssoc.LegacyAssociation
}

// ParseExpression parses the expression with the given association
func (newAssoc *InstanceAssociation) ParseExpression(log log.T) error {
	if err := parseExpression(log, newAssoc); err != nil {
		return fmt.Errorf("Failed to parse schedule expression %v, %v", *newAssoc.Association.ScheduleExpression, err)
	}

	if _, err := cronexpr.Parse(newAssoc.Expression); err != nil {
		return fmt.Errorf("Failed to parse cron expression %v, %v", newAssoc.Expression, err)
	}

	return nil
}

// SetNextScheduledDate initializes default values for the given new association
func (newAssoc *InstanceAssociation) SetNextScheduledDate() {
	currentTime := time.Now().UTC()

	// for all the association that doesn't have lastExecutionDate it will run now
	if newAssoc.RunNow {
		newAssoc.NextScheduledDate = currentTime
		newAssoc.RunNow = false
		return
	}

	newAssoc.NextScheduledDate = cronexpr.MustParse(newAssoc.Expression).Next(newAssoc.Association.LastExecutionDate.UTC()).UTC()
}

func parseExpression(log log.T, assoc *InstanceAssociation) error {
	expression := *assoc.Association.ScheduleExpression

	if strings.HasPrefix(expression, expressionTypeCron) {
		assoc.ExpressionType = expressionTypeCron
		assoc.Expression = expression[len(expressionTypeCron)+1 : len(expression)-1]
		return nil
	}

	return fmt.Errorf("unkonw expression type")
}
