// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.
package trace

import (
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/log"
)

type PluginOutputTrace struct {
	Tracer   Tracer
	exitCode int
	status   contracts.ResultStatus
}

// Getter/Setter

func (po *PluginOutputTrace) GetStatus() contracts.ResultStatus { return po.status }
func (po *PluginOutputTrace) GetExitCode() int                  { return po.exitCode }
func (po *PluginOutputTrace) GetStdout() string                 { return po.Tracer.ToPluginOutput().GetStdout() }
func (po *PluginOutputTrace) GetStderr() string                 { return po.Tracer.ToPluginOutput().GetStderr() }

func (po *PluginOutputTrace) SetStatus(status contracts.ResultStatus) { po.status = status }
func (po *PluginOutputTrace) SetExitCode(exitCode int)                { po.exitCode = exitCode }

// Compatibility functions with Plugin Output

func (po *PluginOutputTrace) MarkAsFailed(log log.T, err error) {
	// Update the error exit code
	if po.exitCode == 0 {
		po.exitCode = 1
	}
	po.status = contracts.ResultStatusFailed

	if err != nil {
		po.Tracer.CurrentTrace().Error = err.Error()
	}
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
	p := out.Tracer.ToPluginOutput()
	return iohandler.TruncateOutput(p.GetStdout(), p.GetStderr(), iohandler.MaximumPluginOutputSize)
}

// Forward to tracer

func (po *PluginOutputTrace) AppendInfo(log log.T, message string) {
	po.Tracer.CurrentTrace().AppendInfo(message)
}

func (po *PluginOutputTrace) AppendInfof(log log.T, format string, params ...interface{}) {
	po.Tracer.CurrentTrace().AppendInfof(format, params...)
}

func (po *PluginOutputTrace) AppendError(log log.T, message string) {
	po.Tracer.CurrentTrace().AppendError(message)
}

func (po *PluginOutputTrace) AppendErrorf(log log.T, format string, params ...interface{}) {
	po.Tracer.CurrentTrace().AppendErrorf(format, params...)
}

func (po *PluginOutputTrace) AppendDebug(log log.T, message string) {
	po.Tracer.CurrentTrace().AppendDebug(message)
}

func (po *PluginOutputTrace) AppendDebugf(log log.T, format string, params ...interface{}) {
	po.Tracer.CurrentTrace().AppendDebugf(format, params...)
}
