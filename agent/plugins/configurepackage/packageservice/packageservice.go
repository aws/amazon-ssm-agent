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
	"fmt"
	"sort"

	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/trace"
)

// Trace contains one specific operation done for the agent install/upgrade/uninstall
type Trace struct {
	Operation string
	Exitcode  int64
	Timing    int64
}

// PackageResult contains all data collected in one install/upgrade/uninstall and gets reported back to PackageService
type PackageResult struct {
	PackageName            string
	Version                string
	PreviousPackageVersion string
	Operation              string
	Timing                 int64
	Exitcode               int64
	Environment            map[string]string
	Trace                  []*Trace
}

// PackageService is used to determine the latest version and to obtain the local repository content for a given version.
type PackageService interface {
	PackageServiceName() string
	GetPackageArnAndVersion(packageName string, version string) (string, string)
	DownloadManifest(tracer trace.Tracer, packageName string, version string) (string, string, bool, error)
	DownloadArtifact(tracer trace.Tracer, packageName string, version string) (string, error)
	ReportResult(tracer trace.Tracer, result PackageResult) error
}

const (
	PackageServiceName_ssms3       = "ssms3"
	PackageServiceName_birdwatcher = "birdwatcherUsingBirdwatcherArchive"
	PackageServiceName_document    = "birdwatcherUsingDocumentArchive"
)

// ByTiming implements sort.Interface for []*packageservice.Trace based on the
// Timing field.
type ByTiming []*Trace

func (a ByTiming) Len() int           { return len(a) }
func (a ByTiming) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByTiming) Less(i, j int) bool { return a[i].Timing < a[j].Timing }

// ConvertToPackageServiceTrace will return traces compatible with PackageService
func ConvertToPackageServiceTrace(traces []*trace.Trace) []*Trace {
	pkgtraces := []*Trace{}

	for _, trace := range traces {
		exitcode := trace.Exitcode
		if exitcode == 0 && trace.Error != "" {
			exitcode = 1
		}

		// single trace - no end time
		if trace.Start != 0 && trace.Stop == 0 {
			msg := fmt.Sprintf("= %s", trace.Operation)

			if trace.Error != "" {
				msg = fmt.Sprintf("%s (err `%s`)", msg, trace.Error)
			}

			pkgtraces = append(pkgtraces,
				&Trace{
					Operation: msg,
					Exitcode:  exitcode,
					Timing:    trace.Start,
				},
			)
		}

		// trace - start and end time - start block
		if trace.Start != 0 && trace.Stop != 0 {
			msg := fmt.Sprintf("> %s", trace.Operation)
			pkgtraces = append(pkgtraces,
				&Trace{
					Operation: msg,
					Exitcode:  exitcode,
					Timing:    trace.Start,
				},
			)
		}

		// trace - start and end time - end block
		if trace.Start != 0 && trace.Stop != 0 {
			msg := fmt.Sprintf("< %s", trace.Operation)

			if trace.Error != "" {
				msg = fmt.Sprintf("%s (err `%s`)", msg, trace.Error)
			}

			pkgtraces = append(pkgtraces,
				&Trace{
					Operation: msg,
					Exitcode:  exitcode,
					Timing:    trace.Stop,
				},
			)
		}
	}

	sort.Stable(ByTiming(pkgtraces))
	return pkgtraces
}
