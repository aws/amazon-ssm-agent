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

// Package processor manage polling of associations, dispatching association to processor
package processor

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	ssmsvc "github.com/aws/amazon-ssm-agent/agent/ssm"
	"github.com/carlescere/scheduler"
)

const (
	name = "Association"
)

// Processor contains the logic for processing association
type Processor struct {
	PollJob *scheduler.Job
	SsmSvc  ssmsvc.Service
	Context context.T
}

// NewAssociationProcessor returns a new Processor with the given context.
func NewAssociationProcessor(context context.T) *Processor {
	assocContext := context.With("[" + name + "]")

	ssmService := ssmsvc.NewService()
	return &Processor{
		Context: assocContext,
		SsmSvc:  ssmService,
	}
}

// ProcessAssociation poll and process all the associations
func (p *Processor) ProcessAssociation() {
	log := p.Context.Log()
	instanceID, err := platform.InstanceID()
	if err != nil {
		assocSvc.ListAssociations(log, p.SsmSvc, instanceID)
	}

}
