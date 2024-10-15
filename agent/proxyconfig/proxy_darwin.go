//go:build darwin
// +build darwin

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

// Package proxy config to handle set/get proxy settings
package proxyconfig

import (
	"context"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
)

const (
	proxyCommand            = "scutil"
	proxyArgument           = "--proxy"
	httpProxyEnabledRegex   = "HTTPEnable:1"
	httpsProxyEnabledRegex  = "HTTPSEnable:1"
	bypassProxyEnabledRegex = "ExceptionsList:"
	httpHostNameRegexExp    = `HTTPProxy:(.*)`
	httpPortRegexExp        = `HTTPPort:(.*)`
	httpsHostNameRegexExp   = `HTTPSProxy:(.*)`
	httpsPortRegexExp       = `HTTPSPort:(.*)`
	bypassProxyRegexExp     = `ExceptionsList:.*\n(\d.*\n)*}`
)

// SetProxyConfig queries the proxy configuration from scutil and sets the required environment variables
func SetProxyConfig(log log.T) map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*1) // command cancelled after 1 second
	defer cancel()

	output, err := exec.CommandContext(ctx, proxyCommand, proxyArgument).Output()
	if err != nil {
		log.Info("Could not run scutil to retrieve proxy information.")
		return GetProxyConfig()
	}

	var proxyInfo = strings.ReplaceAll(string(output), " ", "") // remove white spaces

	httpProxyEnabled, _ := regexp.MatchString(httpProxyEnabledRegex, proxyInfo)
	httpsProxyEnabled, _ := regexp.MatchString(httpsProxyEnabledRegex, proxyInfo)
	bypassProxyEnabled, _ := regexp.MatchString(bypassProxyEnabledRegex, proxyInfo)

	httpProxy := ""
	httpsProxy := ""
	noProxy := ""

	// Parse http proxy
	if httpProxyEnabled == true {
		httpHostNameRegex, _ := regexp.Compile(httpHostNameRegexExp)
		httpHostName := httpHostNameRegex.FindStringSubmatch(proxyInfo)[1]

		httpPortRegex, _ := regexp.Compile(httpPortRegexExp)
		httpPort := httpPortRegex.FindStringSubmatch(proxyInfo)[1]

		httpProxy = httpHostName + ":" + httpPort
	}

	// Parse https proxy
	if httpsProxyEnabled == true {
		httpsHostNameRegex, _ := regexp.Compile(httpsHostNameRegexExp)
		httpsHostName := httpsHostNameRegex.FindStringSubmatch(proxyInfo)[1]

		httpsPortRegex, _ := regexp.Compile(httpsPortRegexExp)
		httpsPort := httpsPortRegex.FindStringSubmatch(proxyInfo)[1]

		httpsProxy = httpsHostName + ":" + httpsPort
	}

	// Parse bypass proxy settings
	if bypassProxyEnabled == true {
		bypassProxyRegex, _ := regexp.Compile(bypassProxyRegexExp)
		exceptionsList := bypassProxyRegex.FindStringSubmatch(proxyInfo)

		// Ignore first
		exceptionsList = strings.Split(exceptionsList[0], "\n")
		exceptionsList = exceptionsList[1 : len(exceptionsList)-1]
		for i := 0; i < len(exceptionsList); i++ {
			exceptionsList[i] = exceptionsList[i][2:]
		}

		noProxy = strings.Join(exceptionsList, ",")
	}

	os.Setenv("http_proxy", httpProxy)
	os.Setenv("https_proxy", httpsProxy)
	os.Setenv("no_proxy", noProxy)

	return GetProxyConfig()
}
