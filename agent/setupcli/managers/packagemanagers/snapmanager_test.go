package packagemanagers

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/common/mocks"
	"github.com/stretchr/testify/mock"
)

func TestSnapManager_UninstallAgent(t *testing.T) {
	waitTimeInterval = 500 * time.Millisecond
	errExit10 := fmt.Errorf("snap failed exit 10")
	testCases := []struct {
		testName string
		attempts int
		errors   []error
	}{
		{
			testName: "FailsAfter1Attempt",
			attempts: 1,
			errors:   []error{fmt.Errorf("SomeError")},
		},
		{
			testName: "FailsAfter3Attempts",
			attempts: 3,
			errors:   []error{errExit10, errExit10, fmt.Errorf("SomeError")},
		},
		{
			testName: "SucceedsAfter1Attempt",
			attempts: 1,
			errors:   []error{nil},
		},
		{
			testName: "SucceedsAfter3Attempts",
			attempts: 3,
			errors:   []error{errExit10, errExit10, nil},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			managerHelper := &mocks.IManagerHelper{}
			manager := &snapManager{managerHelper: managerHelper}
			for i := 0; i < tc.attempts; i++ {
				managerHelper.On("RunCommand", "snap", "remove", "amazon-ssm-agent").Return("SomeOutput", tc.errors[i])
				if tc.errors[i] == errExit10 {
					managerHelper.On("IsTimeoutError", tc.errors[i]).Return(false)
					managerHelper.On("IsExitCodeError", tc.errors[i]).Return(true)
					managerHelper.On("GetExitCode", tc.errors[i]).Return(snapAutoRefreshInProgressExitCode)
				} else if tc.errors[i] != nil {
					managerHelper.On("IsTimeoutError", mock.Anything).Return(false)
					managerHelper.On("IsExitCodeError", mock.Anything).Return(true)
					managerHelper.On("GetExitCode", mock.Anything).Return(1)
				}
			}

			manager.UninstallAgent(log.NewMockLog(), "")
			managerHelper.AssertExpectations(t)
		})
	}
}
