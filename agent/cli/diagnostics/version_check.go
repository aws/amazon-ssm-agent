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

package diagnostics

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/cli/cliutil"
	"github.com/aws/amazon-ssm-agent/agent/cli/diagnosticsutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/network"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
	"github.com/aws/amazon-ssm-agent/agent/versionutil"
)

const (
	maxLatestVersionQueryTimeSeconds = 3

	versionCheckStrName                    = "SSM Agent version"
	versionCheckStrFailedGetLatestVersion  = "failed to get latest version from bucket with status: %v"
	versionCheckStrFailedGetCurrentVersion = "Failed to get SSM Agent version: %v"
	versionCheckStrSuccessOnlyVersion      = "SSM Agent version is %v"
	versionCheckStrSuccessIsLatest         = "SSM Agent version is %v which is the latest version"
	versionCheckStrSuccessCurrentNotLatest = "SSM Agent version is %v, latest agent version in %v is %v"
)

type versionRegionResponse struct {
	latestVersion string
	region        string
	err           error
}

type versionQuery struct{}

func (q versionQuery) GetName() string {
	return versionCheckStrName
}

func (versionQuery) GetPriority() int {
	return 9
}

func (versionQuery) getLatestVersion(resChan chan versionRegionResponse) {
	log := log.NewSilentMockLog()
	config := appconfig.DefaultConfig()

	agentIdentity, err := cliutil.GetAgentIdentity()
	// Failed to get identity of the running agent
	if err != nil {
		resChan <- versionRegionResponse{
			"",
			"",
			err,
		}
		return
	}

	region, err := agentIdentity.Region()
	if err != nil {
		resChan <- versionRegionResponse{
			"",
			"",
			err,
		}
		return
	}

	s3Url := updateutil.ResolveAgentReleaseBucketURL(region, agentIdentity)

	// Construct the path to the latest version file
	s3Url += "latest/VERSION"

	tr := network.GetDefaultTransport(log, config)
	client := &http.Client{
		Transport: tr,
		Timeout:   30 * time.Second,
	}
	resp, err := client.Get(s3Url)

	if err != nil {
		resChan <- versionRegionResponse{
			"",
			"",
			err,
		}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		latestVersionBytes, err := ioutil.ReadAll(resp.Body)

		if err != nil {
			resChan <- versionRegionResponse{
				"",
				"",
				err,
			}
			return
		}

		resChan <- versionRegionResponse{
			strings.TrimSpace(string(latestVersionBytes)),
			region,
			nil,
		}
		return
	}

	resChan <- versionRegionResponse{
		"",
		"",
		fmt.Errorf(versionCheckStrFailedGetLatestVersion, resp.StatusCode),
	}
}

func (q versionQuery) Execute() diagnosticsutil.DiagnosticOutput {
	version, err := diagnosticsutil.GetSSMAgentVersion()
	if err != nil {
		return diagnosticsutil.DiagnosticOutput{
			Check:  q.GetName(),
			Status: diagnosticsutil.DiagnosticsStatusFailed,
			Note:   fmt.Sprintf(versionCheckStrFailedGetCurrentVersion, err),
		}
	}

	resChan := make(chan versionRegionResponse, 1)
	go q.getLatestVersion(resChan)

	select {
	case <-time.After(time.Second * maxLatestVersionQueryTimeSeconds):
		// Ignore timeout, latest available version is only informative
		return diagnosticsutil.DiagnosticOutput{
			Check:  q.GetName(),
			Status: diagnosticsutil.DiagnosticsStatusSuccess,
			Note:   fmt.Sprintf(versionCheckStrSuccessOnlyVersion, version),
		}
	case res := <-resChan:
		// Failed to get latest version in s3 bucket
		// ignore error since latest version is only informative
		if res.err != nil {
			return diagnosticsutil.DiagnosticOutput{
				Check:  q.GetName(),
				Status: diagnosticsutil.DiagnosticsStatusSuccess,
				Note:   fmt.Sprintf(versionCheckStrSuccessOnlyVersion, version),
			}
		}

		comp, err := versionutil.VersionCompare(version, res.latestVersion)
		isLatestVersion := err == nil && comp == 0

		if isLatestVersion {
			return diagnosticsutil.DiagnosticOutput{
				Check:  q.GetName(),
				Status: diagnosticsutil.DiagnosticsStatusSuccess,
				Note:   fmt.Sprintf(versionCheckStrSuccessIsLatest, version),
			}
		}

		return diagnosticsutil.DiagnosticOutput{
			Check:  q.GetName(),
			Status: diagnosticsutil.DiagnosticsStatusSuccess,
			Note:   fmt.Sprintf(versionCheckStrSuccessCurrentNotLatest, version, res.region, res.latestVersion),
		}
	}
}

func init() {
	diagnosticsutil.RegisterDiagnosticQuery(versionQuery{})
}
