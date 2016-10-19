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

	"github.com/aws/aws-sdk-go/service/ssm"
)

// AssociationRawData represents detail information of association
type AssociationRawData struct {
	CreateDate        time.Time
	NextScheduledDate time.Time
	IsExecuting       bool
	Association       *ssm.InstanceAssociationSummary
	Document          *string
}
