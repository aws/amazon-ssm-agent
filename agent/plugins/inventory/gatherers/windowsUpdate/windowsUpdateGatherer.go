package windowsUpdate

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

import (
	"encoding/json"
	"os/exec"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
)

const (
	// GathererName represents name of windows update gatherer
	GathererName = "AWS:WindowsUpdate"

	schemaVersionOfWindowsUpdate = "1.0"

	// Represents the status of TrustedInstaller when running
	TrustedInstallerRunningStatus = "Running"
	windowsUpdateQueryCmd         = `
  [Console]::OutputEncoding = [System.Text.Encoding]::UTF8
  Get-WmiObject -Class win32_quickfixengineering | Select-Object HotFixId,Description,@{l="InstalledTime";e={[DateTime]::Parse($_.psbase.properties["installedon"].value,$([System.Globalization.CultureInfo]::GetCultureInfo("en-US"))).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")}},InstalledBy | sort InstalledTime -desc | ConvertTo-Json`

	trustedInstallerStatusQueryCmd = `
  $TrustedInstallerStatus = Get-Service -Name TrustedInstaller | Select-Object Status
  $TrustedInstallerStatus.Status`
)

var (
	cmd = appconfig.PowerShellPluginCommandName

	// decouple for unit test
	cmdExecutor = executeCommand
)

// T represents windows update gatherer
type T struct{}

// Gatherer returns new windows update gatherer
func Gatherer(context context.T) *T {
	return new(T)
}

// Name returns name of windows update gatherer
func (t *T) Name() string {
	return GathererName
}

// Run executes windows update gatherer and returns list of inventory.Item
func (t *T) Run(context context.T, configuration model.Config) (items []model.Item, err error) {
	var result model.Item
	log := context.Log()
	var data []model.WindowsUpdateData
	out, err := cmdExecutor(cmd, windowsUpdateQueryCmd)
	if err == nil {
		// It has been observed that the Get-WmiObject -Class win32_quickfixengineering command can return empty
		// or partial results without an error, depending on the state of TrustedInstaller.exe. This method checks
		// the status of TrustedInstaller. If the TrustedInstaller is not running Windows update data will be Skipped.
		if isTrustedInstallerStopped(context) {
			log.Warnf("Skipping windows update - TrustedInstaller is not running")
			return
		}

		//If there is no windows update in instance, will return empty result instead of throwing error
		if len(out) != 0 {
			err = json.Unmarshal(out, &data)
		}
		//CaptureTime must comply with format: 2016-07-30T18:15:37Z or else it will throw error
		currentTime := time.Now().UTC()
		captureTime := currentTime.Format(time.RFC3339)

		result = model.Item{
			Name:          t.Name(),
			SchemaVersion: schemaVersionOfWindowsUpdate,
			Content:       data,
			CaptureTime:   captureTime,
		}
		log.Infof("%v windows update found", len(data))
		log.Debugf("update info = %+v", result)
	} else {
		log.Errorf("Unable to fetch windows update - %v %v", err.Error(), string(out))
	}
	items = append(items, result)
	return
}

// RequestStop stops the execution of windows update gatherer
func (t *T) RequestStop() error {
	return nil
}

func executeCommand(command string, args ...string) ([]byte, error) {
	return exec.Command(command, args...).CombinedOutput()
}

func isTrustedInstallerStopped(context context.T) bool {
	out, err := cmdExecutor(cmd, trustedInstallerStatusQueryCmd)
	if err == nil {
		cmdOutput := string(out)
		cmdOutput = strings.TrimSpace(cmdOutput)
		context.Log().Infof("TrustedInstallerStatus : %v", cmdOutput)
		if cmdOutput != TrustedInstallerRunningStatus {
			return true
		}
	} else {
		context.Log().Errorf("Error checking TrustedInstallerStatus : %v", err)
		return true
	}
	return false
}
