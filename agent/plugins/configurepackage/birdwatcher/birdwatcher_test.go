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

package birdwatcher

import (
	"errors"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/packageservice"
	"github.com/aws/aws-sdk-go/service/birdwatcherstationservice"
	"github.com/stretchr/testify/assert"
)

var loggerMock = log.NewMockLog()
var platformName = "testplatform"
var platformVersion = "testversion"
var architecture = "testarch"

type pkgtree map[string]map[string]map[string]*PackageInfo
type pkgselector struct {
	platform     string
	version      string
	architecture string
	pkginfo      *PackageInfo
}

func manifestPackageGen(sel *[]pkgselector) pkgtree {
	result := pkgtree{}
	for _, s := range *sel {
		if _, ok := result[s.platform]; !ok {
			result[s.platform] = map[string]map[string]*PackageInfo{}
		}

		if _, ok := result[s.platform][s.version]; !ok {
			result[s.platform][s.version] = map[string]*PackageInfo{}
		}

		if _, ok := result[s.platform][s.version][s.architecture]; !ok {
			result[s.platform][s.version][s.architecture] = s.pkginfo
		} else {
			panic("invalid test data")
		}
	}
	return result
}

func TestExtracePackageInfo(t *testing.T) {
	platformProviderdep = &platformProviderMock{
		name:         platformName,
		version:      platformVersion,
		architecture: architecture,
	}

	data := []struct {
		name        string
		manifest    *Manifest
		expected    *PackageInfo
		expectedErr bool
	}{
		{
			"single entry, matching manifest",
			&Manifest{
				Packages: manifestPackageGen(&[]pkgselector{
					{platformName, platformVersion, architecture, &PackageInfo{File: "filename"}},
				}),
			},
			&PackageInfo{File: "filename"},
			false,
		},
		{
			"non-matching name in manifest",
			&Manifest{
				Packages: manifestPackageGen(&[]pkgselector{
					{"nonexistname", platformVersion, architecture, &PackageInfo{File: "filename"}},
				}),
			},
			nil,
			true,
		},
		{
			"non-matching version in manifest",
			&Manifest{
				Packages: manifestPackageGen(&[]pkgselector{
					{platformName, "nonexistversion", architecture, &PackageInfo{File: "filename"}},
				}),
			},
			nil,
			true,
		},
		{
			"non-matching arch in manifest",
			&Manifest{
				Packages: manifestPackageGen(&[]pkgselector{
					{platformName, platformVersion, "nonexistarch", &PackageInfo{File: "filename"}},
				}),
			},
			nil,
			true,
		},
		{
			"multiple entry, matching manifest",
			&Manifest{
				Packages: manifestPackageGen(&[]pkgselector{
					{platformName, platformVersion, "nonexistarch", &PackageInfo{File: "wrongfilename"}},
					{platformName, platformVersion, architecture, &PackageInfo{File: "filename"}},
				}),
			},
			&PackageInfo{File: "filename"},
			false,
		},
		{
			"`_any` platform entry, matching manifest",
			&Manifest{
				Packages: manifestPackageGen(&[]pkgselector{
					{"_any", platformVersion, architecture, &PackageInfo{File: "filename"}},
				}),
			},
			&PackageInfo{File: "filename"},
			false,
		},
		{
			"`_any` version entry, matching manifest",
			&Manifest{
				Packages: manifestPackageGen(&[]pkgselector{
					{platformName, "_any", architecture, &PackageInfo{File: "filename"}},
				}),
			},
			&PackageInfo{File: "filename"},
			false,
		},
		{
			"`_any` arch entry, matching manifest",
			&Manifest{
				Packages: manifestPackageGen(&[]pkgselector{
					{platformName, platformVersion, "_any", &PackageInfo{File: "filename"}},
				}),
			},
			&PackageInfo{File: "filename"},
			false,
		},
		{
			"`_any` entry and concrete entry, matching manifest",
			&Manifest{
				Packages: manifestPackageGen(&[]pkgselector{
					{platformName, platformVersion, "_any", &PackageInfo{File: "wrongfilename"}},
					{platformName, platformVersion, architecture, &PackageInfo{File: "filename"}},
				}),
			},
			&PackageInfo{File: "filename"},
			false,
		},
		{
			"multi-level`_any` entry, matching manifest",
			&Manifest{
				Packages: manifestPackageGen(&[]pkgselector{
					{"_any", "_any", "_any", &PackageInfo{File: "filename"}},
				}),
			},
			&PackageInfo{File: "filename"},
			false,
		},
		{
			"`_any` entry and non-matching entry, non-matching manifest",
			&Manifest{
				Packages: manifestPackageGen(&[]pkgselector{
					{"_any", platformVersion, architecture, &PackageInfo{File: "wrongfilename"}},
					{platformName, platformVersion, "nonexistarch", &PackageInfo{File: "alsowrongfilename"}},
				}),
			},
			nil,
			true,
		},
	}

	for _, testdata := range data {
		t.Run(testdata.name, func(t *testing.T) {
			result, err := extractPackageInfo(loggerMock, testdata.manifest)
			if testdata.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testdata.expected, result)
			}
		})
	}
}

