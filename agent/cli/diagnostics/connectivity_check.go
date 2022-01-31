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

//go:build freebsd || linux || netbsd || openbsd || darwin || windows
// +build freebsd linux netbsd openbsd darwin windows

package diagnostics

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/cli/cliutil"
	"github.com/aws/amazon-ssm-agent/agent/cli/diagnosticsutil"
	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/network"
	"github.com/aws/amazon-ssm-agent/agent/proxyconfig"
	"golang.org/x/net/proxy"
)

const (
	connectivityCheckTimeoutSeconds = 3

	httpPrefix  = "http://"
	httpsPrefix = "https://"

	connectivityCheckStrName             = "Connectivity to %s endpoint"
	connectivityCheckStrFailInit         = "Failed to initialize proxy client: %v"
	connectivityCheckStrFailNotReachable = "%s is not reachable: %v"
	connectivityCheckStrSuccess          = "%s is reachable"
	connectivityCheckStrFailRegion       = "Unable to fetch AWS region details"
	connectivityCheckStrTimeout          = "connection timed out for %s"
)

type httpDialWrapper struct{}

func (httpDialWrapper) Dial(network, addr string) (net.Conn, error) {
	return net.Dial(network, addr)
}

type httpsDialerWrapper struct {
	tlsConfig *tls.Config
}

func (d *httpsDialerWrapper) Dial(network, addr string) (c net.Conn, err error) {
	c, err = tls.Dial("tcp", addr, d.tlsConfig)
	return
}

// proxyDef is the object that defines how to connect to a proxy
type proxyDef struct {
	host    string
	forward proxy.Dialer
}

func (s *proxyDef) Dial(network, addr string) (net.Conn, error) {
	c, err := s.forward.Dial("tcp", s.host)
	if err != nil {
		return nil, err
	}

	reqURL, err := url.Parse("http://" + addr)
	if err != nil {
		c.Close()
		return nil, err
	}

	reqURL.Scheme = ""

	req, err := http.NewRequest("CONNECT", reqURL.String(), nil)
	if err != nil {
		c.Close()
		return nil, err
	}
	req.Close = false
	req.Header.Set("User-Agent", "ssm-cli get diagnostics")

	err = req.Write(c)
	if err != nil {
		c.Close()
		return nil, err
	}

	resp, err := http.ReadResponse(bufio.NewReader(c), req)
	if err != nil {
		c.Close()
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		c.Close()
		return nil, fmt.Errorf("failed to connect to http proxy with status code %d", resp.StatusCode)
	}

	return c, nil
}

func newProxy(uri *url.URL, forward proxy.Dialer) (proxy.Dialer, error) {
	return &proxyDef{
		host:    uri.Host,
		forward: forward,
	}, nil
}

func init() {
	// Initialize http and https dialer
	proxy.RegisterDialerType("http", newProxy)
	proxy.RegisterDialerType("https", newProxy)
}

func isProxyDefined(proxyMap map[string]string) bool {
	if httpsProxy, ok := proxyMap[proxyconfig.PROXY_VAR_HTTPS]; ok && httpsProxy != "" {
		return true
	}
	if httpProxy, ok := proxyMap[proxyconfig.PROXY_VAR_HTTP]; ok && httpProxy != "" {
		return true
	}
	return false
}

type connectivityCheckQuery struct {
	service string
}

func (q connectivityCheckQuery) GetName() string {
	return fmt.Sprintf(connectivityCheckStrName, q.service)
}

func (connectivityCheckQuery) GetPriority() int {
	return 3
}

func getProxyDialer(proxyMap map[string]string) (proxy.Dialer, error) {
	proxyUrl := ""
	// Assign proxyUrl with either the https or http proxy
	if httpsProxy, ok := proxyMap[proxyconfig.PROXY_VAR_HTTPS]; ok && httpsProxy != "" {
		proxyUrl = httpsProxy
	} else if httpProxy, ok := proxyMap[proxyconfig.PROXY_VAR_HTTP]; ok && httpProxy != "" {
		proxyUrl = httpProxy
	} else {
		return nil, fmt.Errorf("neither http nor https proxy is set")
	}

	// Check if the proxy url has https:// prefix, indicating we should use https dialer
	if strings.HasPrefix(proxyUrl, httpsPrefix) {
		parsedProxyUrl, err := url.Parse(proxyUrl)
		if err != nil {
			return nil, err
		}

		tlsConfig := network.GetDefaultTLSConfig(logger.NewSilentMockLog(), appconfig.DefaultConfig())
		return proxy.FromURL(parsedProxyUrl, &httpsDialerWrapper{tlsConfig})
	}

	// If neither http nor https prefix has been set, prefix the url with http://
	if !strings.HasPrefix(proxyUrl, httpPrefix) {
		proxyUrl = httpPrefix + proxyUrl
	}

	parsedProxyUrl, err := url.Parse(proxyUrl)
	if err != nil {
		return nil, err
	}

	return proxy.FromURL(parsedProxyUrl, &httpDialWrapper{})
}

