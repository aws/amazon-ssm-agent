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
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/inventory/model"
)

const (
	// GathererName represents name of windows update gatherer
	GathererName = "AWS:WindowsUpdate"

	schemaVersionOfWindowsUpdate = "1.0"
	cmd                          = "powershell"
	windowsUpdateQueryCmd        = "Get-WmiObject -Class win32_quickfixengineering | Select-Object HotFixID,Description,@{N=\"InstalledTime\";E={$_.InstalledOn.DateTime}},InstalledBy | ConvertTo-Json"
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

// decouple exec.Command for unit test
var cmdExecutor = executeCommand

// Run executes windows update gatherer and returns list of inventory.Item
func (t *T) Run(context context.T, configuration inventory.Config) (items []inventory.Item, err error) {
	var result inventory.Item
	log := context.Log()
	var data []inventory.WindowsUpdateData
	out, err := cmdExecutor(cmd, windowsUpdateQueryCmd)
	if err == nil {
		err = json.Unmarshal(out, &data)
		//CaptureTime must comply with format: 2016-07-30T18:15:37Z or else it will throw error
		currentTime := time.Now().UTC()
		captureTime := currentTime.Format(time.RFC3339)

		result = inventory.Item{
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
func (t *T) RequestStop(stopType contracts.StopType) error {
	var err error
	return err
}

func executeCommand(command string, args ...string) ([]byte, error) {
	return exec.Command(command, args...).CombinedOutput()
}