func TestExtracePackageInfoError(t *testing.T) {
	var manifest Manifest // not used

	data := []struct {
		name        string
		provider    platformProviderDep
		expectedErr string
	}{
		{
			"error in platform detection",
			&platformProviderMock{
				nameerr: errors.New("testerror"),
			},
			"failed to detect platform: testerror",
		},
		{
			"error in version detection",
			&platformProviderMock{
				versionerr: errors.New("testerror"),
			},
			"failed to detect platform version: testerror",
		},
		{
			"error in arch detection",
			&platformProviderMock{
				architectureerr: errors.New("testerror"),
			},
			"failed to detect architecture: testerror",
		},
	}

	for _, testdata := range data {
		t.Run(testdata.name, func(t *testing.T) {
			platformProviderdep = testdata.provider

			_, err := extractPackageInfo(loggerMock, &manifest)
			assert.EqualError(t, err, testdata.expectedErr)
		})
	}
}

func TestReportResult(t *testing.T) {
	pkgresult := packageservice.PackageResult{
		PackageName: "name",
		Version:     "1234",
		Timing:      29347,
		Exitcode:    815,
	}

	data := []struct {
		name        string
		bwclient    birdwatcherStationServiceMock
		expectedErr bool
	}{
		{
			"successful api call",
			birdwatcherStationServiceMock{
				putConfigurePackageResultOutput: &birdwatcherstationservice.PutConfigurePackageResultOutput{},
			},
			false,
		},
		{
			"successful api call",
			birdwatcherStationServiceMock{
				putConfigurePackageResultError: errors.New("testerror"),
			},
			true,
		},
	}

	for _, testdata := range data {
		t.Run(testdata.name, func(t *testing.T) {
			ds := &PackageService{bwclient: &testdata.bwclient, manifestCache: &ManifestCacheMem{cache: map[string][]byte{}}}

			err := ds.ReportResult(loggerMock, pkgresult)
			if testdata.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, pkgresult.PackageName, *testdata.bwclient.putConfigurePackageResultInput.PackageName)
				assert.Equal(t, pkgresult.Version, *testdata.bwclient.putConfigurePackageResultInput.PackageVersion)
				assert.Equal(t, pkgresult.Timing, *testdata.bwclient.putConfigurePackageResultInput.OverallTiming)
				assert.Equal(t, pkgresult.Exitcode, *testdata.bwclient.putConfigurePackageResultInput.Result)
			}
		})
	}
}

func TestDownloadManifest(t *testing.T) {
	manifestStrErr := "xkj]{}["
	manifestStr := "{\"version\": \"1234\"}"

	data := []struct {
		name           string
		packageName    string
		packageVersion string
		bwclient       birdwatcherStationServiceMock
		expectedErr    bool
	}{
		{
			"successful getManifest with concrete version",
			"packagename",
			"1234",
			birdwatcherStationServiceMock{
				getManifestOutput: &birdwatcherstationservice.GetManifestOutput{
					Manifest: &manifestStr,
				},
			},
			false,
		},
		{
			"successful getManifest with latest",
			"packagename",
			packageservice.Latest,
			birdwatcherStationServiceMock{
				getManifestOutput: &birdwatcherstationservice.GetManifestOutput{
					Manifest: &manifestStr,
				},
			},
			false,
		},
		{
			"error in getManifest",
			"packagename",
			packageservice.Latest,
			birdwatcherStationServiceMock{
				getManifestError: errors.New("testerror"),
			},
			true,
		},
		{
			"error in parsing manifest",
			"packagename",
			packageservice.Latest,
			birdwatcherStationServiceMock{
				getManifestOutput: &birdwatcherstationservice.GetManifestOutput{
					Manifest: &manifestStrErr,
				},
			},
			true,
		},
	}

	for _, testdata := range data {
		t.Run(testdata.name, func(t *testing.T) {
			ds := &PackageService{bwclient: &testdata.bwclient, manifestCache: &ManifestCacheMem{cache: map[string][]byte{}}}

			result, err := ds.DownloadManifest(loggerMock, testdata.packageName, testdata.packageVersion)

			if testdata.expectedErr {
				assert.Error(t, err)
			} else {
				// verify parameter for api call
				assert.Equal(t, testdata.packageName, *testdata.bwclient.getManifestInput.PackageName)
				assert.Equal(t, testdata.packageVersion, *testdata.bwclient.getManifestInput.PackageVersion)
				// verify result
				assert.Equal(t, "1234", result)
				assert.NoError(t, err)
			}
		})
	}
}

