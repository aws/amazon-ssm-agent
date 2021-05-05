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

// Package handler provides methods to access resources over HTTP(s)
package handler

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/fileutil/filemanager"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/httpresource/handler/auth/digest"
	"github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/types"
	"github.com/aws/amazon-ssm-agent/agent/ssm/ssmparameterresolver"
)

var ioCopy = io.Copy

// Allowed auth method types
const (
	NONE   = "None"
	BASIC  = "Basic"
	DIGEST = "Digest"
)

var authMethods = map[types.TrimmedString]bool{
	NONE:   true,
	BASIC:  true,
	DIGEST: true,
}

// HTTPAuthConfig defines the attributes used to perform authentication over HTTP
type HTTPAuthConfig struct {
	AuthMethod types.TrimmedString
	Username   types.TrimmedString
	Password   types.TrimmedString
}

// IHTTPHandler defines methods to interact with HTTP resources
type IHTTPHandler interface {
	Download(log log.T, fileSystem filemanager.FileSystem, downloadPath string) (string, error)
	Validate() (bool, error)
}

// httpHandler is used to interact with specific HTTP resources
type httpHandler struct {
	client                     http.Client
	url                        url.URL
	allowInsecureDownload      bool
	authConfig                 HTTPAuthConfig
	ssmParameterResolverBridge ssmparameterresolver.ISsmParameterResolverBridge
}

// NewHTTPHandler creates a new http handler object
func NewHTTPHandler(
	client http.Client,
	url url.URL,
	allowInsecureDownload bool,
	authConfig HTTPAuthConfig,
	bridge ssmparameterresolver.ISsmParameterResolverBridge,
) IHTTPHandler {
	return &httpHandler{
		client:                     client,
		url:                        url,
		allowInsecureDownload:      allowInsecureDownload,
		authConfig:                 authConfig,
		ssmParameterResolverBridge: bridge,
	}
}

// Download downloads a HTTP resource into a specific download path
func (handler *httpHandler) Download(log log.T, fileSystem filemanager.FileSystem, downloadPath string) (string, error) {
	if !handler.isUsingSecureProtocol() && !handler.allowInsecureDownload {
		log.Info("Non secure URL provided and insecure download is not allowed")
		return "", fmt.Errorf("Non secure URL provided and insecure download is not allowed. " +
			"Provide a secure URL or set 'allowInsecureDownload' to true to perform the download operation")
	}

	request, err := handler.prepareRequest(log)
	if err != nil {
		return "", fmt.Errorf("Failed to prepare the request: %s", err.Error())
	}

	out, err := fileSystem.CreateFile(downloadPath)
	if err != nil {
		return "", fmt.Errorf("Cannot create destinaton file: %s", err.Error())
	}
	defer out.Close()

	contentReader, err := handler.requestContent(request)
	if err != nil {
		return "", fmt.Errorf("Failed to download file: %s", err.Error())
	}
	defer contentReader.Close()

	_, err = ioCopy(out, contentReader)
	if err != nil {
		return "", fmt.Errorf("An error occurred during data transfer: %s", err.Error())
	}

	return downloadPath, nil
}

// Validate validates handler's attributes values
func (handler *httpHandler) Validate() (bool, error) {
	if strings.ToUpper(handler.url.Scheme) != "HTTP" && !handler.isUsingSecureProtocol() {
		return false, errors.New("URL scheme for HTTP resource type is invalid. HTTP or HTTPS is accepted")
	}

	if handler.authConfig.AuthMethod != "" && !authMethods[handler.authConfig.AuthMethod] {
		return false, fmt.Errorf("Invalid authentication method: %s. "+
			"The following methods are accepted: None, Basic, Digest", handler.authConfig.AuthMethod)
	}

	return true, nil
}

// authRequest takes care of adding the authorization header to a given request
func (handler *httpHandler) authRequest(log log.T, req *http.Request) (err error) {
	if handler.authConfig.AuthMethod == NONE || handler.authConfig.AuthMethod == "" {
		return nil
	}

	var username = handler.authConfig.Username.Val()
	if handler.ssmParameterResolverBridge.IsValidParameterStoreReference(username) {
		username, err = handler.ssmParameterResolverBridge.GetParameterFromSsmParameterStore(log, username)
		if err != nil {
			return err
		}
	}

	var password = handler.authConfig.Password.Val()
	if handler.ssmParameterResolverBridge.IsValidParameterStoreReference(password) {
		password, err = handler.ssmParameterResolverBridge.GetParameterFromSsmParameterStore(log, password)
		if err != nil {
			return err
		}
	}

	switch handler.authConfig.AuthMethod {
	case DIGEST:
		authzHeader, err := digest.Authorize(username, password, req, &handler.client)
		if err != nil {
			return err
		}

		if authzHeader != "" {
			if req.Header == nil {
				req.Header = make(map[string][]string)
			}
			req.Header.Set("Authorization", authzHeader)
		}
	case BASIC:
		req.SetBasicAuth(username, password)
	default:
		log.Warn("Auth method not supported: %s", handler.authConfig.AuthMethod)
	}

	return nil
}

// isUsingSecureProtocol determines whether the scheme of the given url is HTTPS
func (handler *httpHandler) isUsingSecureProtocol() bool {
	return strings.ToUpper(handler.url.Scheme) == "HTTPS"
}

// prepareRequest prepares the request and takes care of authentication
func (handler *httpHandler) prepareRequest(log log.T) (request *http.Request, err error) {
	request, err = http.NewRequest(http.MethodGet, handler.url.String(), nil)
	if err != nil {
		return nil, err
	}

	err = handler.authRequest(log, request)
	if err != nil {
		return nil, err
	}

	return request, nil
}

// requestContent executes the given request and returns the response body
func (handler *httpHandler) requestContent(request *http.Request) (io.ReadCloser, error) {
	response, err := handler.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("Cannot execute request: %s", err.Error())
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Status: %s", response.Status)
	}

	return response.Body, nil
}
