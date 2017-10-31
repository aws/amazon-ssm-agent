// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package proxy config to handle set/get functions of the Windows proxy settings
package proxyconfig

import (
	"errors"
	"net/url"
	"os"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"github.com/aws/amazon-ssm-agent/agent/log"
)

// WinHttpIEProxyConfig represents the Internet Explorer proxy configuration information
// 	fAutoDetect: If TRUE, indicates that the Internet Explorer proxy configuration for the current user specifies "automatically detect settings".
// 	lpszAutoConfigUrl: Pointer to a null-terminated Unicode string that contains the auto-configuration URL if the Internet Explorer proxy configuration for the current user specifies "Use automatic proxy configuration".
// 	lpszProxy: Pointer to a null-terminated Unicode string that contains the proxy URL if the Internet Explorer proxy configuration for the current user specifies "use a proxy server".
// 	lpszProxyBypass: Pointer to a null-terminated Unicode string that contains the optional proxy by-pass server list.
type WinHttpIEProxyConfig struct {
	fAutoDetect       bool
	lpszAutoConfigUrl *uint16
	lpszProxy         *uint16
	lpszProxyBypass   *uint16
}

// WinHttpProxyInfo represents the WinHTTP machine proxy configuration.
// 	lpszProxy: Pointer to a string value that contains the proxy server list.
// 	lpszProxyBypass: Pointer to a string value that contains the proxy bypass list.
type WinHttpProxyInfo struct {
	dwAccessType    uint32
	lpszProxy       *uint16
	lpszProxyBypass *uint16
}

// HttpIEProxyConfig represents the Internet Explorer proxy configuration.
// 	auto: indicates if the 'Automatically detect settings' option in IE is enabled
// 	enabled: indicates if the 'Use proxy settings for your LAN' option in IE is enabled
// 	proxy: specifies the proxy addresses to use.
//	bypass: specifies addresses that should be excluded from proxy
type HttpIEProxyConfig struct {
	proxy   string
	bypass  string
	config  string
	auto    bool
	enabled bool
}

// HttpDefaultProxyConfig represents the WinHTTP machine proxy configuration.
// 	proxy: specifies the proxy addresses to use.
//	bypass: specifies addresses that should be excluded from proxy
type HttpDefaultProxyConfig struct {
	proxy  string
	bypass string
}

// HTTP Proxy environment variables possible values
var ProxyEnvVariables = [3]string{
	"https_proxy",
	"http_proxy",
	"no_proxy",
}

// ProxySettings represents the proxy settings for https_proxy and http_proxy
type ProxySettings struct {
	https_proxy *url.URL
	http_proxy  *url.URL
}

// StringFromUTF16Ptr converts a *uint16 C string to a Go String
// https://github.com/mattn/go-ieproxy/blob/master/utils.go
func StringFromUTF16Ptr(s *uint16) string {
	if s == nil {
		return ""
	}

	p := (*[1<<30 - 1]uint16)(unsafe.Pointer(s))

	// find the string length
	sz := 0
	for p[sz] != 0 {
		sz++
	}

	return string(utf16.Decode(p[:sz:sz]))
}

// For HTTP requests the agent gets the proxy address from the
// environment variables http_proxy, https_proxy and no_proxy
// https_proxy takes precedence over http_proxy for https requests.
// SetProxySettings() overwrites the environment variables using the
// Windows proxy configuration if no settings are provided in the
// registry HKLM:\SYSTEM\CurrentControlSet\Services\AmazonSSMAgent\Environment

