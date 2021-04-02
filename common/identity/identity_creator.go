// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
package identity

import (
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/ec2"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/ecs"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/onprem"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
)

func init() {
	allIdentityGenerators = make(map[string]createIdentityFunc)

	allIdentityGenerators[ec2.IdentityType] = newEC2Identity
	allIdentityGenerators[ecs.IdentityType] = newECSIdentity
	allIdentityGenerators[onprem.IdentityType] = newOnPremIdentity
}

func newEC2Identity(log log.T, _ *appconfig.SsmagentConfig) []IAgentIdentityInner {
	awsConfig := &aws.Config{}
	awsConfig = awsConfig.WithMaxRetries(3)
	sess, _ := session.NewSession(awsConfig)

	return []IAgentIdentityInner{&ec2.Identity{
		Log:    log,
		Client: ec2metadata.New(sess),
	}}
}

func newECSIdentity(log log.T, _ *appconfig.SsmagentConfig) []IAgentIdentityInner {
	return []IAgentIdentityInner{&ecs.Identity{
		Log: log,
	}}
}

func newOnPremIdentity(log log.T, config *appconfig.SsmagentConfig) []IAgentIdentityInner {
	return []IAgentIdentityInner{&onprem.Identity{
		Log:    log,
		Config: config,
	}}
}
