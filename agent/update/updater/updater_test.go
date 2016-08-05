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

	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/update/processor"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
	"github.com/stretchr/testify/assert"
)

var updateCommand = []string{"updater", "-update", "-source.version", "1.0.0.0", "-source.location", "http://source",
	"-target.version", "5.0.0.0", "-target.location", "http://target"}

func regionStub() (string, error) {
	return "us-east-1", nil
}

func regionFailedStub() (string, error) {
	return "", fmt.Errorf("Cannot set region")
}

type stubUpdater struct {
	returnUpdateError bool
}

func (u *stubUpdater) StartOrResumeUpdate(log logger.T, context *processor.UpdateContext) (err error) {
	if u.returnUpdateError {
		return fmt.Errorf("Fail update")
	}
	return nil
}

func (u *stubUpdater) InitializeUpdate(log logger.T, detail *processor.UpdateDetail) (context *processor.UpdateContext, err error) {
	context = &processor.UpdateContext{}
	context.Current = &processor.UpdateDetail{}
	context.Current.StandardOut = "output message"

	return context, nil
}

func (u *stubUpdater) Failed(
	context *processor.UpdateContext,
	log logger.T,
	code updateutil.ErrorCode,
	errMessage string,
	noRollbackMessage bool) (err error) {
	return nil
}

func TestUpdater(t *testing.T) {
	// setup
	log = logger.NewMockLog()
	region = regionStub
	updater = &stubUpdater{}

	os.Args = updateCommand

	// action
	main()
}

func TestUpdaterFailedStartOrResume(t *testing.T) {
	// setup
	log = logger.NewMockLog()
	region = regionStub
	updater = &stubUpdater{returnUpdateError: true}

	os.Args = updateCommand

	// action
	main()
}

func TestUpdaterFailedSetRegion(t *testing.T) {
	// setup
	log = logger.NewMockLog()
	region = regionFailedStub
	updater = &stubUpdater{returnUpdateError: true}

	os.Args = updateCommand

	// action
	main()
}

func TestUpdaterWithDowngrade(t *testing.T) {
	// setup
	log = logger.NewMockLog()
	region = regionStub
	updater = &stubUpdater{returnUpdateError: true}

	os.Args = []string{"updater", "-update", "-source.version", "5.0.0.0", "-source.location", "http://source",
		"-target.version", "1.0.0.0", "-target.location", "http://target"}

	// action
	main()

	// assert
	assert.Equal(t, *sourceVersion, "5.0.0.0")
	assert.Equal(t, *targetVersion, "1.0.0.0")
}

func TestUpdaterFailedWithoutSourceTargetCmd(t *testing.T) {
	// setup
	log = logger.NewMockLog()
	region = regionStub
	updater = &stubUpdater{returnUpdateError: true}

	os.Args = []string{"updater", "-update", "-source.version", "", "-source.location", "http://source",
		"-target.version", "", "-target.location", "http://target"}

	// action
	main()

	// assert
	assert.Equal(t, *update, true)
	assert.Empty(t, *sourceVersion)
	assert.Empty(t, *targetVersion)
}
