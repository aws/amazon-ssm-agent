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

//TODO: add unit tests.

const (
	Name                         = "AWS:WindowsUpdate"
	SchemaVersionOfWindowsUpdate = "1.0"
	Cmd                          = "powershell"
	WindowsUpdateQueryCmd        = "Get-WmiObject -Class win32_quickfixengineering | Select-Object HotFixID,Description,@{N=\"InstalledTime\";E={$_.InstalledOn.DateTime}},InstalledBy | ConvertTo-Json"
)

type T struct{}

func Gatherer(context context.T) (*T, error) {
	return new(T), nil
}

func (t *T) Name() string {
	return Name
}

func (t *T) Run(context context.T, configuration inventory.Config) (inventory.Item, error) {
	var result inventory.Item
	log := context.Log()
	content, err := getWindowsUpdateData()
	if err == nil {
		//CaptureTime must comply with format: 2016-07-30T18:15:37Z or else it will throw error
		currentTime := time.Now().UTC()
		captureTime := currentTime.Format(time.RFC3339)

		result = inventory.Item{
			Name:          t.Name(),
			SchemaVersion: SchemaVersionOfWindowsUpdate,
			Content:       content,
			CaptureTime:   captureTime,
		}
		log.Infof("%v windows update found", len(content))
		log.Debugf("update info = %+v", result)
	} else {
		log.Errorf("Unable to fetch windows update - %v", err.Error())
	}
	return result, err
}

func (t *T) RequestStop(stopType contracts.StopType) error {
	var err error
	return err
}

func getWindowsUpdateData() ([]inventory.WindowsUpdateData, error) {
	var data []inventory.WindowsUpdateData
	out, err := exec.Command(Cmd, WindowsUpdateQueryCmd).Output()
	if err == nil {
		err = json.Unmarshal(out, &data)
	}
	return data, err
}