func SetProxySettings(log log.T) {
	var err error
	var ie HttpIEProxyConfig
	var df HttpDefaultProxyConfig
	var proxy string
	var bypass string
	var v = []string{}

	for _, value := range ProxyEnvVariables {
		v = append(v, value+":"+os.Getenv(value))
	}
	log.Debugf("Current proxy environment variables: %v", strings.Join(v, ";"))
	v = nil

	// IE current user proxy settings have precedence over WinHTTP machine proxy settings
	if ie, err = GetIEProxySettings(log); ie.enabled && err == nil {
		proxy = ie.proxy
		bypass = ie.bypass

		if ie.auto {
			log.Warnf("IE option 'Automatically  detect settings' is not supported")
		}

		if len(ie.config) > 0 {
			log.Warnf("IE option 'Use automatic configuration script' is not supported")
		}
	} else {
		if df, err = GetDefaultProxySettings(log); len(df.proxy) > 0 && err == nil {
			proxy = df.proxy
			bypass = df.bypass
		}
	}

	// Current registry environment variables http_proxy, https_proxy and no_proxy
	// have precedence over IE and WinHTTP machine proxy settings
	for _, value := range ProxyEnvVariables {
		if v := os.Getenv(value); len(v) > 0 {
			switch value {
			case "https_proxy", "http_proxy":
				proxy = ""
			case "no_proxy":
				bypass = ""
			}
		}
	}

	settings := ParseProxySettings(log, proxy)

	if settings.https_proxy != nil {
		os.Setenv("https_proxy", settings.https_proxy.String())
	}
	if settings.http_proxy != nil {
		os.Setenv("http_proxy", settings.http_proxy.String())
	}

	// Parse no_proxy settings allowing only valid URL or host[:port] values
	// The proxy bypass string contains multiple addresses and host names, separate
	// with blank spaces or semicolons
	var bypassList = []string{}
	for _, f := range strings.Fields(bypass) {
		for _, s := range strings.Split(f, ";") {
			if len(s) == 0 {
				continue
			}
			url, err := ValidateHost(s)
			if err == nil {
				bypassList = append(bypassList, url.Host)
			} else {
				log.Warnf("SetProxySettings invalid URL or host for no_proxy: %v", err.Error())
			}
		}
	}

	if len(bypassList) > 0 {
		os.Setenv("no_proxy", strings.Join(bypassList, ","))
	}

	for _, value := range ProxyEnvVariables {
		v = append(v, value+":"+os.Getenv(value))
	}
	log.Debugf("New proxy environment variables: %v", strings.Join(v, ";"))

	return
}

// GetDefaultProxySettings returns the machine WinHTTP proxy configuration
func GetDefaultProxySettings(log log.T) (p HttpDefaultProxyConfig, err error) {
	winhttp, err := syscall.LoadLibrary("Winhttp.dll")
	if err != nil {
		log.Error("Failed to load Winhttp.dll library: %v", err.Error())
		return p, err
	}

	defer syscall.FreeLibrary(winhttp)

	getDefaultProxy, err := syscall.GetProcAddress(winhttp, "WinHttpGetDefaultProxyConfiguration")
	if err != nil {
		log.Error("Failed to get default machine WinHTTP proxy configuration: %v", err.Error())
		return p, err
	}

	settings := new(WinHttpProxyInfo)

	ret, _, err := syscall.Syscall(uintptr(getDefaultProxy), 1, uintptr(unsafe.Pointer(settings)), 0, 0)
	if ret != 1 {
		log.Error("Failed to get default machine WinHTTP proxy configuration: %v", err.Error())
		return p, err
	} else {
		log.Infof("Getting WinHTTP proxy default configuration: %v", err.Error())
	}
	err = nil

	result := HttpDefaultProxyConfig{
		proxy:  StringFromUTF16Ptr(settings.lpszProxy),
		bypass: StringFromUTF16Ptr(settings.lpszProxyBypass),
	}

	log.Debugf("WinHTTP proxy default configuration: proxy:%v,bypass:%v",
		result.proxy,
		result.bypass,
	)

	return result, nil
}

