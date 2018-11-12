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

package birdwatcherservice

import (
	"errors"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher/archive"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher/birdwatcherarchive"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher/documentarchive"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher/facade"
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

type pkgtree map[string]map[string]map[string]*birdwatcher.PackageInfo
type pkgselector struct {
	platform     string
	version      string
	architecture string
	pkginfo      *birdwatcher.PackageInfo
}

func manifestPackageGen(sel *[]pkgselector) pkgtree {
	result := pkgtree{}
	for _, s := range *sel {
		if _, ok := result[s.platform]; !ok {
			result[s.platform] = map[string]map[string]*birdwatcher.PackageInfo{}
		}

		if _, ok := result[s.platform][s.version]; !ok {
			result[s.platform][s.version] = map[string]*birdwatcher.PackageInfo{}
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
		manifest    *birdwatcher.Manifest
		expected    *birdwatcher.PackageInfo
		expectedErr bool
	}{
		{
			"single entry, matching manifest",
			&birdwatcher.Manifest{
				Packages: manifestPackageGen(&[]pkgselector{
					{platformName, platformVersion, architecture, &birdwatcher.PackageInfo{FileName: "filename"}},
				}),
			},
			&birdwatcher.PackageInfo{FileName: "filename"},
			false,
		},
		{
			"non-matching name in manifest",
			&birdwatcher.Manifest{
				Packages: manifestPackageGen(&[]pkgselector{
					{"nonexistname", platformVersion, architecture, &birdwatcher.PackageInfo{FileName: "filename"}},
				}),
			},
			nil,
			true,
		},
		{
			"non-matching version in manifest",
			&birdwatcher.Manifest{
				Packages: manifestPackageGen(&[]pkgselector{
					{platformName, "nonexistversion", architecture, &birdwatcher.PackageInfo{FileName: "filename"}},
				}),
			},
			nil,
			true,
		},
		{
			"non-matching arch in manifest",
			&birdwatcher.Manifest{
				Packages: manifestPackageGen(&[]pkgselector{
					{platformName, platformVersion, "nonexistarch", &birdwatcher.PackageInfo{FileName: "filename"}},
				}),
			},
			nil,
			true,
		},
		{
			"multiple entry, matching manifest",
			&birdwatcher.Manifest{
				Packages: manifestPackageGen(&[]pkgselector{
					{platformName, platformVersion, "nonexistarch", &birdwatcher.PackageInfo{FileName: "wrongfilename"}},
					{platformName, platformVersion, architecture, &birdwatcher.PackageInfo{FileName: "filename"}},
				}),
			},
			&birdwatcher.PackageInfo{FileName: "filename"},
			false,
		},
		{
			"`_any` platform entry, matching manifest",
			&birdwatcher.Manifest{
				Packages: manifestPackageGen(&[]pkgselector{
					{"_any", platformVersion, architecture, &birdwatcher.PackageInfo{FileName: "filename"}},
				}),
			},
			&birdwatcher.PackageInfo{FileName: "filename"},
			false,
		},
		{
			"`_any` version entry, matching manifest",
			&birdwatcher.Manifest{
				Packages: manifestPackageGen(&[]pkgselector{
					{platformName, "_any", architecture, &birdwatcher.PackageInfo{FileName: "filename"}},
				}),
			},
			&birdwatcher.PackageInfo{FileName: "filename"},
			false,
		},
		{
			"`_any` arch entry, matching manifest",
			&birdwatcher.Manifest{
				Packages: manifestPackageGen(&[]pkgselector{
					{platformName, platformVersion, "_any", &birdwatcher.PackageInfo{FileName: "filename"}},
				}),
			},
			&birdwatcher.PackageInfo{FileName: "filename"},
			false,
		},
		{
			"`_any` entry and concrete entry, matching manifest",
			&birdwatcher.Manifest{
				Packages: manifestPackageGen(&[]pkgselector{
					{platformName, platformVersion, "_any", &birdwatcher.PackageInfo{FileName: "wrongfilename"}},
					{platformName, platformVersion, architecture, &birdwatcher.PackageInfo{FileName: "filename"}},
				}),
			},
			&birdwatcher.PackageInfo{FileName: "filename"},
			false,
		},
		{
			"multi-level`_any` entry, matching manifest",
			&birdwatcher.Manifest{
				Packages: manifestPackageGen(&[]pkgselector{
					{"_any", "_any", "_any", &birdwatcher.PackageInfo{FileName: "filename"}},
				}),
			},
			&birdwatcher.PackageInfo{FileName: "filename"},
			false,
		},
		{
			"`_any` entry and non-matching entry, non-matching manifest",
			&birdwatcher.Manifest{
				Packages: manifestPackageGen(&[]pkgselector{
					{"_any", platformVersion, architecture, &birdwatcher.PackageInfo{FileName: "wrongfilename"}},
					{platformName, platformVersion, "nonexistarch", &birdwatcher.PackageInfo{FileName: "alsowrongfilename"}},
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

			facadeClientMock := facade.FacadeStub{
				PutConfigurePackageResultOutput: &ssm.PutConfigurePackageResultOutput{},
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
		facadeClient  facade.FacadeStub
		expectedErr   bool
		packageResult packageservice.PackageResult
	}{
		{
			"successful api call",
			facade.FacadeStub{
				PutConfigurePackageResultOutput: &ssm.PutConfigurePackageResultOutput{},
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
			facade.FacadeStub{
				PutConfigurePackageResultOutput: &ssm.PutConfigurePackageResultOutput{},
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
			facade.FacadeStub{
				PutConfigurePackageResultError: errors.New("testerror"),
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
				assert.Equal(t, testdata.packageResult.PackageName, *testdata.facadeClient.PutConfigurePackageResultInput.PackageName)
				assert.Equal(t, testdata.packageResult.Version, *testdata.facadeClient.PutConfigurePackageResultInput.PackageVersion)
				assert.Equal(t, testdata.packageResult.Operation, *testdata.facadeClient.PutConfigurePackageResultInput.Operation)
				if testdata.packageResult.PreviousPackageVersion == "" {
					assert.Nil(t, testdata.facadeClient.PutConfigurePackageResultInput.PreviousPackageVersion)
				} else {
					assert.EqualValues(t, &testdata.packageResult.PreviousPackageVersion, testdata.facadeClient.PutConfigurePackageResultInput.PreviousPackageVersion)
				}
				assert.Equal(t, (int64(now)-testdata.packageResult.Timing)/1000000, *testdata.facadeClient.PutConfigurePackageResultInput.OverallTiming)
				assert.Equal(t, testdata.packageResult.Exitcode, *testdata.facadeClient.PutConfigurePackageResultInput.Result)
				assert.Equal(t, "abc", *testdata.facadeClient.PutConfigurePackageResultInput.Attributes["platformName"])
				assert.Equal(t, "567", *testdata.facadeClient.PutConfigurePackageResultInput.Attributes["platformVersion"])
				assert.Equal(t, "xyz", *testdata.facadeClient.PutConfigurePackageResultInput.Attributes["architecture"])
				assert.Equal(t, "instanceIDX", *testdata.facadeClient.PutConfigurePackageResultInput.Attributes["instanceID"])
				assert.Equal(t, "instanceTypeZ", *testdata.facadeClient.PutConfigurePackageResultInput.Attributes["instanceType"])
				assert.Equal(t, "AZ1", *testdata.facadeClient.PutConfigurePackageResultInput.Attributes["availabilityZone"])
				assert.Equal(t, "Reg1", *testdata.facadeClient.PutConfigurePackageResultInput.Attributes["region"])
			}
		})
	}
}

func TestDownloadManifest(t *testing.T) {
	manifestStrErr := "xkj]{}["
	manifestStr := "{\"version\": \"1234\",\"packageArn\":\"packagearn\"}"
	tracer := trace.NewTracer(log.NewMockLog())

	data := []struct {
		name           string
		packageName    string
		packageVersion string
		facadeClient   facade.FacadeStub
		manifest       string
		expectedErr    bool
	}{
		{
			"successful getManifest with concrete version",
			"packagename",
			"1234",
			facade.FacadeStub{
				GetManifestOutput: &ssm.GetManifestOutput{
					Manifest: &manifestStr,
				},
			},
			"",
			false,
		},
		{
			"successful getManifest with latest",
			"packagename",
			packageservice.Latest,
			facade.FacadeStub{
				GetManifestOutput: &ssm.GetManifestOutput{
					Manifest: &manifestStr,
				},
			},
			"",
			false,
		},
		{
			"error in getManifest",
			"packagename",
			packageservice.Latest,
			facade.FacadeStub{
				GetManifestError: errors.New("testerror"),
			},
			"",
			true,
		},
		{
			"error in parsing manifest",
			"packagename",
			packageservice.Latest,
			facade.FacadeStub{
				GetManifestOutput: &ssm.GetManifestOutput{
					Manifest: &manifestStrErr,
				},
			},
			"",
			true,
		},
		{
			"Manifest already stored in package service",
			"packagename",
			packageservice.Latest,
			facade.FacadeStub{},
			manifestStr,
			false,
		},
	}

	for _, testdata := range data {
		t.Run(testdata.name, func(t *testing.T) {
			testArchive := birdwatcherarchive.New(&testdata.facadeClient, testdata.manifest)
			mockedCollector := envdetect.CollectorMock{}
			envdata := &envdetect.Environment{
				&osdetect.OperatingSystem{"abc", "567", "", "xyz", "", ""},
				&ec2infradetect.Ec2Infrastructure{"instanceIDX", "Reg1", "", "AZ1", "instanceTypeZ"},
			}

			mockedCollector.On("CollectData", mock.Anything).Return(envdata, nil).Once()
			cache := packageservice.ManifestCacheMemNew()
			ds := &PackageService{facadeClient: &testdata.facadeClient, manifestCache: cache, collector: &mockedCollector, archive: testArchive}

			_, result, isSameAsCache, err := ds.DownloadManifest(tracer, testdata.packageName, testdata.packageVersion)

			if testdata.expectedErr {
				assert.Error(t, err)
			} else {
				if testdata.manifest == "" {
					// verify parameter for api call
					assert.Equal(t, testdata.packageName, *testdata.facadeClient.GetManifestInput.PackageName)
					assert.Equal(t, testdata.packageVersion, *testdata.facadeClient.GetManifestInput.PackageVersion)
				}
				// verify result
				assert.Equal(t, "1234", result)
				assert.NoError(t, err)
				assert.False(t, isSameAsCache)
				// verify cache
				cachedManifest, cacheErr := cache.ReadManifest("packagearn", "1234")
				assert.Equal(t, []byte(manifestStr), cachedManifest)
				assert.NoError(t, cacheErr)
			}
		})
	}
}

func TestDownloadDocument(t *testing.T) {
	manifestStr := "{\"version\": \"1234\"}"
	documentActive := ssm.DocumentStatusActive
	tracer := trace.NewTracer(log.NewMockLog())
	packageName := "documentarn"
	packageVersion := "1234"

	data := []struct {
		name           string
		packageName    string
		packageVersion string
		facadeClient   facade.FacadeStub
		expectedErr    bool
	}{
		{
			"successful getDocument with concrete version",
			packageName,
			packageVersion,
			facade.FacadeStub{
				GetDocumentOutput: &ssm.GetDocumentOutput{
					Content:     &manifestStr,
					Status:      &documentActive,
					Name:        &packageName,
					VersionName: &packageVersion,
				},
			},
			false,
		},
	}

	for _, testdata := range data {
		t.Run(testdata.name, func(t *testing.T) {
			testArchive := documentarchive.New(&testdata.facadeClient)
			mockedCollector := envdetect.CollectorMock{}
			envdata := &envdetect.Environment{
				&osdetect.OperatingSystem{"abc", "567", "", "xyz", "", ""},
				&ec2infradetect.Ec2Infrastructure{"instanceIDX", "Reg1", "", "AZ1", "instanceTypeZ"},
			}

			mockedCollector.On("CollectData", mock.Anything).Return(envdata, nil).Once()
			cache := packageservice.ManifestCacheMemNew()
			ds := &PackageService{facadeClient: &testdata.facadeClient, manifestCache: cache, collector: &mockedCollector, archive: testArchive}

			_, result, isSameAsCache, err := ds.DownloadManifest(tracer, testdata.packageName, testdata.packageVersion)

			if testdata.expectedErr {
				assert.Error(t, err)
			} else {
				// verify parameter for api call
				assert.Equal(t, testdata.packageName, *testdata.facadeClient.GetDocumentInput.Name)
				assert.Equal(t, testdata.packageVersion, *testdata.facadeClient.GetDocumentInput.VersionName)
				// verify result
				assert.Equal(t, "1234", result)
				assert.NoError(t, err)
				assert.False(t, isSameAsCache)
				// verify cache
				cachedManifest, cacheErr := cache.ReadManifest("documentarn", "1234")
				assert.Equal(t, []byte(manifestStr), cachedManifest)
				assert.NoError(t, cacheErr)
			}
		})
	}
}

func TestDownloadManifestSameAsCacheManifest(t *testing.T) {
	manifestStr := "{\"version\": \"1234\",\"packageArn\":\"packagearn\"}"
	tracer := trace.NewTracer(log.NewMockLog())
	data := []struct {
		name           string
		packageName    string
		packageVersion string
		facadeClient   facade.FacadeStub
		expectedErr    bool
	}{
		{
			"successful getManifest same as cache",
			"packagearn",
			"1234",
			facade.FacadeStub{
				GetManifestOutput: &ssm.GetManifestOutput{
					Manifest: &manifestStr,
				},
			},
			false,
		},
		{
			"successful getManifest same as cache for latest version",
			"packagearn",
			packageservice.Latest,
			facade.FacadeStub{
				GetManifestOutput: &ssm.GetManifestOutput{
					Manifest: &manifestStr,
				},
			},
			false,
		},
		{
			"successful getManifest same as cache if name != returned arn",
			"packagename",
			packageservice.Latest,
			facade.FacadeStub{
				GetManifestOutput: &ssm.GetManifestOutput{
					Manifest: &manifestStr,
				},
			},
			false,
		},
	}

	tracer.BeginSection("test successful getManifest same as cache")

	mockedCollector := envdetect.CollectorMock{}

	for _, testdata := range data {
		testArchive := birdwatcherarchive.New(&testdata.facadeClient, "")
		cache := packageservice.ManifestCacheMemNew()

		ds := &PackageService{facadeClient: &testdata.facadeClient, manifestCache: cache, collector: &mockedCollector, archive: testArchive}

		// first call has empty cache and is expected to come back with isSameAsCache == false
		_, result, isSameAsCache, err := ds.DownloadManifest(tracer, testdata.packageName, testdata.packageVersion)
		assert.NoError(t, err)
		assert.False(t, isSameAsCache)

		// second call has the cache already populated by the first call
		_, result, isSameAsCache, err = ds.DownloadManifest(tracer, testdata.packageName, testdata.packageVersion)

		// verify parameter for api call
		assert.Equal(t, testdata.packageName, *testdata.facadeClient.GetManifestInput.PackageName)
		assert.Equal(t, testdata.packageVersion, *testdata.facadeClient.GetManifestInput.PackageVersion)
		// verify result
		assert.Equal(t, "1234", result)
		assert.NoError(t, err)
		assert.True(t, isSameAsCache)
		// verify cache
		cachedManifest, cacheErr := cache.ReadManifest("packagearn", "1234")
		assert.Equal(t, []byte(manifestStr), cachedManifest)
		assert.NoError(t, cacheErr)
	}
}

func TestDownloadManifestDifferentFromCacheManifest(t *testing.T) {
	cachedManifestStr := "{\"version\": \"123\",\"packageArn\":\"packagearn\"}"
	manifestStr := "{\"version\": \"1234\",\"packageArn\":\"packagearn\"}"
	tracer := trace.NewTracer(log.NewMockLog())

	testdata := struct {
		name           string
		packageName    string
		packageVersion string
		facadeClient   facade.FacadeStub
		expectedErr    bool
	}{
		"successful getManifest different from cache",
		"packagenameorarndoesnotmatter",
		"packageversiondoesnotmatter",
		facade.FacadeStub{
			GetManifestOutput: &ssm.GetManifestOutput{
				Manifest: &manifestStr,
			},
		},
		false,
	}

	tracer.BeginSection("test successful getManifest different from cache")

	testArchive := birdwatcherarchive.New(&testdata.facadeClient, "")
	mockedCollector := envdetect.CollectorMock{}
	envdata := &envdetect.Environment{
		&osdetect.OperatingSystem{"abc", "567", "", "xyz", "", ""},
		&ec2infradetect.Ec2Infrastructure{"instanceIDX", "Reg1", "", "AZ1", "instanceTypeZ"},
	}

	mockedCollector.On("CollectData", mock.Anything).Return(envdata, nil).Once()

	cache := packageservice.ManifestCacheMemNew()
	err := cache.WriteManifest("packagearn", "1234", []byte(cachedManifestStr))
	assert.NoError(t, err)

	ds := &PackageService{facadeClient: &testdata.facadeClient, manifestCache: cache, collector: &mockedCollector, archive: testArchive}

	_, result, isSameAsCache, err := ds.DownloadManifest(tracer, testdata.packageName, testdata.packageVersion)

	// verify parameter for api call
	assert.Equal(t, testdata.packageName, *testdata.facadeClient.GetManifestInput.PackageName)
	assert.Equal(t, testdata.packageVersion, *testdata.facadeClient.GetManifestInput.PackageVersion)
	// verify result
	assert.Equal(t, "1234", result)
	assert.NoError(t, err)
	assert.False(t, isSameAsCache)
	// verify cache
	cachedManifest, cacheErr := cache.ReadManifest("packagearn", "1234")
	assert.Equal(t, []byte(manifestStr), cachedManifest)
	assert.NoError(t, cacheErr)
}

func TestFindFileFromManifest(t *testing.T) {
	tracer := trace.NewTracer(log.NewMockLog())
	tracer.BeginSection("test segment root")
	fileName := "test.zip"

	data := []struct {
		name        string
		manifest    *birdwatcher.Manifest
		file        archive.File
		expectedErr bool
	}{
		{
			"successful file read",
			&birdwatcher.Manifest{
				Packages: manifestPackageGen(&[]pkgselector{
					{"platformName", "platformVersion", "architecture", &birdwatcher.PackageInfo{FileName: fileName}},
				}),
				Files: map[string]*birdwatcher.FileInfo{"test.zip": &birdwatcher.FileInfo{DownloadLocation: "https://example.com/agent"}},
			},
			archive.File{
				fileName,
				birdwatcher.FileInfo{
					DownloadLocation: "https://example.com/agent",
				},
			},
			false,
		},
		{
			"fail to find match in file",
			&birdwatcher.Manifest{
				Packages: manifestPackageGen(&[]pkgselector{}),
				Files:    map[string]*birdwatcher.FileInfo{},
			},
			archive.File{},
			true,
		},
		{
			"fail to find file name",
			&birdwatcher.Manifest{
				Packages: manifestPackageGen(&[]pkgselector{
					{"platformName", "platformVersion", "architecture", &birdwatcher.PackageInfo{FileName: "test.zip"}},
				}),
				Files: map[string]*birdwatcher.FileInfo{},
			},
			archive.File{},
			true,
		},
		{
			"fail to find matching file name",
			&birdwatcher.Manifest{
				Packages: manifestPackageGen(&[]pkgselector{
					{"platformName", "platformVersion", "architecture", &birdwatcher.PackageInfo{FileName: "test.zip"}},
				}),
				Files: map[string]*birdwatcher.FileInfo{"nomatch": &birdwatcher.FileInfo{DownloadLocation: "https://example.com/agent"}},
			},
			archive.File{},
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

			facadeClientMock := facade.FacadeStub{
				PutConfigurePackageResultOutput: &ssm.PutConfigurePackageResultOutput{},
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
	packagename := "packagename"
	version := "version"
	fileName := "fileName.zip"

	data := []struct {
		name        string
		network     networkMock
		file        *archive.File
		expectedErr bool
	}{
		{
			"working file download",
			networkMock{
				downloadOutput: artifact.DownloadOutput{
					LocalFilePath: "agent.zip",
				},
			},
			&archive.File{
				fileName,
				birdwatcher.FileInfo{
					DownloadLocation: "https://example.com/agent",
					Checksums: map[string]string{
						"sha256": "asdf",
					},
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
			&archive.File{
				fileName,
				birdwatcher.FileInfo{
					DownloadLocation: "https://example.com/agent",
					Checksums: map[string]string{
						"sha256": "asdf",
					},
				},
			},
			true,
		},
		{
			"error during file download",
			networkMock{
				downloadError: errors.New("testerror"),
			},
			&archive.File{
				fileName,
				birdwatcher.FileInfo{
					DownloadLocation: "https://example.com/agent",
					Checksums: map[string]string{
						"sha256": "asdf",
					},
				},
			},
			true,
		},
	}
	for _, testdata := range data {
		t.Run(testdata.name, func(t *testing.T) {
			birdwatcher.Networkdep = &testdata.network
			cache := packageservice.ManifestCacheMemNew()
			testArchive := birdwatcherarchive.New(&facade.FacadeStub{}, "manifest")

			mockedCollector := envdetect.CollectorMock{}
			ds := &PackageService{manifestCache: cache, collector: &mockedCollector, archive: testArchive}

			result, err := downloadFile(ds, tracer, testdata.file, packagename, version)
			if testdata.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, "agent.zip", result)
				// verify download input
				input := artifact.DownloadInput{
					SourceURL:       testdata.file.Info.DownloadLocation,
					SourceChecksums: map[string]string{"sha256": "asdf"},
				}
				assert.Equal(t, input, testdata.network.downloadInput)
			}
		})
	}
}

func TestDownloadFileFromDocumentArchive(t *testing.T) {
	tracer := trace.NewTracer(log.NewMockLog())
	tracer.BeginSection("test segment root")
	packagename := "packagename"
	version := "version"
	fileName := "fileName.zip"
	url := "url"
	documentActive := ssm.DocumentStatusActive
	data := []struct {
		name        string
		network     networkMock
		file        *archive.File
		expectedErr bool
	}{
		{
			"working file download",
			networkMock{
				downloadOutput: artifact.DownloadOutput{
					LocalFilePath: "agent.zip",
				},
			},
			&archive.File{
				fileName,
				birdwatcher.FileInfo{},
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
			&archive.File{
				fileName,
				birdwatcher.FileInfo{},
			},
			true,
		},
		{
			"error during file download",
			networkMock{
				downloadError: errors.New("testerror"),
			},
			&archive.File{
				fileName,
				birdwatcher.FileInfo{},
			},
			true,
		},
	}
	for _, testdata := range data {
		t.Run(testdata.name, func(t *testing.T) {
			birdwatcher.Networkdep = &testdata.network
			cache := packageservice.ManifestCacheMemNew()
			facadeClient := facade.FacadeStub{
				GetDocumentOutput: &ssm.GetDocumentOutput{
					Status: &documentActive,
					Name:   &packagename,
					AttachmentsContent: []*ssm.AttachmentContent{
						{
							Name: &fileName,
							Url:  &url,
						},
					},
				},
			}
			testArchive := documentarchive.New(&facadeClient)

			mockedCollector := envdetect.CollectorMock{}
			ds := &PackageService{manifestCache: cache, collector: &mockedCollector, archive: testArchive}

			result, err := downloadFile(ds, tracer, testdata.file, packagename, version)
			if testdata.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, "agent.zip", result)
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
			testArchive := birdwatcherarchive.New(&facade.FacadeStub{}, manifestStr)
			mockedCollector := envdetect.CollectorMock{}

			mockedCollector.On("CollectData", mock.Anything).Return(&envdetect.Environment{
				&osdetect.OperatingSystem{"platformName", "platformVersion", "", "architecture", "", ""},
				&ec2infradetect.Ec2Infrastructure{"instanceID", "region", "", "availabilityZone", "instanceType"},
			}, nil).Once()

			ds := &PackageService{manifestCache: cache, collector: &mockedCollector, archive: testArchive}
			birdwatcher.Networkdep = &testdata.network

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
