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
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/gorhill/cronexpr"
)

const (
	cronExpressionEveryFiveMinutes = "cron(0 0/5 * 1/1 * ? *)"
	expressionTypeCron             = "cron"
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
	RunOnce                     bool
	RunNow                      bool
	ExcludeFromFutureScheduling bool
}

// Update updates new association with old association details
func (newAssoc *InstanceAssociation) Update(oldAssoc *InstanceAssociation) {
	newAssoc.CreateDate = oldAssoc.CreateDate
	newAssoc.NextScheduledDate = oldAssoc.NextScheduledDate
	newAssoc.Expression = oldAssoc.Expression
	newAssoc.ExpressionType = oldAssoc.ExpressionType
	newAssoc.ExcludeFromFutureScheduling = oldAssoc.ExcludeFromFutureScheduling
	newAssoc.RunOnce = oldAssoc.RunOnce
}

// Initialize initializes default values for the given new association
func (newAssoc *InstanceAssociation) Initialize(log log.T, currentTime time.Time) error {

	if newAssoc.Association.ScheduleExpression == nil || *newAssoc.Association.ScheduleExpression == "" {
		newAssoc.Association.ScheduleExpression = aws.String(cronExpressionEveryFiveMinutes)
		// legacy association, run only once
		newAssoc.RunOnce = true
	}

	if newAssoc.RunNow {
		newAssoc.NextScheduledDate = currentTime
		return nil
	}

	if err := parseExpression(log, newAssoc); err != nil {
		return fmt.Errorf("Failed to parse schedule expression %v, %v", *newAssoc.Association.ScheduleExpression, err)
	}

	if _, err := cronexpr.Parse(newAssoc.Expression); err != nil {
		return fmt.Errorf("Failed to parse schedule expression %v, %v", newAssoc.Expression, err)
	}

	newAssoc.NextScheduledDate = cronexpr.MustParse(newAssoc.Expression).Next(currentTime)
	return nil
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
