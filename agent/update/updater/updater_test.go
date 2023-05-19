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

// Package main represents the entry point of the ssm agent updater.
package main

import (
	"fmt"
	"os"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/update/processor"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
	"github.com/aws/amazon-ssm-agent/common/identity"
	identity2 "github.com/aws/amazon-ssm-agent/common/identity/identity"
	identityMocks "github.com/aws/amazon-ssm-agent/common/identity/mocks"
	"github.com/stretchr/testify/assert"
)

var updateCommand = []string{"updater", "-" + updateconstants.UpdateCmd,
	"-" + updateconstants.SourceVersionCmd, "1.0.0.0", "-" + updateconstants.SourceLocationCmd, "http://source",
	"-" + updateconstants.TargetVersionCmd, "5.0.0.0", "-" + updateconstants.TargetLocationCmd, "http://target"}

type stubUpdater struct {
	returnUpdateError  bool
	returnCleanupError bool
}

func (u *stubUpdater) StartOrResumeUpdate(log logger.T, updateDetail *processor.UpdateDetail) (err error) {
	if u.returnUpdateError {
		return fmt.Errorf("Fail update")
	}
	return nil
}

func (u *stubUpdater) InitializeUpdate(log logger.T, updateDetail *processor.UpdateDetail) (err error) {
	return nil
}

func (u *stubUpdater) CleanupUpdate(log logger.T, updateDetail *processor.UpdateDetail) (err error) {
	if u.returnCleanupError {
		return fmt.Errorf("Cleanup update failed.")
	}
	return nil
}

func (u *stubUpdater) Failed(
	updateDetail *processor.UpdateDetail,
	log logger.T,
	code updateconstants.ErrorCode,
	errMessage string,
	noRollbackMessage bool) (err error) {
	return nil
}

func TestUpdater(t *testing.T) {
	// setup
	updater = &stubUpdater{}
	newAgentIdentity = func(logger.T, *appconfig.SsmagentConfig, identity2.IAgentIdentitySelector) (identity.IAgentIdentity, error) {
		return identityMocks.NewDefaultMockAgentIdentity(), nil
	}

	os.Args = updateCommand

	// action
	updateAgent()
}

func TestUpdaterFailedStartOrResume(t *testing.T) {
	// setup
	updater = &stubUpdater{returnUpdateError: true}
	newAgentIdentity = func(logger.T, *appconfig.SsmagentConfig, identity2.IAgentIdentitySelector) (identity.IAgentIdentity, error) {
		return identityMocks.NewDefaultMockAgentIdentity(), nil
	}

	os.Args = updateCommand

	// action
	updateAgent()
}

func TestUpdaterWithDowngrade(t *testing.T) {
	// setup
	updater = &stubUpdater{returnUpdateError: true}
	newAgentIdentity = func(logger.T, *appconfig.SsmagentConfig, identity2.IAgentIdentitySelector) (identity.IAgentIdentity, error) {
		return identityMocks.NewDefaultMockAgentIdentity(), nil
	}

	os.Args = []string{"updater", "-" + updateconstants.UpdateCmd,
		"-" + updateconstants.SourceVersionCmd, "5.0.0.0", "-" + updateconstants.SourceLocationCmd, "http://source",
		"-" + updateconstants.TargetVersionCmd, "1.0.0.0", "-" + updateconstants.TargetLocationCmd, "http://target"}

	// action
	updateAgent()

	// assert
	assert.Equal(t, *sourceVersion, "5.0.0.0")
	assert.Equal(t, *targetVersion, "1.0.0.0")
}

func TestUpdaterFailedWithoutSourceTargetCmd(t *testing.T) {
	// setup
	updater = &stubUpdater{returnUpdateError: true}
	newAgentIdentity = func(logger.T, *appconfig.SsmagentConfig, identity2.IAgentIdentitySelector) (identity.IAgentIdentity, error) {
		return identityMocks.NewDefaultMockAgentIdentity(), nil
	}

	os.Args = []string{"updater", "-" + updateconstants.UpdateCmd,
		"-" + updateconstants.SourceVersionCmd, "", "-" + updateconstants.SourceLocationCmd, "http://source",
		"-" + updateconstants.TargetVersionCmd, "", "-" + updateconstants.TargetLocationCmd, "http://target"}

	// action
	updateAgent()

	// assert
	assert.Equal(t, *update, true)
	assert.Empty(t, *sourceVersion)
	assert.Empty(t, *targetVersion)
}

func TestCleanupFailed(t *testing.T) {
	// setup
	updater = &stubUpdater{returnCleanupError: true}
	newAgentIdentity = func(logger.T, *appconfig.SsmagentConfig, identity2.IAgentIdentitySelector) (identity.IAgentIdentity, error) {
		return identityMocks.NewDefaultMockAgentIdentity(), nil
	}

	os.Args = updateCommand

	// action
	updateAgent()

}
