package configurecontainers

import (
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/mock"
	"golang.org/x/sys/windows/registry"
)

type DepMock struct {
	mock.Mock
}

func (m *DepMock) FileutilUncompress(src, dest string) error {
	args := m.Called(src, dest)
	return args.Error(0)
}

func (m *DepMock) MakeDirs(destinationDir string) (err error) {
	args := m.Called(destinationDir)
	return args.Error(0)
}

func (m *DepMock) TempDir(dir, prefix string) (name string, err error) {
	args := m.Called(dir, prefix)
	return args.String(0), args.Error(1)
}

func (m *DepMock) RegistryOpenKey(k registry.Key, path string, access uint32) (registry.Key, error) {
	args := m.Called(k, path, access)
	return 42, args.Error(0)
}

func (m *DepMock) RegistryKeySetDWordValue(key registry.Key, name string, value uint32) error {
	args := m.Called(name, value)
	return args.Error(0)
}

func (m *DepMock) RegistryKeyGetStringValue(key registry.Key, name string) (val string, valtype uint32, err error) {
	args := m.Called(name)
	return args.String(0), uint32(args.Int(1)), args.Error(2)
}

func (m *DepMock) UpdateUtilExeCommandOutput(
	customUpdateExecutionTimeoutInSeconds int,
	log log.T,
	cmd string,
	parameters []string,
	workingDir string,
	outputRoot string,
	stdOut string,
	stdErr string,
	usePlatformSpecificCommand bool) (output string, err error) {
	args := m.Called(log, cmd, parameters, workingDir, outputRoot, stdOut, stdErr, usePlatformSpecificCommand)
	return args.String(0), args.Error(1)
}

func (m *DepMock) ArtifactDownload(log log.T, input artifact.DownloadInput) (output artifact.DownloadOutput, err error) {
	args := m.Called(log, input)
	return artifact.DownloadOutput{}, args.Error(1)
}
