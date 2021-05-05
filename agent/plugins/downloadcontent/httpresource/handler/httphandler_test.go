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

package handler

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	filemock "github.com/aws/amazon-ssm-agent/agent/fileutil/filemanager/mock"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/types"
	bridgemock "github.com/aws/amazon-ssm-agent/agent/ssm/ssmparameterresolver/mock"
	"github.com/stretchr/testify/assert"
)

var logMock = log.NewMockLog()

func getAuthConfig(authMethod, user, password string) HTTPAuthConfig {
	return HTTPAuthConfig{
		AuthMethod: types.NewTrimmedString(authMethod),
		Username:   types.NewTrimmedString(user),
		Password:   types.NewTrimmedString(password),
	}
}

func getExampleURL(scheme, path string) url.URL {
	if scheme == "" {
		scheme = "HTTP"
	}

	return url.URL{
		Scheme: scheme,
		Host:   "example.com",
		Path:   path,
	}
}

func getHttpHandler(httpClient http.Client, url url.URL, allowInsecureDownload bool, authMethod, user, password string) httpHandler {
	return httpHandler{
		client:                     httpClient,
		url:                        url,
		allowInsecureDownload:      allowInsecureDownload,
		authConfig:                 getAuthConfig(authMethod, user, password),
		ssmParameterResolverBridge: bridgemock.GetSsmParamResolverBridge(parameterStoreParameters),
	}
}

func getString(obj interface{}) string {
	return fmt.Sprintf("%v", obj)
}

var parameterStoreParameters = map[string]string{
	"{{ssm-secure:username}}": "admin",
	"{{ssm-secure:password}}": "pwd",
}

func getParameterFromSsmParameterStoreStub(log log.T, reference string) (string, error) {
	if value, exists := parameterStoreParameters[reference]; exists {
		return value, nil
	}

	return "", errors.New("parameter does not exist")
}

func copyStub(dst io.Writer, src io.Reader) (written int64, err error) {
	return 1, nil
}

func TestNewHTTPHandler(t *testing.T) {
	authConfig := HTTPAuthConfig{
		AuthMethod: BASIC,
		Username:   "admin",
		Password:   "pwd",
	}

	bridge := bridgemock.GetSsmParamResolverBridge(parameterStoreParameters)

	assert.Equal(t, &httpHandler{
		url:                        getExampleURL("http", ""),
		authConfig:                 authConfig,
		ssmParameterResolverBridge: bridge,
	}, NewHTTPHandler(
		http.Client{},
		getExampleURL("http", ""),
		false,
		authConfig,
		bridge,
	))
}

func TestHttpHandlerImpl_Validate(t *testing.T) {
	tests := []struct {
		resource httpHandler
		isValid  bool
		err      error
	}{
		{
			getHttpHandler(http.Client{}, getExampleURL("http", ""), true, "", "", ""),
			true,
			nil,
		},
		{
			getHttpHandler(http.Client{}, getExampleURL("http", ""), false, "", "", ""),
			true,
			nil,
		},
		{
			getHttpHandler(http.Client{}, getExampleURL("https", ""), true, " None ", "", ""),
			true,
			nil,
		},
		{
			getHttpHandler(http.Client{}, getExampleURL("http", ""), true, " Basic ", "", ""),
			true,
			nil,
		},
		{
			getHttpHandler(http.Client{}, getExampleURL("http", ""), true, "Digest ", "", ""),
			true,
			nil,
		},
		{
			getHttpHandler(http.Client{}, getExampleURL("file", ""), true, "", "", ""),
			false,
			errors.New("URL scheme for HTTP resource type is invalid. HTTP or HTTPS is accepted"),
		},
		{
			getHttpHandler(http.Client{}, getExampleURL("http", ""), true, "Test ", "", ""),
			false,
			errors.New("Invalid authentication method: Test. The following methods are accepted: None, Basic, Digest"),
		},
	}

	for _, test := range tests {
		isValid, err := test.resource.Validate()

		if test.err != nil {
			assert.Error(t, err, getString(test))
			assert.EqualError(t, err, test.err.Error(), getString(test))
			assert.False(t, isValid, getString(test))
		} else {
			assert.NoError(t, err, getString(test))
			assert.True(t, isValid, getString(test))
		}
	}
}

