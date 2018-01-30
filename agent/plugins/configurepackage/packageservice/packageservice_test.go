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

package packageservice

import (
	"errors"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/trace"

	"github.com/stretchr/testify/assert"
)

var loggerMock = log.NewMockLog()

func TestPackageServiceTrace(t *testing.T) {
	tracer := trace.NewTracer(loggerMock)
	tracea := tracer.BeginSection("traceA")
	tracer.BeginSection("traceB").WithError(errors.New("testerror")).End()
	tracea.WithExitcode(42).End()
	tracer.AddTrace(&trace.Trace{Operation: "traceC"})
	tracer.AddTrace(&trace.Trace{Operation: "traceD", Error: "testerror2"})

	traces := ConvertToPackageServiceTrace(tracer.Traces())

	assert.Equal(t, 6, len(traces))
	assert.Equal(t, "> traceA", traces[0].Operation)
	assert.Equal(t, "> traceB", traces[1].Operation)
	assert.Equal(t, "< traceB (err `testerror`)", traces[2].Operation)
	assert.Equal(t, "< traceA", traces[3].Operation)
	assert.Equal(t, "= traceC", traces[4].Operation)
	assert.Equal(t, "= traceD (err `testerror2`)", traces[5].Operation)
}
