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
	"github.com/aws/amazon-ssm-agent/agent/log"
)

// Trace contains one specific operation done for the agent install/upgrade/uninstall
type Trace struct {
	Operation string
	Exitcode  int64
	Timing    int64
}

// PackageResult contains all data collected in one install/upgrade/uninstall and gets reported back to PackageService
type PackageResult struct {
	PackageName string
	Version     string
	Operation   string
	Timing      int64
	Exitcode    int64
	Environment map[string]string
	Trace       map[string]Trace
}

// PackageService is used to determine the latest version and to obtain the local repository content for a given version.
type PackageService interface {
	DownloadManifest(log log.T, packageName string, version string) (string, error)
	DownloadArtifact(log log.T, packageName string, version string) (string, error)
	ReportResult(log log.T, result PackageResult) error
}
