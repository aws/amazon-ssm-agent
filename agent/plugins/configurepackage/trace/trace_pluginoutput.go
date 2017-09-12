package trace

import (
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/log"
)

type PluginOutputTrace struct {
	tracer   Tracer
	exitCode int
	status   contracts.ResultStatus
}

// Getter/Setter

func (po *PluginOutputTrace) GetStatus() contracts.ResultStatus { return po.status }
func (po *PluginOutputTrace) GetExitCode() int                  { return po.exitCode }
func (po *PluginOutputTrace) GetStdout() string                 { return po.tracer.ToPluginOutput().Stdout }
func (po *PluginOutputTrace) GetStderr() string                 { return po.tracer.ToPluginOutput().Stderr }

func (po *PluginOutputTrace) SetStatus(status contracts.ResultStatus) { po.status = status }
func (po *PluginOutputTrace) SetExitCode(exitCode int)                { po.exitCode = exitCode }

// Compatibilty functions with Plugin Output

func (po *PluginOutputTrace) MarkAsFailed(log log.T, err error) {
	// Update the error exit code
	if po.exitCode == 0 {
		po.exitCode = 1
	}
	po.status = contracts.ResultStatusFailed

	po.tracer.CurrentTrace().Error = err
}

func (out *PluginOutputTrace) MarkAsSucceeded() {
	out.exitCode = 0
	out.status = contracts.ResultStatusSuccess
}

func (out *PluginOutputTrace) MarkAsInProgress() {
	out.exitCode = 0
	out.status = contracts.ResultStatusInProgress
}

func (out *PluginOutputTrace) MarkAsSuccessWithReboot() {
	out.exitCode = 0
	out.status = contracts.ResultStatusSuccessAndReboot
}

func (out *PluginOutputTrace) MarkAsCancelled() {
	out.exitCode = 1
	out.status = contracts.ResultStatusCancelled
}

func (out *PluginOutputTrace) MarkAsShutdown() {
	out.exitCode = 1
	out.status = contracts.ResultStatusCancelled
}

func (out *PluginOutputTrace) String() string {
	p := out.tracer.ToPluginOutput()
	return contracts.TruncateOutput(p.Stdout, p.Stderr, contracts.MaximumPluginOutputSize)
}

// Forward to tracer

func (po *PluginOutputTrace) AppendInfo(log log.T, message string) {
	po.tracer.CurrentTrace().AppendInfo(message)
}

func (po *PluginOutputTrace) AppendInfof(log log.T, format string, params ...interface{}) {
	po.tracer.CurrentTrace().AppendInfof(format, params...)
}

func (po *PluginOutputTrace) AppendError(log log.T, message string) {
	po.tracer.CurrentTrace().AppendError(message)
}

func (po *PluginOutputTrace) AppendErrorf(log log.T, format string, params ...interface{}) {
	po.tracer.CurrentTrace().AppendErrorf(format, params...)
}
