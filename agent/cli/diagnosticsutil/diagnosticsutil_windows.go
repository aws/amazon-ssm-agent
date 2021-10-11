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

//go:build windows
// +build windows

package diagnosticsutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/proxyconfig"
	"github.com/aws/amazon-ssm-agent/core/workerprovider/longrunningprovider/model"
	"golang.org/x/sys/windows/registry"
)

const (
	// ExpectedServiceRunningUser is the user we expect the agent to be running as
	ExpectedServiceRunningUser = "NT AUTHORITY\\SYSTEM"

	// newlineCharacter is the system specific newline character
	newlineCharacter = "\r\n"
)

var powershellArgs = []string{"-InputFormat", "None", "-Noninteractive", "-NoProfile", "-ExecutionPolicy", "unrestricted"}

func executePowershellCommandWithTimeout(timeout time.Duration, command string) (string, error) {
	args := append(powershellArgs, "-Command", command)
	return ExecuteCommandWithTimeout(timeout, appconfig.PowerShellPluginCommandName, args...)
}

func executePowershellScriptWithTimeout(timeout time.Duration, scriptPath string, scriptArgs ...string) (string, error) {
	args := append(powershellArgs, "-File", scriptPath)
	args = append(args, scriptArgs...)
	return ExecuteCommandWithTimeout(timeout, appconfig.PowerShellPluginCommandName, args...)
}

// IsRunningElevatedPermissions checks if the ssm-cli is being executed as administrator
func IsRunningElevatedPermissions() error {
	checkAdminCmd := `([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole] 'Administrator')`
	output, err := executePowershellCommandWithTimeout(2*time.Second, checkAdminCmd)

	if err != nil {
		return fmt.Errorf("failed to check permissions: %s", err)
	}

	if output == "True" {
		return nil
	} else if output == "False" {
		return fmt.Errorf("get-diagnostics needs to be executed by administrator")
	} else {
		return fmt.Errorf("unexpected permission check output: %s", output)
	}
}

func isHttpOrHttpsProxyConfigured(proxyEnv map[string]string) bool {
	_, ishttpSet := proxyEnv[proxyconfig.PROXY_VAR_HTTP]
	_, ishttpsSet := proxyEnv[proxyconfig.PROXY_VAR_HTTPS]

	return ishttpSet || ishttpsSet
}

func isBypassProxyConfigured(proxyEnv map[string]string) bool {
	_, isBypassSet := proxyEnv[proxyconfig.PROXY_VAR_BYPASS]

	return isBypassSet
}

func setWindowsBypass(log logger.T, strToParse string, proxyEnv map[string]string) {
	if len(strToParse) == 0 {
		return
	}

	proxyBypassList := proxyconfig.ParseProxyBypass(log, strToParse)
	if len(proxyBypassList) > 0 {
		proxyEnv[proxyconfig.PROXY_VAR_BYPASS] = strings.Join(proxyBypassList, ",")
	}
}

func setWindowsProxy(log logger.T, strToParse string, proxyEnv map[string]string) {
	if len(strToParse) == 0 {
		return
	}

	proxySettings := proxyconfig.ParseProxySettings(log, strToParse)

	// http proxy
	if proxySettings.HttpProxy != nil {
		proxyEnv["http_proxy"] = proxySettings.HttpProxy.String()
	}

	// https proxy
	if proxySettings.HttpsProxy != nil {
		proxyEnv["https_proxy"] = proxySettings.HttpsProxy.String()
	}
}

func assumeSystemUserEnvironmentProxy(proxyEnv map[string]string) {
	SysRegKey, err := registry.OpenKey(registry.LOCAL_MACHINE, "SYSTEM\\CurrentControlSet\\Control\\Session Manager\\Environment", registry.QUERY_VALUE)
	if err != nil {
		return
	}

	defer SysRegKey.Close()
	for _, key := range proxyconfig.ProxyEnvVariables {
		// Service registry has priority over system env variables, don't overwrite if value already set
		if _, ok := proxyEnv[key]; ok {
			continue
		}

		systemVal, _, err := SysRegKey.GetStringValue(key)
		if err != nil {
			continue
		}

		proxyEnv[key] = systemVal
	}
}

func assumeLocalSystemUserEnvironmentProxy(proxyEnv map[string]string) {
	localSysRegKey, err := registry.OpenKey(registry.USERS, ".DEFAULT\\Environment", registry.QUERY_VALUE)
	if err != nil {
		return
	}

	defer localSysRegKey.Close()
	for _, key := range proxyconfig.ProxyEnvVariables {
		// System user env variables have priority over local user env variables, don't overwrite if value already set
		if _, ok := proxyEnv[key]; ok {
			continue
		}

		localSystemVal, _, err := localSysRegKey.GetStringValue(key)
		if err != nil {
			continue
		}

		proxyEnv[key] = localSystemVal
	}
}

func assumeAgentServiceEnvironmentProxy(proxyEnv map[string]string) {
	// golang windows registry module does not support REG_MULTI_SZ registry value types and produces an error 'unexpected key value type'
	command := `(Get-Item -Path 'HKLM:\System\CurrentControlSet\Services\AmazonSSMAgent').GetValue('Environment')`

	output, err := executePowershellCommandWithTimeout(2*time.Second, command)
	if err != nil {
		return
	}

	if len(output) == 0 {
		return
	}

	// Create proxy key map
	supportedProxyVars := map[string]bool{}
	for _, proxyKey := range proxyconfig.ProxyEnvVariables {
		supportedProxyVars[proxyKey] = true
	}

	for _, envKeyVal := range strings.Split(output, newlineCharacter) {
		envSplit := strings.Split(strings.TrimSpace(envKeyVal), "=")
		if len(envSplit) < 2 {
			// if split at = does not create two indices, skip
			continue
		}

		if supportedProxyVars[envSplit[0]] {
			// Override environment variables because amazonssmagent registry has highest precedent
			proxyEnv[envSplit[0]] = envSplit[1]
		}
	}
}