func TestFindFileFromManifest(t *testing.T) {
	platformProviderdep = &platformProviderMock{
		name:         "platformName",
		version:      "platformVersion",
		architecture: "architecture",
	}

	data := []struct {
		name        string
		manifest    *Manifest
		file        File
		expectedErr bool
	}{
		{
			"successful file read",
			&Manifest{
				Packages: manifestPackageGen(&[]pkgselector{
					{"platformName", "platformVersion", "architecture", &PackageInfo{File: "test.zip"}},
				}),
				Files: map[string]*File{"test.zip": &File{DownloadLocation: "https://example.com/agent"}},
			},
			File{
				DownloadLocation: "https://example.com/agent",
			},
			false,
		},
		{
			"fail to find match in file",
			&Manifest{
				Packages: manifestPackageGen(&[]pkgselector{}),
				Files:    map[string]*File{},
			},
			File{},
			true,
		},
		{
			"fail to find file name",
			&Manifest{
				Packages: manifestPackageGen(&[]pkgselector{
					{"platformName", "platformVersion", "architecture", &PackageInfo{File: "test.zip"}},
				}),
				Files: map[string]*File{},
			},
			File{},
			true,
		},
		{
			"fail to find matching file name",
			&Manifest{
				Packages: manifestPackageGen(&[]pkgselector{
					{"platformName", "platformVersion", "architecture", &PackageInfo{File: "test.zip"}},
				}),
				Files: map[string]*File{"nomatch": &File{DownloadLocation: "https://example.com/agent"}},
			},
			File{},
			true,
		},
	}

	for _, testdata := range data {
		t.Run(testdata.name, func(t *testing.T) {
			result, err := findFileFromManifest(loggerMock, testdata.manifest)

			if testdata.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testdata.file, *result)
			}
		})
	}
}

func TestDownloadFile(t *testing.T) {
	data := []struct {
		name        string
		network     networkMock
		file        *File
		expectedErr bool
	}{
		{
			"working file download",
			networkMock{
				downloadOutput: artifact.DownloadOutput{
					LocalFilePath: "agent.zip",
				},
			},
			&File{
				DownloadLocation: "https://example.com/agent",
				Checksums: map[string]string{
					"sha256": "asdf",
				},
			},
			false,
		},
		{
			"empty local file location",
			networkMock{
				downloadOutput: artifact.DownloadOutput{
					LocalFilePath: "",
				},
			},
			&File{
				DownloadLocation: "https://example.com/agent",
				Checksums: map[string]string{
					"sha256": "asdf",
				},
			},
			true,
		},
		{
			"error during file download",
			networkMock{
				downloadError: errors.New("testerror"),
			},
			&File{
				DownloadLocation: "https://example.com/agent",
				Checksums: map[string]string{
					"sha256": "asdf",
				},
			},
			true,
		},
	}
	for _, testdata := range data {
		t.Run(testdata.name, func(t *testing.T) {
			networkdep = &testdata.network

			result, err := downloadFile(loggerMock, testdata.file)
			if testdata.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, "agent.zip", result)
				// verify download input
				input := artifact.DownloadInput{
					SourceURL:       testdata.file.DownloadLocation,
					SourceHashType:  "sha256",
					SourceHashValue: "asdf",
				}
				assert.Equal(t, input, testdata.network.downloadInput)
			}
		})
	}
}

func TestDownloadArtifact(t *testing.T) {
	platformProviderdep = &platformProviderMock{
		name:         "platformName",
		version:      "platformVersion",
		architecture: "architecture",
	}
	manifestStr := `
	{
		"packages": {
			"platformName": {
				"platformVersion": {
					"architecture": {
						"file": "test.zip"
					}
				}
			}
		},
		"files": {
			"test.zip": {
				"downloadLocation": "https://example.com/agent"
			}
		}
	}
	`

	data := []struct {
		name           string
		packageName    string
		packageVersion string
		network        networkMock
		expectedErr    bool
	}{
		{
			"successful download",
			"packageName",
			"1234",
			networkMock{
				downloadOutput: artifact.DownloadOutput{
					LocalFilePath: "agent.zip",
				},
			},
			false,
		},
		{
			"failed manifest loading",
			"packageName",
			"1234",
			networkMock{},
			true,
		},
	}

	for _, testdata := range data {
		t.Run(testdata.name, func(t *testing.T) {
			cache := &ManifestCacheMem{cache: map[string][]byte{}}
			cache.WriteManifest(testdata.packageName, testdata.packageVersion, []byte(manifestStr))

			ds := &PackageService{manifestCache: cache}
			networkdep = &testdata.network

			result, err := ds.DownloadArtifact(loggerMock, testdata.packageName, testdata.packageVersion)

			if testdata.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, "agent.zip", result)
			}
		})
	}
}
