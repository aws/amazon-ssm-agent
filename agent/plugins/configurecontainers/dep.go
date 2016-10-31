package configurecontainers

import (
	"io/ioutil"

	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
)

type dependencies interface {
	MakeDirs(destinationDir string) (err error)
	TempDir(dir, prefix string) (name string, err error)
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
	ArtifactDownload(log log.T, input artifact.DownloadInput) (output artifact.DownloadOutput, err error)
}

type deps struct{}

func (deps) MakeDirs(destinationDir string) (err error) {
	return fileutil.MakeDirs(destinationDir)
}
func (deps) TempDir(dir, prefix string) (name string, err error) {
	return ioutil.TempDir(dir, prefix)
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

func (deps) ArtifactDownload(log log.T, input artifact.DownloadInput) (output artifact.DownloadOutput, err error) {
	return artifact.Download(log, input)
}
