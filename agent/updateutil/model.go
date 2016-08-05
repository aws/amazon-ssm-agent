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

// Package updateutil contains updater specific utilities.
package updateutil

import (
	"encoding/json"
	"io/ioutil"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
)

//UpdatePluginResult represents Agent update plugin result
type UpdatePluginResult struct {
	StandOut      string    `json:"StandOut"`
	StartDateTime time.Time `json:"StartDateTime"`
}

//LoadUpdatePluginResult loads UpdatePluginResult from local storage
func LoadUpdatePluginResult(
	log log.T, updateRoot string) (updateResult *UpdatePluginResult, err error) {

	//Load specified file from file system
	result, err := ioutil.ReadFile(UpdatePluginResultFilePath(updateRoot))
	if err != nil {
		return
	}
	// parse context file
	err = json.Unmarshal([]byte(result), &updateResult)
	if err != nil {
		return
	}

	return updateResult, nil
}

//SaveUpdatePluginResult saves UpdatePluginResult to the local storage
func (util *Utility) SaveUpdatePluginResult(
	log log.T, updateRoot string, updateResult *UpdatePluginResult) (err error) {
	var jsonData = []byte{}
	jsonData, err = json.Marshal(updateResult)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(UpdatePluginResultFilePath(updateRoot), jsonData, appconfig.ReadWriteAccess)
	if err != nil {
		return err
	}

	return nil
}