func TestNewHTTPHandler_isUsingSecureProtocol(t *testing.T) {
	tests := []struct {
		url                   url.URL
		isUsingSecureProtocol bool
	}{
		{
			getExampleURL("http", ""),
			false,
		},
		{
			getExampleURL("definetelynotsecure", ""),
			false,
		},
		{
			getExampleURL("httpsdasda", ""),
			false,
		},
		{
			getExampleURL("https", ""),
			true,
		},
	}

	for _, test := range tests {
		handler := getHttpHandler(http.Client{}, test.url, true, "", "", "")
		assert.Equal(t, test.isUsingSecureProtocol, handler.isUsingSecureProtocol(), getString(test))
	}
}

func TestHttpHandlerImpl_authRequestBasic(t *testing.T) {
	bridgemock.GetSsmParamResolverBridge(parameterStoreParameters)

	dummyUrl := getExampleURL("http", "")

	tests := []struct {
		resource      httpHandler
		authenticated bool
		credentials   *url.Userinfo
		err           error
	}{
		{
			getHttpHandler(http.Client{}, dummyUrl, true, "Basic", "admin", "pwd"),
			true,
			url.UserPassword("admin", "pwd"),
			nil,
		},
		{
			getHttpHandler(http.Client{}, dummyUrl, true, "None", "", ""),
			false,
			nil,
			nil,
		},
		{
			getHttpHandler(http.Client{}, dummyUrl, true, "Basic", "{{ssm-secure:username}}", "{{ssm-secure:password}}"),
			true,
			url.UserPassword("admin", "pwd"),
			nil,
		},
		{
			getHttpHandler(http.Client{}, dummyUrl, true, "Basic", "{{ssm-secure:invalid-param}}", "pwd"),
			false,
			nil,
			errors.New("parameter does not exist"),
		},
		{
			getHttpHandler(http.Client{}, dummyUrl, true, "Basic", "admin", "{{ssm-secure:invalid-param}}"),
			false,
			nil,
			errors.New("parameter does not exist"),
		},
	}

	for _, test := range tests {
		request := httptest.NewRequest(http.MethodGet, dummyUrl.String(), nil)
		err := test.resource.authRequest(logMock, request)

		if test.err != nil {
			assert.Error(t, err, getString(test))
			assert.EqualError(t, err, test.err.Error(), getString(test))
		} else {
			assert.NoError(t, err, getString(test))
			username, password, authHeaderSet := request.BasicAuth()
			if test.authenticated {
				assert.True(t, authHeaderSet, getString(test))
				assert.Equal(t, url.UserPassword(username, password), test.credentials, getString(test))
			} else {
				assert.False(t, authHeaderSet, getString(test))
			}
		}
	}
}

func TestHttpHandlerImpl_prepareRequestBasicAuth(t *testing.T) {
	tests := []struct {
		handler       httpHandler
		authenticated bool
		err           error
	}{
		{
			getHttpHandler(http.Client{}, getExampleURL("http", ""), true, "Basic", "admin", "pwd"),
			true,
			nil,
		},
		{
			getHttpHandler(http.Client{}, getExampleURL("http", ""), true, "Basic", "{{ssm-secure:test}}", "pwd"),
			false,
			errors.New("parameter does not exist"),
		},
	}

	for _, test := range tests {
		request, err := test.handler.prepareRequest(logMock)

		if test.err != nil {
			assert.Error(t, err, getString(test))
			assert.EqualError(t, err, test.err.Error(), getString(test))
			assert.Nil(t, request)
		} else {
			assert.NoError(t, err, getString(test))
			assert.NotNil(t, request)

			_, _, hasBasicAuth := request.BasicAuth()
			assert.Equal(t, test.authenticated, hasBasicAuth)

		}
	}
}