func (q connectivityCheckQuery) dialWithProxy(address string, proxyMap map[string]string, resChan chan diagnosticsutil.DiagnosticOutput) {
	dialer, err := getProxyDialer(proxyMap)

	if err != nil {
		resChan <- diagnosticsutil.DiagnosticOutput{
			Check:  q.GetName(),
			Status: diagnosticsutil.DiagnosticsStatusFailed,
			Note:   fmt.Sprintf(connectivityCheckStrFailInit, err),
		}
		return
	}

	conn, err := dialer.Dial("tcp", fmt.Sprintf("%v:443", address))
	if err != nil {
		resChan <- diagnosticsutil.DiagnosticOutput{
			Check:  q.GetName(),
			Status: diagnosticsutil.DiagnosticsStatusFailed,
			Note:   fmt.Sprintf(connectivityCheckStrFailNotReachable, address, err),
		}
		return
	}

	defer conn.Close()
	resChan <- diagnosticsutil.DiagnosticOutput{
		Check:  q.GetName(),
		Status: diagnosticsutil.DiagnosticsStatusSuccess,
		Note:   fmt.Sprintf(connectivityCheckStrSuccess, address),
	}
}

func (q connectivityCheckQuery) dialWithoutProxy(address string) diagnosticsutil.DiagnosticOutput {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%v:443", address), time.Second)

	if err != nil {
		return diagnosticsutil.DiagnosticOutput{
			Check:  q.GetName(),
			Status: diagnosticsutil.DiagnosticsStatusFailed,
			Note:   fmt.Sprintf(connectivityCheckStrFailNotReachable, address, err),
		}
	}
	defer conn.Close()

	return diagnosticsutil.DiagnosticOutput{
		Check:  q.GetName(),
		Status: diagnosticsutil.DiagnosticsStatusSuccess,
		Note:   fmt.Sprintf(connectivityCheckStrSuccess, address),
	}
}

func (q connectivityCheckQuery) Execute() diagnosticsutil.DiagnosticOutput {
	agentIdentity, err := cliutil.GetAgentIdentity()

	if err != nil {
		return diagnosticsutil.DiagnosticOutput{
			Check:  q.GetName(),
			Status: diagnosticsutil.DiagnosticsStatusSkipped,
			Note:   connectivityCheckStrFailRegion,
		}
	}

	address := agentIdentity.GetServiceEndpoint(q.service)

	proxyMap := proxyconfig.GetProxyConfig()
	if isProxyDefined(proxyMap) {
		// must introduce custom timeout logic as proxy dial does not support timeout
		resChan := make(chan diagnosticsutil.DiagnosticOutput, 1)
		go q.dialWithProxy(address, proxyMap, resChan)

		select {
		case <-time.After(time.Second * connectivityCheckTimeoutSeconds):
			return diagnosticsutil.DiagnosticOutput{
				Check:  q.GetName(),
				Status: diagnosticsutil.DiagnosticsStatusFailed,
				Note:   fmt.Sprintf(connectivityCheckStrTimeout, address),
			}
		case res := <-resChan:
			return res
		}
	}

	return q.dialWithoutProxy(address)
}

func init() {
	// ssm endpoints
	diagnosticsutil.RegisterDiagnosticQuery(connectivityCheckQuery{"ssm"})
	diagnosticsutil.RegisterDiagnosticQuery(connectivityCheckQuery{"ec2messages"})
	diagnosticsutil.RegisterDiagnosticQuery(connectivityCheckQuery{"ssmmessages"})

	// s3 endpoint
	diagnosticsutil.RegisterDiagnosticQuery(connectivityCheckQuery{"s3"})

	// kms endpoint
	diagnosticsutil.RegisterDiagnosticQuery(connectivityCheckQuery{"kms"})

	// cloudwatch endpoints
	diagnosticsutil.RegisterDiagnosticQuery(connectivityCheckQuery{"logs"})
	diagnosticsutil.RegisterDiagnosticQuery(connectivityCheckQuery{"monitoring"})
}
