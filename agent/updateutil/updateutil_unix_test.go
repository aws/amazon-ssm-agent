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

//go:build freebsd || linux || netbsd || openbsd || darwin
// +build freebsd linux netbsd openbsd darwin

package updateutil

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateinfo"
	updateinfomocks "github.com/aws/amazon-ssm-agent/agent/updateutil/updateinfo/mocks"
	"github.com/aws/amazon-ssm-agent/core/executor"
	executormocks "github.com/aws/amazon-ssm-agent/core/executor/mocks"
	"github.com/stretchr/testify/assert"
)

func TestIsServiceRunning(t *testing.T) {
	infoRedHat65 := &updateinfomocks.T{}
	infoRedHat65.On("GetPlatform").Return(updateconstants.PlatformRedHat)
	infoRedHat65.On("IsPlatformUsingSystemD").Return(false, nil)
	infoRedHat65.On("IsPlatformDarwin").Return(false)

	infoRedHat71 := &updateinfomocks.T{}
	infoRedHat71.On("GetPlatform").Return(updateconstants.PlatformRedHat)
	infoRedHat71.On("IsPlatformUsingSystemD").Return(true, nil)
	infoRedHat71.On("IsPlatformDarwin").Return(false)

	infoDarwin := &updateinfomocks.T{}
	infoDarwin.On("GetPlatform").Return(updateconstants.PlatformDarwin)
	infoDarwin.On("IsPlatformUsingSystemD").Return(false, nil)
	infoDarwin.On("IsPlatformDarwin").Return(true)

	mock := &executormocks.IExecutor{}
	mock.On("Processes").Return([]executor.OsProcess{{Executable: updateconstants.DarwinBinaryPath}}, nil)

	util := Utility{
		ProcessExecutor: mock,
		Context:         context.NewMockDefault(),
	}
	testCases := []struct {
		info   updateinfo.T
		result bool
	}{
		// test system with upstart
		{infoRedHat65, true},
		// test system with systemD
		{infoRedHat71, true},
		// test system for mac os
		{infoDarwin, true},
	}

	// Stub exec.Command
	execCommand = fakeExecCommand

	for _, test := range testCases {
		fmt.Printf("Testing %s\n", test.info.GetPlatform())
		result, _ := util.IsServiceRunning(logger, test.info)
		assert.Equal(t, result, test.result)
	}
}

func TestIsServiceRunningWithErrorMessageFromCommandExec(t *testing.T) {
	infoRedHat65 := &updateinfomocks.T{}
	infoRedHat65.On("GetPlatform").Return(updateconstants.PlatformRedHat)
	infoRedHat65.On("IsPlatformUsingSystemD").Return(false, nil)
	infoRedHat65.On("IsPlatformDarwin").Return(false)

	infoRedHat71 := &updateinfomocks.T{}
	infoRedHat71.On("GetPlatform").Return(updateconstants.PlatformRedHat)
	infoRedHat71.On("IsPlatformUsingSystemD").Return(true, nil)
	infoRedHat71.On("IsPlatformDarwin").Return(false)

	infoDarwin := &updateinfomocks.T{}
	infoDarwin.On("GetPlatform").Return(updateconstants.PlatformDarwin)
	infoDarwin.On("IsPlatformUsingSystemD").Return(false, nil)
	infoDarwin.On("IsPlatformDarwin").Return(true)

	mock := &executormocks.IExecutor{}
	mock.On("Processes").Return(nil, fmt.Errorf("SomeError"))
	util := Utility{
		ProcessExecutor: mock,
		Context:         context.NewMockDefault(),
	}
	testCases := []struct {
		info updateinfo.T
	}{
		// test system with upstart
		{infoRedHat65},
		// test system with systemD
		{infoRedHat71},
		// test system for mac os
		{infoDarwin},
	}

	// Stub exec.Command
	execCommand = fakeExecCommandWithError

	for _, test := range testCases {
		fmt.Printf("Testing %s\n", test.info.GetPlatform())
		_, err := util.IsServiceRunning(logger, test.info)
		assert.Error(t, err)
	}
}
