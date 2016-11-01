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
package windowsUpdate

import (
	"testing"

	"encoding/json"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/stretchr/testify/assert"
)

var testUpdate = []model.WindowsUpdateData{
	model.WindowsUpdateData{
		"KB000001", "Security Update", "Wednesday, October 15, 2014 12:00:00 AM", "ADMINISTRATOR",
	},
	model.WindowsUpdateData{
		"KB000002", "Update", "Friday, June 20, 2014 12:00:00 AM", "NT AUTHORITY SYSTEM",
	},
}

func testExecuteCommand(command string, args ...string) ([]byte, error) {

	output, _ := json.Marshal(testUpdate)
	return output, nil
}

func TestGatherer(t *testing.T) {
	contextMock := context.NewMockDefault()
	gatherer := Gatherer(contextMock)
	cmdExecutor = testExecuteCommand
	item, err := gatherer.Run(contextMock, model.Config{})
	assert.Nil(t, err)
	assert.Equal(t, 1, len(item))
	assert.Equal(t, GathererName, item[0].Name)
	assert.Equal(t, schemaVersionOfWindowsUpdate, item[0].SchemaVersion)
	assert.Equal(t, testUpdate, item[0].Content)
}
