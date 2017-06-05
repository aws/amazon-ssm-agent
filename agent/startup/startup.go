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

// Package startup implements startup plugin processor
package startup

import (
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
)

const (
	name = "StartupProcessor"
)

// Processor is an object that can process startup tasks.
type Processor struct {
	context context.T
}

// NewProcessor creates and returns StartupProcessor object.
func NewProcessor(context context.T) *Processor {
	context.Log().Infof("Create new startup processor")
	startupContext := context.With("[" + name + "]")

	return &Processor{
		context: startupContext,
	}
}

// Name returns the name of the module that executes the startup tasks.
func (p *Processor) ModuleName() string {
	return name
}

// Execute executes the startup tasks and return error if any.
func (p *Processor) ModuleExecute(context context.T) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Internal error occurred by startup processor: %v", r)
		}
	}()

	if p.IsAllowed() {
		err = p.ExecuteTasks()
	}
	return
}

// RequestStop is not necessarily used since startup task only happens once.
func (p *Processor) ModuleRequestStop(stopType contracts.StopType) (err error) {
	return nil
}
