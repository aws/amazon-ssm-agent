package configurecontainers

import (
	"io/ioutil"

	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
	"golang.org/x/sys/windows/registry"
)

type dependencies interface {
	MakeDirs(destinationDir string) (err error)
	TempDir(dir, prefix string) (name string, err error)
	RegistryOpenKey(k registry.Key, path string, access uint32) (registry.Key, error)
	RegistryKeySetDWordValue(key registry.Key, name string, value uint32) error
	RegistryKeyGetStringValue(key registry.Key, name string) (val string, valtype uint32, err error)
	UpdateUtilExeCommandOutput(
		customUpdateExecutionTimeoutInSeconds int,
		log log.T,
		cmd string,
		parameters []string,
		workingDir string,
		outputRoot string,
		stdOut string,
		stdErr string,
		usePlatformSpecificCommand bool) (output string, err error)
	ArtifaceDownload(log log.T, input artifact.DownloadInput) (output artifact.DownloadOutput, err error)
}

type deps struct{}

func (deps) MakeDirs(destinationDir string) (err error) {
	return fileutil.MakeDirs(destinationDir)
}
func (deps) TempDir(dir, prefix string) (name string, err error) {
	return ioutil.TempDir(dir, prefix)
}

//func CreateKey(k registry.Key, path string, access uint32) (newk registry.Key, openedExisting bool, err error)
func (deps) RegistryOpenKey(k registry.Key, path string, access uint32) (registry.Key, error) {
	return registry.OpenKey(k, path, access)
}
func (deps) RegistryKeySetDWordValue(key registry.Key, name string, value uint32) error {
	return key.SetDWordValue(name, value)
}
func (deps) RegistryKeyGetStringValue(key registry.Key, name string) (val string, valtype uint32, err error) {
	return key.GetStringValue(name)
}

func (deps) UpdateUtilExeCommandOutput(
	customUpdateExecutionTimeoutInSeconds int,
	log log.T,
	cmd string,
	parameters []string,
	workingDir string,
	outputRoot string,
	stdOut string,
	stdErr string,
	usePlatformSpecificCommand bool) (output string, err error) {
	util := updateutil.Utility{CustomUpdateExecutionTimeoutInSeconds: customUpdateExecutionTimeoutInSeconds}
	return util.ExeCommandOutput(log, cmd, parameters, workingDir, outputRoot, stdOut, stdErr, usePlatformSpecificCommand)
}

func (deps) ArtifaceDownload(log log.T, input artifact.DownloadInput) (output artifact.DownloadOutput, err error) {
	return artifact.Download(log, input)
}
