// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/log"
)

// NanoTime is helper interface for mocking time
type NanoTime interface {
	NowUnixNano() int64
}

type TimeImpl struct {
}

func (t *TimeImpl) NowUnixNano() int64 {
	return time.Now().UnixNano()
}

type Trace struct {
	Tracer Tracer `json:"-"`
	Logger log.T  `json:"-"`

	Operation string
	// results
	Exitcode int64
	Error    string `json:",omitempty"`
	// timing
	Start int64
	Stop  int64 `json:",omitempty"`
	// output
	InfoOut  bytes.Buffer `json:"-"`
	ErrorOut bytes.Buffer `json:"-"`
}

// Tracer is used for collecting traces during a package installation
type Tracer interface {
	BeginSection(message string) *Trace
	EndSection(trace *Trace) error
	AddTrace(trace *Trace)

	Traces() []*Trace
	PrependTraces([]*Trace)
	CurrentTrace() *Trace

	ToPluginOutput() iohandler.IOHandler
}

// TracerImpl implements the Tracer interface for collecting traces
type TracerImpl struct {
	timeProvider NanoTime
	traces       []*Trace
	tracestack   []*Trace
	logger       log.T
}

func NewTracer(logger log.T) Tracer {
	return &TracerImpl{
		timeProvider: &TimeImpl{},
		logger:       logger,
	}
}

// BeginSection will create a new trace and registers with the tracer
func (t *TracerImpl) BeginSection(message string) *Trace {
	t.logger.Debugf("starting with %s", message)

	trace := &Trace{
		Tracer:    t,
		Logger:    t.logger,
		Operation: message,
		Start:     t.timeProvider.NowUnixNano(),
	}
	t.tracestack = append(t.tracestack, trace)

	return trace
}

func logTraceDone(logger log.T, trace *Trace) {
	if trace.Error != "" {
		logger.Errorf("done with %s - error: %s", trace.Operation, trace.Error)
	} else if trace.Exitcode != 0 {
		logger.Errorf("done with %s - exitcode: %d", trace.Operation, trace.Exitcode)
	} else {
		logger.Debugf("done with %s", trace.Operation)
	}
}

func containsTrace(traces []*Trace, trace *Trace) bool {
	for _, x := range traces {
		if x == trace {
			return true
		}
	}
	return false
}

// EndSection will close the trace provided in the parameter.
// If the provided trace is not the upper one on the stack it will close all
// traces in between.
func (t *TracerImpl) EndSection(trace *Trace) error {
	if trace.Start == 0 {
		return errors.New("Trying to end section without start time")
	}
	if !containsTrace(t.tracestack, trace) {
		return errors.New("Provided trace not found")
	}

	logTraceDone(t.logger, trace)

	trace.Stop = t.timeProvider.NowUnixNano()

	l := len(t.tracestack)
	for t.tracestack[l-1] != trace {
		var x *Trace
		x, t.tracestack = t.tracestack[l-1], t.tracestack[:l-1]
		l = len(t.tracestack)

		// Trace not closed correctly - closing now
		x.Stop = t.timeProvider.NowUnixNano()
		t.logger.Tracef("closing skipped trace: %s", x.Operation)
		t.traces = append(t.traces, x)
	}

	// only keep remaining traces
	t.tracestack = t.tracestack[:l-1]

	// appending traces on end should ensure they are sorted by Stop time
	t.traces = append(t.traces, trace)

	return nil
}

// AddTrace takes a one time trace without tracking a duration
func (t *TracerImpl) AddTrace(trace *Trace) {
	logTraceDone(t.logger, trace)

	if trace.Start == 0 {
		trace.Start = t.timeProvider.NowUnixNano()
	}

	t.traces = append(t.traces, trace)
}

// Traces will return all closed traces
func (t *TracerImpl) Traces() []*Trace {
	return t.traces
}

// PrependTraces takes existing traces and add them at the beginning
// while also setting their Tracer and Logger
func (t *TracerImpl) PrependTraces(traces []*Trace) {
	for _, trace := range traces {
		trace.Tracer = t
		trace.Logger = t.logger
	}
	t.traces = append(traces, t.traces...)
}

// CurrentTrace will return the last unclosed trace
// If no trace is open it will return nil
func (t *TracerImpl) CurrentTrace() *Trace {
	if len(t.tracestack) > 0 {
		return t.tracestack[len(t.tracestack)-1]
	} else {
		return nil
	}
}

// ToPluginOutput will convert info and error output into a IOHandler struct
// It will sort the output by trace end time
func (t *TracerImpl) ToPluginOutput() iohandler.IOHandler {
	var out iohandler.DefaultIOHandler
	var infoOut bytes.Buffer
	var errorOut bytes.Buffer

	for _, trace := range t.Traces() {
		infoOut.Write(trace.InfoOut.Bytes())
		errorOut.Write(trace.ErrorOut.Bytes())
		if trace.Error != "" {
			errorOut.WriteString(trace.Error)
			errorOut.WriteString("\n")
		}
	}

	out.SetStdout(infoOut.String())
	out.SetStderr(errorOut.String())

	return &out
}

// Trace

// WithExitcode sets the exitcode of the trace
func (t *Trace) WithExitcode(exitcode int64) *Trace {
	t.Exitcode = exitcode
	return t
}

// WithError sets the error of the trace
func (t *Trace) WithError(err error) *Trace {
	t.Logger.Error(err)
	if err != nil {
		t.Error = err.Error()
	} else {
		t.Error = ""
	}
	return t
}

// End will close the trace. Afterwards no other operation should be called.
func (t *Trace) End() error {
	return t.Tracer.EndSection(t)
}

// EndWithError just combines two commonly used methods to be able to use it in
// combination with defer
//
// func asdf(tracer Tracer) {
//		var err error
//		defer tracer.BeginSection("testtracemsg").EndWithError(err)
//		...
//	}
func (t *Trace) EndWithError(err *error) *Trace {
	t.WithError(*err)
	t.End()
	return t
}

// PluginOutput

// AppendInfo adds info to PluginOutput StandardOut.
func (t *Trace) AppendInfo(message string) *Trace {
	t.Logger.Info(message)
	t.InfoOut.WriteString(message)
	t.InfoOut.WriteString("\n")
	return t
}

// AppendInfof adds info to PluginOutput StandardOut with formatting parameters.
func (t *Trace) AppendInfof(format string, params ...interface{}) *Trace {
	return t.AppendInfo(fmt.Sprintf(format, params...))
}

// AppendDebug adds debug info to the trace
func (t *Trace) AppendDebug(message string) *Trace {
	t.Logger.Debugf(message)
	return t
}

// AppendDebugf adds debug info to the trace
func (t *Trace) AppendDebugf(format string, params ...interface{}) *Trace {
	t.Logger.Debugf(format, params...)
	return t
}

// AppendError adds errors to PluginOutput StandardErr.
func (t *Trace) AppendError(message string) *Trace {
	t.Logger.Error(message)
	t.ErrorOut.WriteString(message)
	t.ErrorOut.WriteString("\n")
	return t
}

// AppendErrorf adds errors to PluginOutput StandardErr with formatting parameters.
func (t *Trace) AppendErrorf(format string, params ...interface{}) *Trace {
	t.AppendError(fmt.Sprintf(format, params...))
	return t
}

// subtraces

func (t *Trace) AppendWithSubtraces(message string) *Trace {
	// TODO: detect subtraces
	return t.AppendInfo(message)
}