func assumeInternetExplorerProxySettings(proxyEnv map[string]string) {
	silentLog := logger.NewSilentMockLog()

	commandFormatStr := `(Get-Item -Path 'Registry::HKEY_USERS\.DEFAULT\SOFTWARE\Microsoft\Windows\CurrentVersion\Internet Settings').GetValue('%s')`

	// Check if IE proxy is enabled
	isEnabled, err := executePowershellCommandWithTimeout(2*time.Second, fmt.Sprintf(commandFormatStr, "ProxyEnable"))
	if err != nil || isEnabled != "1" {
		return
	}

	if !isHttpOrHttpsProxyConfigured(proxyEnv) {
		// Get proxy server settings
		proxyServerStr, err := executePowershellCommandWithTimeout(2*time.Second, fmt.Sprintf(commandFormatStr, "ProxyServer"))
		if err == nil {
			setWindowsProxy(silentLog, proxyServerStr, proxyEnv)
		}
	}

	// Check if bypass is already set, if it is return
	if isBypassProxyConfigured(proxyEnv) {
		return
	}

	// Get proxy server settings
	proxyBypassStr, err := executePowershellCommandWithTimeout(2*time.Second, fmt.Sprintf(commandFormatStr, "ProxyOverride"))
	if err == nil {
		setWindowsBypass(silentLog, proxyBypassStr, proxyEnv)
	}
}

func assumeWinHTTPProxySettings(proxyEnv map[string]string) {
	silentLog := logger.NewSilentMockLog()
	if isHttpOrHttpsProxyConfigured(proxyEnv) && isBypassProxyConfigured(proxyEnv) {
		// both proxy and bypass are set, ignore winhttp
		return
	}

	// script to query winhttp proxy settings using powershell
	scriptStr := `$QueryString=$args[0]
$MethodDefinition = @'
using System.Runtime.InteropServices;
public enum AccessType
{
    DefaultProxy = 0,
    NamedProxy = 3,
    NoProxy = 1
}
[StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
public struct WINHTTP_PROXY_INFO
{
    public AccessType AccessType;
    public string Proxy;
    public string Bypass;
}
public class WinHttp
{
    [DllImport("winhttp.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    public static extern bool WinHttpGetDefaultProxyConfiguration(ref WINHTTP_PROXY_INFO config);
}
'@
    $Assembly = Add-Type -TypeDefinition $MethodDefinition -PassThru
    $object = New-Object WINHTTP_PROXY_INFO
    $object.AccessType = [AccessType]::DefaultProxy
    $ret = [WinHttp]::WinHttpGetDefaultProxyConfiguration([ref]$object)

	If ($QueryString -eq 'Proxy') {
		Write-Host $object.Proxy
	} Else {
		Write-Host $object.Bypass
	}`
	scriptPath := filepath.Join(appconfig.SSMDataPath, "ssm_cli_query_winhttp_proxy.ps1")
	f, err := os.Create(scriptPath)
	if err != nil {
		return
	}
	defer os.Remove(scriptPath)

	_, err = fmt.Fprintln(f, scriptStr)
	f.Close()
	if err != nil {
		return
	}

	if !isHttpOrHttpsProxyConfigured(proxyEnv) {
		proxyServerStr, err := executePowershellScriptWithTimeout(10*time.Second, scriptPath, "Proxy")
		if err == nil {
			setWindowsProxy(silentLog, proxyServerStr, proxyEnv)
		}
	}

	// Check if bypass is already set, if it is return
	if isBypassProxyConfigured(proxyEnv) {
		return
	}

	proxyBypassStr, err := executePowershellScriptWithTimeout(10*time.Second, scriptPath, "Bypass")
	if err == nil {
		setWindowsBypass(silentLog, proxyBypassStr, proxyEnv)
	}
}

// AssumeAgentEnvironmentProxy reads system and localsystem environment, and amazonssmagent registry to assume agent proxy
func AssumeAgentEnvironmentProxy() {
	proxyEnv := map[string]string{}

	// AmazonSSMAgent service registry
	assumeAgentServiceEnvironmentProxy(proxyEnv)

	// System environment vars
	assumeSystemUserEnvironmentProxy(proxyEnv)

	// Local system user vars
	assumeLocalSystemUserEnvironmentProxy(proxyEnv)

	// Internet Explorer proxy settings
	assumeInternetExplorerProxySettings(proxyEnv)

	// WinHTTP proxy settings
	assumeWinHTTPProxySettings(proxyEnv)

	// Set proxy for windows
	for _, proxyVar := range proxyconfig.ProxyEnvVariables {
		if val, ok := proxyEnv[proxyVar]; ok {
			os.Setenv(proxyVar, val)
		} else {
			os.Setenv(proxyVar, "")
		}
	}
}

func GetUserRunningAgentProcess() (string, error) {
	command := `(Get-Process -Name amazon-ssm-agent -IncludeUserName | Select-Object -Property UserName | ConvertTo-Csv -NoTypeInformation | Select-Object -Skip 1) -replace '"'`

	cmdOutput, err := executePowershellCommandWithTimeout(2*time.Second, command)
	if err != nil {
		return "", fmt.Errorf("failed to query for user running amazon-ssm-agent: %s", err)
	}

	return cmdOutput, nil
}

func getAgentFilePath() (string, error) {
	return appconfig.DefaultSSMAgentBinaryPath, nil
}

func getAgentProcessPath() (string, error) {
	return model.SSMAgentBinaryName, nil
}
