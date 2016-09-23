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

// Package application contains a dummy gatherer.

package application

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/inventory/model"
)

//TODO: add unit tests.

const (
	Name = "AWS:Application"
)

type T struct{}

func Gatherer(context context.T) (*T, error) {
	return new(T), nil
}

func (t *T) Name() string {
	return Name
}

func (t *T) Run(context context.T, configuration inventory.Config) (inventory.Item, error) {
	var result inventory.Item
	var err error
	return result, err
}

func (t *T) RequestStop(stopType contracts.StopType) error {
	var err error
	return err
}
