// Copyright 2021 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.

//go:build freebsd || linux || netbsd || openbsd || darwin || windows
// +build freebsd linux netbsd openbsd darwin windows

package diagnostics

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/cli/diagnosticsutil"
	"github.com/aws/amazon-ssm-agent/agent/log/logger"
	"github.com/aws/amazon-ssm-agent/agent/network"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
)

const (
	maxTimeCallMetadataSeconds = 3

	metadataCheckStrName              = "EC2 IMDS"
	metadataCheckStrInstanceIdTimeout = "instance id request timeout"
	metadataCheckStrInstanceIdFailure = "failed to query instance id from IMDS: %v"
	metadataCheckStrRegionTimeout     = "region request timeout"
	metadataCheckStrRegionFailure     = "failed to query region from IMDS: %v"
	metadataCheckStrHasHybrid         = "Instance has hybrid registration"
	metadataCheckStrIMDSTimeout       = "IMDS request timeout"
	metadataCheckStrIMDSFailure       = "Failed to query IMDS: %v"
	metadataCheckStrSuccess           = "IMDS is accessible and has instance id %s in region %s"
)

type stringStringErrorTuple struct {
	firstStr  string
	secondStr string
	err       error
}

type metadataCheckQuery struct{}

func (q metadataCheckQuery) GetName() string {
	return metadataCheckStrName
}

func (metadataCheckQuery) GetPriority() int {
	return 1
}

func (metadataCheckQuery) getRegionAndInstanceId(resChan chan stringStringErrorTuple) {
	log := logger.NewSilentLogger()
	config := appconfig.DefaultConfig()

	tr := network.GetDefaultTransport(log, config)
	awsConfig := aws.Config{
		// IMDS ignores RetryMaxAttempts; it only honors a Retryer func from the
		// config. Set the max attempts via a Retryer so retries are bounded.
		Retryer: func() aws.Retryer {
			return retry.NewStandard(func(o *retry.StandardOptions) {
				o.MaxAttempts = 2
			})
		},
		HTTPClient: &http.Client{
			Transport: tr,
		},
	}

	client := imds.NewFromConfig(awsConfig, func(options *imds.Options) {
		options.DisableDefaultTimeout = true
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	getMDInput := &imds.GetMetadataInput{Path: "instance-id"}
	getMDOutput, err := client.GetMetadata(ctx, getMDInput)
	if err != nil {
		var reqCanceledErr *aws.RequestCanceledError
		if errors.As(err, &reqCanceledErr) {
			resChan <- stringStringErrorTuple{
				"",
				"",
				fmt.Errorf(metadataCheckStrInstanceIdTimeout),
			}
			return
		}

		resChan <- stringStringErrorTuple{
			"",
			"",
			fmt.Errorf(metadataCheckStrInstanceIdFailure, err),
		}
		return
	}

	rc := getMDOutput.Content
	defer rc.Close()
	bytes, err := io.ReadAll(rc)
	if err != nil {
		resChan <- stringStringErrorTuple{
			"",
			"",
			fmt.Errorf(metadataCheckStrInstanceIdFailure, err),
		}
		return
	}
	instanceId := string(bytes)

	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()
	getRegionInput := &imds.GetRegionInput{}
	region, err := client.GetRegion(ctx2, getRegionInput)
	if err != nil {
		var reqCanceledErr *aws.RequestCanceledError
		if errors.As(err, &reqCanceledErr) {
			resChan <- stringStringErrorTuple{
				"",
				"",
				fmt.Errorf(metadataCheckStrRegionTimeout),
			}
			return
		}
		resChan <- stringStringErrorTuple{
			"",
			"",
			fmt.Errorf(metadataCheckStrRegionFailure, err),
		}
		return
	}

	resChan <- stringStringErrorTuple{
		instanceId,
		region.Region,
		nil,
	}
}

func (q metadataCheckQuery) Execute() diagnosticsutil.DiagnosticOutput {
	if diagnosticsutil.IsOnPremRegistration() {
		return diagnosticsutil.DiagnosticOutput{
			Check:  q.GetName(),
			Status: diagnosticsutil.DiagnosticsStatusSkipped,
			Note:   metadataCheckStrHasHybrid,
		}
	}

	resChan := make(chan stringStringErrorTuple, 1)
	go q.getRegionAndInstanceId(resChan)
	select {
	case <-time.After(maxTimeCallMetadataSeconds * time.Second):
		return diagnosticsutil.DiagnosticOutput{
			Check:  q.GetName(),
			Status: diagnosticsutil.DiagnosticsStatusFailed,
			Note:   metadataCheckStrIMDSTimeout,
		}
	case respTouple := <-resChan:
		if respTouple.err != nil {
			return diagnosticsutil.DiagnosticOutput{
				Check:  q.GetName(),
				Status: diagnosticsutil.DiagnosticsStatusFailed,
				Note:   fmt.Sprintf(metadataCheckStrIMDSFailure, respTouple.err),
			}
		}

		return diagnosticsutil.DiagnosticOutput{
			Check:  q.GetName(),
			Status: diagnosticsutil.DiagnosticsStatusSuccess,
			Note:   fmt.Sprintf(metadataCheckStrSuccess, respTouple.firstStr, respTouple.secondStr),
		}
	}
}

func init() {
	diagnosticsutil.RegisterDiagnosticQuery(metadataCheckQuery{})
}
