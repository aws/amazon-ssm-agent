package dockercontainer

import (
	"io"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/mock"
)

type DepMock struct {
	mock.Mock
}

func (m *DepMock) UpdateUtilExeCommandOutput(
	customUpdateExecutionTimeoutInSeconds int,
	log log.T,
	cmd string,
	parameters []string,
	workingDir string,
	outputRoot string,
	stdOut io.Writer,
	stdErr io.Writer,
	usePlatformSpecificCommand bool) (output string, err error) {
	args := m.Called(log, cmd, parameters, workingDir, outputRoot, stdOut, stdErr, usePlatformSpecificCommand)
	return args.String(0), args.Error(1)
}
