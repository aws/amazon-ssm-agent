// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the AWS Customer Agreement (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/agreement/

// Package gatherers contains routines for different types of inventory gatherers
//
// +build darwin freebsd linux netbsd openbsd

package gatherers

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
)

// LoadGatherers loads supported Unix inventory gatherers in memory
func LoadPlatformDependentGatherers(context context.T) Registry {
	log := context.Log()
	var registry = Registry{}
	var names []string

	log.Infof("Supported Unix inventory gatherers : %v", names)

	return registry
}
