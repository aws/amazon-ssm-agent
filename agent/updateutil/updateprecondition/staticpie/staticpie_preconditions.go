// Copyright 2021 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package staticpieprecondition

import (
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/versionutil"
)

// New returns an instance of the static pie preconditons to verify kernel version >= 3.2 for agent version >= 3.1
func New(context context.T) *staticpiePrecondition {
	return &staticpiePrecondition{
		context: context,
	}
}

func (s *staticpiePrecondition) GetPreconditionName() string {
	return preconditionName
}

func (s *staticpiePrecondition) CheckPrecondition(version string) error {

	comp, err := versionutil.VersionCompare(version, preconditionStartAgentVersion)

	if err != nil {
		s.context.Log().Errorf("Failed to compare version %s & %s: %v", version, preconditionStartAgentVersion, err)
		return fmt.Errorf("Failed kernel version precondition: %v", err)
	}

	// if version is lower than first precondition applicable version, pass
	if comp < 0 {
		s.context.Log().Debugf("Not checking static pie precondition, version is below precondition start version")
		return nil
	}

	s.context.Log().Info("Checking kernel version")
	err = hasValidKernelVersion()

	if err != nil {
		return fmt.Errorf("Failed kernel version precondition: %v", err)
	}

	return nil
}
