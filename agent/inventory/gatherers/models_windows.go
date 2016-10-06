// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the AWS Customer Agreement (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/agreement/

// Package gatherers contains routines for different types of inventory gatherers
//
// +build windows

package gatherers

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/inventory/gatherers/windowsUpdate"
)

// LoadGatherers loads supported Windows inventory gatherers in memory
func LoadPlatformDependentGatherers(context context.T) Registry {
	log := context.Log()
	var registry = Registry{}
	var names []string
	// Load windowsUpdate inventory item gather
	if a, err := windowsUpdate.Gatherer(context); err != nil {
		log.Errorf("Windows update gatherer isn't properly configured - %v", err.Error())
	} else {
		registry[a.Name()] = a
		names = append(names, a.Name())
	}

	log.Infof("Supported Windows inventory gatherers : %v", names)

	return registry
}
