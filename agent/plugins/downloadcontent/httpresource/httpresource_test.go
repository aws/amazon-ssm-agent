/*
 * Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"). You may not
 * use this file except in compliance with the License. A copy of the
 * License is located at
 *
 * http://aws.amazon.com/apache2.0/
 *
 * or in the "license" file accompanying this file. This file is distributed
 * on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing
 * permissions and limitations under the License.
 */

package httpresource

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	filemock "github.com/aws/amazon-ssm-agent/agent/fileutil/filemanager/mock"
	"github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/httpresource/handler"
	httpMock "github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/httpresource/handler/mock"
	"github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/types"
	bridgemock "github.com/aws/amazon-ssm-agent/agent/ssm/ssmparameterresolver/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var contextMock = context.NewMockDefault()
var logMock = contextMock.Log()
var bm = bridgemock.GetSsmParamResolverBridge(map[string]string{})

func getString(obj interface{}) string {
	return fmt.Sprintf("%v", obj)
}

func getHttpInfo(url, authMethod, user, password string, allowInsecureDownload bool) HTTPInfo {
	return HTTPInfo{
		URL:                   types.NewTrimmedString(url),
		AuthMethod:            types.NewTrimmedString(authMethod),
		Username:              types.NewTrimmedString(user),
		Password:              types.NewTrimmedString(password),
		AllowInsecureDownload: allowInsecureDownload,
	}
}

func getTestResource(url url.URL, authMethod, user, password string, allowInsecureDownload bool) HTTPResource {
	httpClient := http.Client{}
	httpClient.CloseIdleConnections()

	return HTTPResource{
		context: contextMock,
		Handler: handler.NewHTTPHandler(httpClient, url, allowInsecureDownload, handler.HTTPAuthConfig{
			AuthMethod: types.NewTrimmedString(authMethod),
			Username:   types.NewTrimmedString(user),
			Password:   types.NewTrimmedString(password),
		}, bm),
	}
}

func TestNewHTTPResource(t *testing.T) {
	testResource := getTestResource(url.URL{
		Scheme: "http",
		Host:   "example.com",
	}, "Basic", "admin", "pwd", true)

	tests := []struct {
		sourceInfo string
		resource   *HTTPResource
		err        error
	}{
		{
			`{
				"url": "http://example.com",
				"authMethod": "Basic",
				"username": "admin",
				"password": "pwd",
				"allowInsecureDownload": true
			}`,
			&testResource,
			nil,
		},
		{
			`{
				"url": "http://
			}`,
			nil,
			errors.New("SourceInfo could not be unmarshalled for source type HTTP: " +
				"invalid character '\\n' in string literal"),
		},
		{
			`{
				"url": "http:// invalid-url"
			}`,
			nil,
			errors.New("Invalid URL format: " +
				"parse \"http:// invalid-url\": invalid character \" \" in host name"),
		},
	}

	for _, test := range tests {
		resource, err := NewHTTPResource(contextMock, test.sourceInfo, bm)

		if test.err != nil {
			assert.Error(t, err, getString(test))
			assert.EqualError(t, err, test.err.Error(), getString(test))
			assert.Nil(t, resource, getString(test))
		} else {
			assert.NoError(t, err, getString(test))
			assert.True(t, reflect.DeepEqual(test.resource, resource), getString(test))
		}
	}
}