func TestHttpHandlerImpl_prepareRequestDigestAuth(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Www-Authenticate", `Digest realm="realm"`)
		res.WriteHeader(http.StatusUnauthorized)
	}))
	defer testServer.Close()

	testUrl, _ := url.Parse(testServer.URL)
	handler := getHttpHandler(http.Client{}, *testUrl, true, "Digest", "username", "password")

	request, err := handler.prepareRequest(logMock)

	assert.NoError(t, err)
	assert.NotNil(t, request)
	assert.NotEmpty(t, request.Header["Authorization"])
}

func TestHttpHandlerImpl_requestContent(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/unauthorized" {
			res.WriteHeader(http.StatusUnauthorized)
		} else {
			res.WriteHeader(http.StatusOK)
		}

	}))
	defer testServer.Close()

	testURL, err := url.Parse(testServer.URL)
	assert.NoError(t, err)

	tests := []struct {
		urlPath               string
		request               *http.Request
		allowInsecureDownload bool
		err                   error
	}{
		{
			"unauthorized",
			httptest.NewRequest(http.MethodGet, fmt.Sprintf("%s/unauthorized", testServer.URL), nil),
			true,
			errors.New("Status: 401 Unauthorized"),
		},
		{
			"",
			httptest.NewRequest(http.MethodGet, fmt.Sprintf("%s/", testServer.URL), nil),
			true,
			nil,
		},
		{
			"any-weird-url",
			&http.Request{URL: nil},
			true,
			errors.New("Cannot execute request: Get \"\": http: nil Request.URL"),
		},
	}

	for _, test := range tests {
		handler := getHttpHandler(*testServer.Client(), *testURL, test.allowInsecureDownload, "", "", "")
		handler.url.Path = test.urlPath
		test.request.RequestURI = ""

		responseBody, err := handler.requestContent(test.request)

		if test.err != nil {
			assert.Error(t, err, getString(test))
			assert.EqualError(t, err, test.err.Error(), getString(test))
			assert.Nil(t, responseBody)
		} else {
			assert.NoError(t, err, getString(test))
			assert.NotNil(t, responseBody)
		}
	}

}

func TestHttpHandlerImpl_Download(t *testing.T) {
	tests := []struct {
		secureServer          bool
		allowInsecureDownload bool
		err                   error
	}{
		{
			true,
			false,
			nil,
		},
		{
			true,
			true,
			nil,
		},
		{
			false,
			false,
			errors.New("Non secure URL provided and insecure download is not allowed. " +
				"Provide a secure URL or set 'allowInsecureDownload' to true to perform the download operation"),
		},
		{
			false,
			true,
			nil,
		},
	}

	ioCopy = copyStub

	destPath := os.TempDir()
	fileName := "testFile"
	destinationFile := filepath.Join(destPath, fileName)
	fileSystemMock := filemock.FileSystemMock{}
	fileSystemMock.On("CreateFile", destinationFile).Return(&os.File{}, nil)

	for _, test := range tests {
		var testServer *httptest.Server
		if test.secureServer {
			testServer = httptest.NewTLSServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
				res.WriteHeader(http.StatusOK)
			}))
		} else {
			testServer = httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
				res.WriteHeader(http.StatusOK)
			}))
		}

		testURL, err := url.Parse(testServer.URL)
		assert.NoError(t, err)

		testURL.Path = "testFile"

		handler := getHttpHandler(*testServer.Client(), *testURL, test.allowInsecureDownload, "", "", "")

		downloadedFile, err := handler.Download(logMock, fileSystemMock, destinationFile)

		if test.err == nil {
			assert.NoError(t, err, getString(test))
			assert.Equal(t, destinationFile, downloadedFile, getString(test))
		} else {
			assert.Error(t, err, getString(test))
			assert.EqualError(t, err, test.err.Error(), getString(test))
			logMock.AssertCalled(t, "Info", []interface{}{"Non secure URL provided and insecure download is not allowed"})
		}

		testServer.Close()
	}

	ioCopy = io.Copy
	fileSystemMock.AssertExpectations(t)
}
