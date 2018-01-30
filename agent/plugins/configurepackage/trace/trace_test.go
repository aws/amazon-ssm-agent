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
	"errors"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var loggerMock = log.NewMockLog()

type TimeMock struct {
	mock.Mock
}

func (t *TimeMock) NowUnixNano() int64 {
	args := t.Called()
	return int64(args.Int(0))
}

func TestSimpleTrace(t *testing.T) {
	tracer := NewTracer(loggerMock)

	curtrace := tracer.BeginSection("testtracemsg") // start trace
	// -> Do something traceworthy
	curtrace.WithExitcode(3).End() // end trace with exitcode 3

	assert.Equal(t, 1, len(tracer.Traces()))
	assert.Equal(t, "testtracemsg", tracer.Traces()[0].Operation)
	assert.Equal(t, int64(3), tracer.Traces()[0].Exitcode)
}

func TestDefer(t *testing.T) {
	tracer := NewTracer(loggerMock)

	fun := func(tracer Tracer) (err error) {
		// start trace immediately and defer end/error on method exit
		defer tracer.BeginSection("testtracemsg").EndWithError(&err)

		// -> Do something traceworthy which generates a error:
		err = errors.New("asdf")

		return err
	}

	expectedErr := fun(tracer)

	assert.Equal(t, 1, len(tracer.Traces()))
	trace := tracer.Traces()[0]
	assert.Equal(t, "testtracemsg", trace.Operation)
	assert.Equal(t, expectedErr.Error(), trace.Error)
	assert.NotNil(t, trace.Start)
	assert.NotNil(t, trace.Stop)
	assert.True(t, trace.Start < trace.Stop)
}

func TestInvalidEndSection(t *testing.T) {
	tracer := NewTracer(loggerMock)
	// No start time provided
	err := tracer.EndSection(&Trace{Operation: "testfailure"})
	assert.Error(t, err)
}

func TestUnknownTraceEndSection(t *testing.T) {
	tracer := NewTracer(loggerMock)
	err := tracer.EndSection(&Trace{Operation: "testfailure", Start: 42})
	assert.Error(t, err)
}

func TestSkippedTraceEndSection(t *testing.T) {
	tracer := NewTracer(loggerMock)

	tracea := tracer.BeginSection("traceA")
	tracer.BeginSection("traceB")
	tracer.BeginSection("traceC")
	tracea.End()

	assert.Nil(t, tracer.CurrentTrace())
	assert.Equal(t, 3, len(tracer.Traces()))
	assert.Equal(t, "traceC", tracer.Traces()[0].Operation)
	assert.Equal(t, "traceB", tracer.Traces()[1].Operation)
	assert.Equal(t, "traceA", tracer.Traces()[2].Operation)
}

func TestSkippedTraceEndSectionDoubleClose(t *testing.T) {
	tracer := NewTracer(loggerMock)

	traceA := tracer.BeginSection("traceA")
	traceB := tracer.BeginSection("traceB")
	traceA.End()
	err := traceB.End() // wrong order of close - but no explosion!

	assert.Error(t, err)
}

func TestSubtraces(t *testing.T) {
	tracer := NewTracer(loggerMock)

	tracea := tracer.BeginSection("traceA")
	assert.Equal(t, tracea, tracer.CurrentTrace())

	traceb := tracer.BeginSection("traceB")
	assert.Equal(t, traceb, tracer.CurrentTrace())

	traceb.End()
	assert.Equal(t, tracea, tracer.CurrentTrace())

	tracea.End()
	assert.Nil(t, tracer.CurrentTrace())

	assert.Equal(t, 2, len(tracer.Traces()))
}

func TestAppendOutputInfo(t *testing.T) {
	tracer := NewTracer(loggerMock)

	trace := tracer.BeginSection("tracea")
	trace.AppendInfo("output01")

	assert.Equal(t, "output01\n", tracer.CurrentTrace().InfoOut.String())

	trace.AppendInfo("output02")
	assert.Equal(t, "output01\noutput02\n", tracer.CurrentTrace().InfoOut.String())

	trace.AppendInfof("output0%d", 3)
	assert.Equal(t, "output01\noutput02\noutput03\n", tracer.CurrentTrace().InfoOut.String())
}

func TestAppendOutputError(t *testing.T) {
	tracer := NewTracer(loggerMock)

	trace := tracer.BeginSection("tracea")
	trace.AppendError("output01")

	assert.Equal(t, "output01\n", tracer.CurrentTrace().ErrorOut.String())

	trace.AppendError("output02")
	assert.Equal(t, "output01\noutput02\n", tracer.CurrentTrace().ErrorOut.String())

	trace.AppendErrorf("output0%d", 3)
	assert.Equal(t, "output01\noutput02\noutput03\n", tracer.CurrentTrace().ErrorOut.String())
}

func TestCorrectEndTime(t *testing.T) {
	timemock := &TimeMock{}
	tracer := &TracerImpl{timeProvider: timemock, logger: loggerMock}

	timemock.On("NowUnixNano").Return(42).Once()
	trace := tracer.BeginSection("anothertrace")

	timemock.On("NowUnixNano").Return(142).Once()
	trace.End()

	assert.Equal(t, int64(42), tracer.Traces()[0].Start)
	assert.Equal(t, int64(142), tracer.Traces()[0].Stop)
}

func TestToPluginOutput(t *testing.T) {
	tracer := NewTracer(loggerMock)

	tracea := tracer.BeginSection("traceA")
	tracea.AppendInfo("traceAinfo")
	traceb := tracer.BeginSection("traceB")
	traceb.AppendInfo("traceBinfo")
	traceb.End()
	tracea.End()
	tracec := tracer.BeginSection("traceC")
	tracec.AppendInfo("traceCinfo")
	tracec.End()

	out := tracer.ToPluginOutput()

	assert.Equal(t, "traceBinfo\ntraceAinfo\ntraceCinfo\n", out.GetStdout())
}

func TestAppendWithSubtraces(t *testing.T) {
	tracer := NewTracer(loggerMock)
	traceA := tracer.BeginSection("traceA")

	// TODO: actually emit and extract subtraces
	traceA.AppendWithSubtraces("traceAinfo")

	assert.Equal(t, "traceAinfo\n", tracer.CurrentTrace().InfoOut.String())
}

func TestPrependTraces(t *testing.T) {
	tracer := NewTracer(loggerMock)
	tracer.BeginSection("foo").End()
	tracer.PrependTraces([]*Trace{&Trace{
		Operation: "foo",
		Start:     123,
	}})

	traces := tracer.Traces()
	assert.Len(t, traces, 2)
	assert.Equal(t, "foo", traces[0].Operation)
	assert.Equal(t, int64(123), traces[0].Start)
	assert.Equal(t, traces[1].Tracer, traces[0].Tracer)
	assert.Equal(t, traces[1].Logger, traces[0].Logger)
}
