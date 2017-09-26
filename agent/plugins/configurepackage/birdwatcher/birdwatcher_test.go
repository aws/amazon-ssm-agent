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
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/envdetect"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/envdetect/ec2infradetect"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/envdetect/osdetect"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/packageservice"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/trace"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var platformName = "testplatform"
var platformVersion = "testversion"
var architecture = "testarch"

type TimeMock struct {
	mock.Mock
}

func (t *TimeMock) NowUnixNano() int64 {
	args := t.Called()
	return int64(args.Int(0))
}

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

func TestExtractPackageInfo(t *testing.T) {
	tracer := trace.NewTracer(log.NewMockLog())
	tracer.BeginSection("test segment root")

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
			mockedCollector := envdetect.CollectorMock{}

			mockedCollector.On("CollectData", mock.Anything).Return(&envdetect.Environment{
				&osdetect.OperatingSystem{platformName, platformVersion, "", architecture, "", ""},
				nil,
			}, nil).Once()

			facadeClientMock := facadeMock{
				putConfigurePackageResultOutput: &ssm.PutConfigurePackageResultOutput{},
			}

			ds := &PackageService{facadeClient: &facadeClientMock, manifestCache: packageservice.ManifestCacheMemNew(), collector: &mockedCollector}

			result, err := ds.extractPackageInfo(tracer, testdata.manifest)
			if testdata.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testdata.expected, result)
			}
		})
	}
}

func TestReportResult(t *testing.T) {
	now := 420000
	timemock := &TimeMock{}
	timemock.On("NowUnixNano").Return(now)

	tracer := trace.NewTracer(log.NewMockLog())
	tracer.BeginSection("test segment root")

	data := []struct {
		name          string
		facadeClient  facadeMock
		expectedErr   bool
		packageResult packageservice.PackageResult
	}{
		{
			"successful api call",
			facadeMock{
				putConfigurePackageResultOutput: &ssm.PutConfigurePackageResultOutput{},
			},
			false,
			packageservice.PackageResult{
				PackageName:            "name",
				Version:                "1234",
				PreviousPackageVersion: "5678",
				Timing:                 29347,
				Exitcode:               815,
			},
		},
		{
			"successful api call without previous version",
			facadeMock{
				putConfigurePackageResultOutput: &ssm.PutConfigurePackageResultOutput{},
			},
			false,
			packageservice.PackageResult{
				PackageName: "name",
				Version:     "1234",
				Timing:      29347,
				Exitcode:    815,
			},
		},
		{
			"failing api call",
			facadeMock{
				putConfigurePackageResultError: errors.New("testerror"),
			},
			true,
			packageservice.PackageResult{
				PackageName:            "name",
				Version:                "1234",
				PreviousPackageVersion: "5678",
				Timing:                 29347,
				Exitcode:               815,
			},
		},
	}

	for _, testdata := range data {
		t.Run(testdata.name, func(t *testing.T) {
			mockedCollector := envdetect.CollectorMock{}

			mockedCollector.On("CollectData", mock.Anything).Return(&envdetect.Environment{
				&osdetect.OperatingSystem{"abc", "567", "", "xyz", "", ""},
				&ec2infradetect.Ec2Infrastructure{"instanceIDX", "Reg1", "", "AZ1", "instanceTypeZ"},
			}, nil).Once()
			ds := &PackageService{facadeClient: &testdata.facadeClient, manifestCache: packageservice.ManifestCacheMemNew(), collector: &mockedCollector, timeProvider: timemock}

			err := ds.ReportResult(tracer, testdata.packageResult)
			if testdata.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testdata.packageResult.PackageName, *testdata.facadeClient.putConfigurePackageResultInput.PackageName)
				assert.Equal(t, testdata.packageResult.Version, *testdata.facadeClient.putConfigurePackageResultInput.PackageVersion)
				assert.Equal(t, testdata.packageResult.Operation, *testdata.facadeClient.putConfigurePackageResultInput.Operation)
				if testdata.packageResult.PreviousPackageVersion == "" {
					assert.Nil(t, testdata.facadeClient.putConfigurePackageResultInput.PreviousPackageVersion)
				} else {
					assert.EqualValues(t, &testdata.packageResult.PreviousPackageVersion, testdata.facadeClient.putConfigurePackageResultInput.PreviousPackageVersion)
				}
				assert.Equal(t, (int64(now)-testdata.packageResult.Timing)/1000000, *testdata.facadeClient.putConfigurePackageResultInput.OverallTiming)
				assert.Equal(t, testdata.packageResult.Exitcode, *testdata.facadeClient.putConfigurePackageResultInput.Result)
				assert.Equal(t, "abc", *testdata.facadeClient.putConfigurePackageResultInput.Attributes["platformName"])
				assert.Equal(t, "567", *testdata.facadeClient.putConfigurePackageResultInput.Attributes["platformVersion"])
				assert.Equal(t, "xyz", *testdata.facadeClient.putConfigurePackageResultInput.Attributes["architecture"])
				assert.Equal(t, "instanceIDX", *testdata.facadeClient.putConfigurePackageResultInput.Attributes["instanceID"])
				assert.Equal(t, "instanceTypeZ", *testdata.facadeClient.putConfigurePackageResultInput.Attributes["instanceType"])
				assert.Equal(t, "AZ1", *testdata.facadeClient.putConfigurePackageResultInput.Attributes["availabilityZone"])
				assert.Equal(t, "Reg1", *testdata.facadeClient.putConfigurePackageResultInput.Attributes["region"])
			}
		})
	}
}

