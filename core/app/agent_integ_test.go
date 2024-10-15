// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

//go:build integration
// +build integration

package app

import (
	logPkg "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/identity"
	identity2 "github.com/aws/amazon-ssm-agent/common/identity/identity"
	"github.com/aws/amazon-ssm-agent/core/app/registrar"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/agent/ssm/authregister"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/ec2"
	ctxMocks "github.com/aws/amazon-ssm-agent/core/app/context/mocks"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/assert"
)

var testAgentIdentity identity.IAgentIdentityInner

type ec2IdentitySelector struct{}

func (s *ec2IdentitySelector) SelectAgentIdentity([]identity.IAgentIdentityInner, string) identity.IAgentIdentityInner {
	return testAgentIdentity
}

func TestRetryableRegistrar_RegisterWithRetry_WhenIMDSUnavailable_CancelsSuccessfully(t *testing.T) {
	// Arrange
	log := log.NewMockLog()
	awsConfig := &aws.Config{}
	awsConfig = awsConfig.WithMaxRetries(3).
		WithEndpoint("www.google.com:81").                       // Endpoint is unreachable which causes timeout
		WithHTTPClient(&http.Client{Timeout: time.Second * 10}). // Decrease timeout from http default for test efficiency
		WithEC2MetadataDisableTimeoutOverride(false)
	os.Setenv("AWS_EC2_METADATA_DISABLED", "false")
	sess, _ := session.NewSession(awsConfig)
	imdsClient := ec2metadata.New(sess)

	config := appconfig.SsmagentConfig{
		Ssm: appconfig.SsmCfg{},
		Agent: appconfig.AgentInfo{
			ContainerMode: false,
		},
		Identity: appconfig.IdentityCfg{
			ConsumptionOrder: []string{"EC2"},
		},
	}

	authRegisterService := authregister.NewClientWithConfig(log, config, imdsClient, *awsConfig)
	testAgentIdentity = &ec2.Identity{
		Log:                 log,
		Client:              imdsClient,
		Config:              &config,
		AuthRegisterService: authRegisterService,
	}

	var generators = map[string]identity2.CreateIdentityFunc{
		"EC2": func(t logPkg.T, config *appconfig.SsmagentConfig) []identity.IAgentIdentityInner {
			return []identity.IAgentIdentityInner{testAgentIdentity}
		},
	}

	identity2.SetIdentityGenerators(generators)
	ec2Identity, err := identity2.NewAgentIdentity(log, &config, &ec2IdentitySelector{})
	assert.NoError(t, err, "Failed to create new ec2 agent identity")

	agentContext := &ctxMocks.ICoreAgentContext{}
	agentContext.On("Log").Return(log)
	agentContext.On("Identity").Return(ec2Identity)

	rr := registrar.NewRetryableRegistrar(agentContext)

	statusComm := &contracts.StatusComm{
		TerminationChan: make(chan struct{}, 1),
		DoneChan:        make(chan struct{}, 1),
	}

	coreAgent := &SSMCoreAgent{
		context:        agentContext,
		container:      nil,
		selfupdate:     nil,
		credsRefresher: nil,
		registrar:      rr,
	}

	// Act
	complete := make(chan struct{})
	go func() {
		coreAgent.Start(statusComm)
		complete <- struct{}{}
		close(complete)
	}()

	<-time.After(2 * time.Second)

	// Act
	statusComm.TerminationChan <- struct{}{}

	// Assert
	select {
	case <-complete:
	case <-time.After(1 * time.Second):
		assert.Fail(t, "Test did not complete in allotted time")
	}

	// Assert
	select {
	case <-statusComm.DoneChan:
		// Core agent exited cleanly
	default:
		assert.Fail(t, "Core agent did not exit cleanly in time")
	}
}
