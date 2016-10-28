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

// +build windows

// Package updateec2config implements the UpdateEC2Config plugin.
package updateec2config

import (
	"encoding/json"
	"io/ioutil"

	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
)

// UpdateContextFile represents the json structure of the UpdateContext.json file
type UpdateContextFile struct {
	UpdateState  string `json:"updateState"`
	UpdateResult string `json:"updateResult"`
	UpdateStdOut string `json:"UpdateStandardOut"`
}

//loadUpdateContextFile initializes and creates the context file
func (m *updateManager) loadUpdateContext(log log.T,
	path string) (updateContext *UpdateContextFile, err error) {

	ifFileExists := fileutil.Exists(path)
	if ifFileExists == false {
		updateContext = &UpdateContextFile{}
		updateContext.UpdateState = notStarted
		updateContext.UpdateResult = inProgress
		updateContext.UpdateStdOut = ""
	} else {
		if updateContext, err = parseContext(log, path); err != nil {
			return updateContext, err
		}
	}
	return updateContext, nil
}

// parseContext loads and parses update context from local storage
func parseContext(log log.T, fileName string) (context *UpdateContextFile, err error) {
	// Load specified file from file system
	result, err := ioutil.ReadFile(fileName)
	if err != nil {
		return
	}
	// parse context file
	if err = json.Unmarshal([]byte(result), &context); err != nil {
		return
	}

	return context, err
}