func TestDownloadManifest(t *testing.T) {
	manifestStrErr := "xkj]{}["
	manifestStr := "{\"version\": \"1234\"}"
	tracer := trace.NewTracer(log.NewMockLog())

	data := []struct {
		name           string
		packageName    string
		packageVersion string
		facadeClient   facadeMock
		expectedErr    bool
	}{
		{
			"successful getManifest with concrete version",
			"packagename",
			"1234",
			facadeMock{
				getManifestOutput: &ssm.GetManifestOutput{
					Manifest: &manifestStr,
				},
			},
			false,
		},
		{
			"successful getManifest with latest",
			"packagename",
			packageservice.Latest,
			facadeMock{
				getManifestOutput: &ssm.GetManifestOutput{
					Manifest: &manifestStr,
				},
			},
			false,
		},
		{
			"error in getManifest",
			"packagename",
			packageservice.Latest,
			facadeMock{
				getManifestError: errors.New("testerror"),
			},
			true,
		},
		{
			"error in parsing manifest",
			"packagename",
			packageservice.Latest,
			facadeMock{
				getManifestOutput: &ssm.GetManifestOutput{
					Manifest: &manifestStrErr,
				},
			},
			true,
		},
	}

	for _, testdata := range data {
		t.Run(testdata.name, func(t *testing.T) {
			mockedCollector := envdetect.CollectorMock{}
			envdata := &envdetect.Environment{
				&osdetect.OperatingSystem{"abc", "567", "", "xyz", "", ""},
				&ec2infradetect.Ec2Infrastructure{"instanceIDX", "Reg1", "", "AZ1", "instanceTypeZ"},
			}

			mockedCollector.On("CollectData", mock.Anything).Return(envdata, nil).Once()
			ds := &PackageService{facadeClient: &testdata.facadeClient, manifestCache: packageservice.ManifestCacheMemNew(), collector: &mockedCollector}

			result, err := ds.DownloadManifest(tracer, testdata.packageName, testdata.packageVersion)

			if testdata.expectedErr {
				assert.Error(t, err)
			} else {
				// verify parameter for api call
				assert.Equal(t, testdata.packageName, *testdata.facadeClient.getManifestInput.PackageName)
				assert.Equal(t, testdata.packageVersion, *testdata.facadeClient.getManifestInput.PackageVersion)
				// verify result
				assert.Equal(t, "1234", result)
				assert.NoError(t, err)
			}
		})
	}
}

func TestFindFileFromManifest(t *testing.T) {
	tracer := trace.NewTracer(log.NewMockLog())
	tracer.BeginSection("test segment root")

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
			mockedCollector := envdetect.CollectorMock{}

			mockedCollector.On("CollectData", mock.Anything).Return(&envdetect.Environment{
				&osdetect.OperatingSystem{"platformName", "platformVersion", "", "architecture", "", ""},
				&ec2infradetect.Ec2Infrastructure{"instanceID", "region", "", "availabilityZone", "instanceType"},
			}, nil).Once()

			facadeClientMock := facadeMock{
				putConfigurePackageResultOutput: &ssm.PutConfigurePackageResultOutput{},
			}
			ds := &PackageService{facadeClient: &facadeClientMock, manifestCache: packageservice.ManifestCacheMemNew(), collector: &mockedCollector}

			result, err := ds.findFileFromManifest(tracer, testdata.manifest)

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
	tracer := trace.NewTracer(log.NewMockLog())
	tracer.BeginSection("test segment root")

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

			result, err := downloadFile(tracer, testdata.file)
			if testdata.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, "agent.zip", result)
				// verify download input
				input := artifact.DownloadInput{
					SourceURL:       testdata.file.DownloadLocation,
					SourceChecksums: map[string]string{"sha256": "asdf"},
				}
				assert.Equal(t, input, testdata.network.downloadInput)
			}
		})
	}
}

func TestDownloadArtifact(t *testing.T) {
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
	tracer := trace.NewTracer(log.NewMockLog())
	tracer.BeginSection("test segment root")

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
			cache := packageservice.ManifestCacheMemNew()
			cache.WriteManifest(testdata.packageName, testdata.packageVersion, []byte(manifestStr))

			mockedCollector := envdetect.CollectorMock{}

			mockedCollector.On("CollectData", mock.Anything).Return(&envdetect.Environment{
				&osdetect.OperatingSystem{"platformName", "platformVersion", "", "architecture", "", ""},
				&ec2infradetect.Ec2Infrastructure{"instanceID", "region", "", "availabilityZone", "instanceType"},
			}, nil).Once()

			ds := &PackageService{manifestCache: cache, collector: &mockedCollector}
			networkdep = &testdata.network

			result, err := ds.DownloadArtifact(tracer, testdata.packageName, testdata.packageVersion)

			if testdata.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, "agent.zip", result)
			}
		})
	}
}
