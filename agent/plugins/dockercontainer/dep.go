package dockercontainer

import (
	"io"

	"github.com/aws/amazon-ssm-agent/agent/context"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
)

var dep dependencies = &deps{}

type dependencies interface {
	UpdateUtilExeCommandOutput(
		context context.T,
		customUpdateExecutionTimeoutInSeconds int,
		log log.T,
		cmd string,
		parameters []string,
		workingDir string,
		outputRoot string,
		stdOut io.Writer,
		stdErr io.Writer,
		usePlatformSpecificCommand bool) (output string, err error)
}

type deps struct{}

func (deps) UpdateUtilExeCommandOutput(
	context context.T,
	customUpdateExecutionTimeoutInSeconds int,
	log log.T,
	cmd string,
	parameters []string,
	workingDir string,
	outputRoot string,
	stdOut io.Writer,
	stdErr io.Writer,
	usePlatformSpecificCommand bool) (output string, err error) {
	util := updateutil.Utility{
		Context:                               context,
		CustomUpdateExecutionTimeoutInSeconds: customUpdateExecutionTimeoutInSeconds,
	}
	return util.NewExeCommandOutput(log, cmd, parameters, workingDir, outputRoot, stdOut, stdErr, usePlatformSpecificCommand)
}