// GetIEProxySettings returns the Internet Explorer proxy configuration for the current user
func GetIEProxySettings(log log.T) (p HttpIEProxyConfig, err error) {
	p.auto = false
	p.enabled = false
	winhttp, err := syscall.LoadLibrary("Winhttp.dll")
	if err != nil {
		log.Error("Failed to load Winhttp.dll library: ", err.Error())
		return p, err
	}

	defer syscall.FreeLibrary(winhttp)

	getIEProxy, err := syscall.GetProcAddress(winhttp, "WinHttpGetIEProxyConfigForCurrentUser")
	if err != nil {
		log.Error("Failed to get IE proxy configuration for current user: ", err.Error())
		return p, err
	}

	settings := new(WinHttpIEProxyConfig)
	ret, _, err := syscall.Syscall(uintptr(getIEProxy), 1, uintptr(unsafe.Pointer(settings)), 0, 0)
	if ret != 1 {
		log.Error("Failed to get IE proxy configuration for current user: ", err.Error())
		return p, err
	} else {
		log.Info("Getting IE proxy configuration for current user: ", err.Error())
	}
	err = nil

	result := HttpIEProxyConfig{
		proxy:   StringFromUTF16Ptr(settings.lpszProxy),
		bypass:  StringFromUTF16Ptr(settings.lpszProxyBypass),
		auto:    settings.fAutoDetect,
		enabled: settings.lpszProxy != nil,
		config:  StringFromUTF16Ptr(settings.lpszAutoConfigUrl),
	}

	log.Debugf("IE proxy configuration for current user: proxy:%v,bypass:%v,enabled:%v,automatically detect proxy settings:%v,automatic configuration script:%v",
		result.proxy,
		result.bypass,
		result.enabled,
		result.auto,
		result.config,
	)

	return result, nil
}

// ParseProxySettings parses the proxy-list string
// The Windows proxy server list contains one or more of the following strings
// ([<scheme>=][<scheme>"://"]<server>[":"<port>])
// Internet Explorer and WinHTTP support 4 proxy types for [<scheme>=]:
// http=, https=, ftp=, or socks=
func ParseProxySettings(log log.T, proxy string) ProxySettings {
	// Parse http and https proxy settings allowing only valid URL or host[:port] values
	var http, https, other *url.URL
	var err error = nil

	for _, f := range strings.Fields(proxy) {
		for _, s := range strings.Split(f, ";") {
			if len(s) == 0 {
				continue
			}

			split := strings.SplitN(s, "=", 2)
			if len(split) > 1 {
				switch split[0] {
				case "https":
					https, err = ValidateHost(split[1])
				case "http":
					http, err = ValidateHost(split[1])
				default:
					continue
				}
			} else {
				other, err = ValidateHost(split[0])
			}

			if err != nil {
				log.Warnf("ParseProxySettings, invalid URL or host for proxy: %v", err.Error())
			}
		}
	}

	result := ProxySettings{
		http_proxy:  http,
		https_proxy: https,
	}

	// If no [<scheme>=] is provided http is the default option
	if https == nil && http == nil {
		result.http_proxy = other
	} else if https != nil && http == nil {
		result.http_proxy = other
	} else if https == nil && http != nil {
		result.https_proxy = other
	}

	log.Debugf("ParseProxySettings result: http_proxy:%v,https_proxy:%v",
		result.http_proxy,
		result.https_proxy,
	)

	return result
}

// ValidateHost tries to parse the http_proxy and https_proxy addresses
func ValidateHost(s string) (*url.URL, error) {

	if s == "<-loopback>" || s == "<local>" {
		return nil, errors.New(s + " host not supported, skipped")
	}

	// Helps url.Parse to validate an IP for example 127.0.0.1
	if strings.Index(s, "//") == 0 {
		s = "http:" + s
	}

	// Forces http when the schema is missing
	if strings.Index(s, "://") == -1 {
		s = "http://" + s
	}

	u, err := url.Parse(s)
	if err != nil {
		return nil, errors.New(err.Error() + s + ", skipped")
	}

	return u, nil
}