func TestHTTPResource_parseSourceInfo(t *testing.T) {
	tests := []struct {
		sourceInfo string
		httpInfo   HTTPInfo
		err        error
	}{
		{
			`{
				"url": "http://",
				"authMethod": " Basic",
				"username": "admin ",
				"password": " pwd ",
				"allowInsecureDownload": false
			}`,
			getHttpInfo("http://", "Basic", "admin", "pwd", false),
			nil,
		},
		{
			`{
				"url": "http://",
				"allowInsecureDownload": true
			}`,
			getHttpInfo("http://", "", "", "", true),
			nil,
		},
		{
			`{
				"url": "http://"
			}`,
			getHttpInfo("http://", "", "", "", false),
			nil,
		},
		{
			`{
				"url": "http://
			}`,
			HTTPInfo{},
			errors.New("SourceInfo could not be unmarshalled for source type HTTP: " +
				"invalid character '\\n' in string literal"),
		},
	}

	for _, test := range tests {
		httpInfo, err := parseSourceInfo(test.sourceInfo)

		if test.err != nil {
			assert.Equal(t, HTTPInfo{}, httpInfo, getString(test))
			assert.Error(t, err, getString(test), getString(test))
			assert.EqualError(t, err, test.err.Error(), getString(test))
		} else {
			assert.NoError(t, err, getString(test), getString(test))
			assert.Equal(t, test.httpInfo, httpInfo, getString(test), getString(test))
		}
	}
}

func TestHTTPResource_adjustDownloadPath(t *testing.T) {
	testResource := getTestResource(url.URL{}, "", "", "", false)

	tests := []struct {
		givenPath    string
		fileSuffix   string
		downloadPath string
		pathExists   bool
		isDirectory  bool
	}{
		{
			"",
			"",
			filepath.Join(appconfig.DownloadRoot, "download"),
			true,
			false,
		},
		{
			"",
			"123",
			filepath.Join(appconfig.DownloadRoot, "download123"),
			false,
			true,
		},
		{
			filepath.Join("/tmp", "download.txt"),
			"",
			filepath.Join("/tmp", "download.txt"),
			false,
			false,
		},
		{
			"/tmp/",
			"123",
			filepath.Join("/tmp", "download123"),
			false,
			false,
		},
		{
			"/tmp/",
			"123",
			filepath.Join("/tmp", "download123"),
			true,
			true,
		},
		{
			"/tmp",
			"123",
			filepath.Join("/tmp", "download123"),
			true,
			true,
		},

		{
			"/tmp",
			"123",
			filepath.Join("/tmp"),
			false,
			false,
		},
	}

	for _, test := range tests {
		fileSystemMock := filemock.FileSystemMock{}
		fileSystemMock.On("Exists", mock.Anything).Return(test.pathExists)
		fileSystemMock.On("IsDirectory", mock.Anything).Return(test.isDirectory)

		downloadPath := testResource.adjustDownloadPath(test.givenPath, test.fileSuffix, fileSystemMock)
		assert.Equal(t, test.downloadPath, filepath.Join(downloadPath), getString(test))
	}
}

func TestHTTPResource_ValidateLocationInfo(t *testing.T) {
	httpHandlerMock := httpMock.HTTPHandlerMock{}
	httpHandlerMock.On("Validate").Return(true, nil).Once()

	resource := HTTPResource{
		context: contextMock,
		Handler: &httpHandlerMock,
	}

	_, _ = resource.ValidateLocationInfo()
	httpHandlerMock.AssertExpectations(t)
}

func TestHTTPResource_DownloadRemoteResource(t *testing.T) {
	destPath := filepath.Join(os.TempDir(), "testFile")

	fileSystemMock := filemock.FileSystemMock{}
	fileSystemMock.On("MakeDirs", filepath.Dir(destPath)).Return(nil)
	fileSystemMock.On("Exists", destPath).Return(true)
	fileSystemMock.On("IsDirectory", destPath).Return(false)

	httpHandlerMock := httpMock.HTTPHandlerMock{}
	httpHandlerMock.On("Download", logMock, fileSystemMock, destPath).Return(destPath, nil).Once()

	resource := HTTPResource{
		context: contextMock,
		Handler: &httpHandlerMock,
	}
	err, result := resource.DownloadRemoteResource(fileSystemMock, destPath)

	assert.NoError(t, err)
	assert.Equal(t, []string{destPath}, result.Files)

	fileSystemMock.AssertExpectations(t)
	httpHandlerMock.AssertExpectations(t)
}
