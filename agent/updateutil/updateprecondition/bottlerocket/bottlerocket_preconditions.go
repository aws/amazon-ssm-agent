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

package bottlerocketprecondition

import (
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/context"
)

// New returns an instance of the static pie preconditons to verify the host is not Bottlerocket OS
func New(context context.T) *bottlerocketPrecondition {
	return &bottlerocketPrecondition{
		context: context,
	}
}

func (b *bottlerocketPrecondition) GetPreconditionName() string {
	return preconditionName
}

func (b *bottlerocketPrecondition) CheckPrecondition(version string) error {

	b.context.Log().Info("Checking for Bottlerocket")
	err := isNotBottlerocket(b)

	if err != nil {
		return fmt.Errorf("Failed Bottlerocket precondition: %v", err)
	}

	return nil
}
