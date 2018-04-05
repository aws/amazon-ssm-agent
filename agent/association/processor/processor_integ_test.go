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

// Package processor manage polling of associations, dispatching association to processor
package processor

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/association/converter"
	"github.com/aws/amazon-ssm-agent/agent/association/model"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/log"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/runcommand/contracts"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/stretchr/testify/assert"
)

const (
	FILE_VERSION_1_0 = "./testdata/sampleVersion1_0.json"
	FILE_VERSION_1_2 = "./testdata/sampleVersion1_2.json"
	FILE_VERSION_2_0 = "./testdata/sampleVersion2_0.json"
	FILE_PARAM_2_0   = "./testdata/sampleParams2_0StringMap.json"
)

func TestParseAssociationWithAssociationVersion1_2(t *testing.T) {
	log := log.DefaultLogger()
	context := context.Default(log, appconfig.SsmagentConfig{})
	processor := Processor{
		context: context,
	}
	sys = &systemStub{}

	sampleFile := readFile(FILE_VERSION_1_2)

	instanceID := "i-test"
	assocId := "b2f71a28-cbe1-4429-b848-26c7e1f5ad0d"
	associationName := "testV1.2"
	documentVersion := "1"
	assocRawData := model.InstanceAssociation{
		CreateDate: time.Now(),
		Document:   &sampleFile,
	}
	assocRawData.Association = &ssm.InstanceAssociationSummary{}
	assocRawData.Association.Name = &associationName
	assocRawData.Association.DocumentVersion = &documentVersion
	assocRawData.Association.AssociationId = &assocId
	assocRawData.Association.InstanceId = &instanceID

	params := make(map[string][]*string)
	address := "http://7-zip.org/a/7z1602-x64.msi"
	source := []*string{&address}
	params["source"] = source

	assocRawData.Association.Parameters = params

	docState, err := processor.parseAssociation(&assocRawData)

	documentInfo := new(contracts.DocumentInfo)
	documentInfo.AssociationID = assocId
	documentInfo.InstanceID = instanceID
	documentInfo.MessageID = fmt.Sprintf("aws.ssm.%v.%v", assocId, instanceID)
	documentInfo.DocumentName = associationName
	documentInfo.DocumentVersion = documentVersion

	pluginName := "aws:applications"
	pluginsInfo := make(map[string]contracts.PluginState)
	config := contracts.Configuration{}
	var plugin contracts.PluginState
	plugin.Configuration = config
	plugin.Id = pluginName
	pluginsInfo[pluginName] = plugin

	expectedDocState := contracts.DocumentState{
		InstancePluginsInformation: converter.ConvertPluginsInformation(pluginsInfo),
		DocumentType:               contracts.Association,
		SchemaVersion:              "1.2",
	}

	payload := &messageContracts.SendCommandPayload{}

	err2 := json.Unmarshal([]byte(*assocRawData.Document), &payload.DocumentContent)
	pluginConfig := payload.DocumentContent.RuntimeConfig[pluginName]

	assert.Equal(t, nil, err)
	assert.Equal(t, nil, err2)
	assert.Equal(t, expectedDocState.SchemaVersion, docState.SchemaVersion)
	assert.Equal(t, contracts.Association, docState.DocumentType)

	pluginInfo := docState.InstancePluginsInformation[0]
	expectedProp := []interface{}{map[string]interface{}{"source": *source[0], "sourceHash": "", "id": "0.aws:applications", "action": "Install", "parameters": ""}}

	assert.Equal(t, expectedProp, pluginInfo.Configuration.Properties)
	assert.Equal(t, pluginConfig.Settings, pluginInfo.Configuration.Settings)
	assert.Equal(t, documentInfo.MessageID, pluginInfo.Configuration.MessageId)
}

func TestParseAssociationWithAssociationVersion2_0(t *testing.T) {

	log := log.DefaultLogger()
	context := context.Default(log, appconfig.SsmagentConfig{})
	processor := Processor{
		context: context,
	}
	sys = &systemStub{}

	sampleFile := readFile(FILE_VERSION_2_0)

	instanceID := "i-test"
	assocId := "b2f71a28-cbe1-4429-b848-26c7e1f5ad0d"
	associationName := "testV2.0"
	documentVersion := "1"
	assocRawData := model.InstanceAssociation{
		CreateDate: time.Now(),
		Document:   &sampleFile,
	}
	assocRawData.Association = &ssm.InstanceAssociationSummary{}
	assocRawData.Association.Name = &associationName
	assocRawData.Association.DocumentVersion = &documentVersion
	assocRawData.Association.AssociationId = &assocId
	assocRawData.Association.InstanceId = &instanceID

	params := make(map[string][]*string)
	cmd0 := "ls"
	source0 := []*string{&cmd0}
	cmd1 := "pwd"
	source1 := []*string{&cmd1}
	params["runCommand0"] = source0
	params["runCommand1"] = source1

	assocRawData.Association.Parameters = params

	// test the method
	docState, err := processor.parseAssociation(&assocRawData)

	documentInfo := new(contracts.DocumentInfo)
	documentInfo.AssociationID = assocId
	documentInfo.InstanceID = instanceID
	documentInfo.MessageID = fmt.Sprintf("aws.ssm.%v.%v", assocId, instanceID)
	documentInfo.DocumentName = associationName
	documentInfo.DocumentVersion = documentVersion

	instancePluginsInfo := make([]contracts.PluginState, 2)

	action0 := "aws:runPowerShellScript"
	name0 := "runPowerShellScript1"
	var plugin0 contracts.PluginState
	plugin0.Configuration = contracts.Configuration{}
	plugin0.Id = name0
	plugin0.Name = action0
	instancePluginsInfo[0] = plugin0

	action1 := "aws:runPowerShellScript"
	name1 := "runPowerShellScript2"
	var plugin1 contracts.PluginState
	plugin1.Configuration = contracts.Configuration{}
	plugin1.Id = name1
	plugin1.Name = action1
	instancePluginsInfo[1] = plugin1

	expectedDocState := contracts.DocumentState{
		//DocumentInformation: documentInfo,
		InstancePluginsInformation: instancePluginsInfo,
		DocumentType:               contracts.Association,
		SchemaVersion:              "2.0",
	}

	assert.Equal(t, nil, err)
	assert.Equal(t, expectedDocState.SchemaVersion, docState.SchemaVersion)
	assert.Equal(t, contracts.Association, docState.DocumentType)
	assert.Equal(t, documentInfo.MessageID, docState.DocumentInformation.MessageID)

	pluginInfo1 := docState.InstancePluginsInformation[0]
	pluginInfo2 := docState.InstancePluginsInformation[1]

	assert.Equal(t, name0, pluginInfo1.Id)
	assert.Equal(t, name1, pluginInfo2.Id)
	assert.Equal(t, action0, pluginInfo1.Name)
	assert.Equal(t, action1, pluginInfo2.Name)

	expectProp1 := map[string]interface{}{"id": "0.aws:psModule", "runCommand": *source0[0]}
	expectProp2 := map[string]interface{}{"id": "1.aws:psModule", "runCommand": *source1[0]}

	assert.Equal(t, expectProp1, pluginInfo1.Configuration.Properties)
	assert.Equal(t, expectProp2, pluginInfo2.Configuration.Properties)
}

func TestParseAssociationWithAssociationVersion2_0_StringMapParams(t *testing.T) {

	log := log.DefaultLogger()
	context := context.Default(log, appconfig.SsmagentConfig{})
	processor := Processor{
		context: context,
	}
	sys = &systemStub{}

	sampleFile := readFile(FILE_PARAM_2_0)

	instanceID := "i-test"
	assocId := "b2f71a28-cbe1-4429-b848-26c7e1f5ad0d"
	associationName := "testV2.0"
	documentVersion := "1"
	assocRawData := model.InstanceAssociation{
		CreateDate: time.Now(),
		Document:   &sampleFile,
	}
	assocRawData.Association = &ssm.InstanceAssociationSummary{}
	assocRawData.Association.Name = &associationName
	assocRawData.Association.DocumentVersion = &documentVersion
	assocRawData.Association.AssociationId = &assocId
	assocRawData.Association.InstanceId = &instanceID

	params := make(map[string][]*string)
	cmd0 := "{\"name\":\"AWS-RunPowerShellScript\"}"
	source0 := []*string{&cmd0}
	cmd1 := "pwd"
	source1 := []*string{&cmd1}
	params["sourceInfo"] = source0
	params["runCommand1"] = source1

	assocRawData.Association.Parameters = params

	// test the method
	docState, err := processor.parseAssociation(&assocRawData)

	documentInfo := new(contracts.DocumentInfo)
	documentInfo.AssociationID = assocId
	documentInfo.InstanceID = instanceID
	documentInfo.MessageID = fmt.Sprintf("aws.ssm.%v.%v", assocId, instanceID)
	documentInfo.DocumentName = associationName
	documentInfo.DocumentVersion = documentVersion

	instancePluginsInfo := make([]contracts.PluginState, 2)

	action0 := "aws:downloadContent"
	name0 := "downloadContent"
	var plugin0 contracts.PluginState
	plugin0.Configuration = contracts.Configuration{}
	plugin0.Id = name0
	plugin0.Name = action0
	instancePluginsInfo[0] = plugin0

	action1 := "aws:runPowerShellScript"
	name1 := "runPowerShellScript2"
	var plugin1 contracts.PluginState
	plugin1.Configuration = contracts.Configuration{}
	plugin1.Id = name1
	plugin1.Name = action1
	instancePluginsInfo[1] = plugin1

	expectedDocState := contracts.DocumentState{
		//DocumentInformation: documentInfo,
		InstancePluginsInformation: instancePluginsInfo,
		DocumentType:               contracts.Association,
		SchemaVersion:              "2.0",
	}

	assert.Equal(t, nil, err)
	assert.Equal(t, expectedDocState.SchemaVersion, docState.SchemaVersion)
	assert.Equal(t, contracts.Association, docState.DocumentType)
	assert.Equal(t, documentInfo.MessageID, docState.DocumentInformation.MessageID)

	pluginInfo1 := docState.InstancePluginsInformation[0]
	pluginInfo2 := docState.InstancePluginsInformation[1]

	assert.Equal(t, name0, pluginInfo1.Id)
	assert.Equal(t, name1, pluginInfo2.Id)
	assert.Equal(t, action0, pluginInfo1.Name)
	assert.Equal(t, action1, pluginInfo2.Name)

	expectProp1 := map[string]interface{}{"sourceType": "SSMDocument", "sourceInfo": *source0[0], "id": "0.aws.downloadContent"}
	expectProp2 := map[string]interface{}{"id": "1.aws:psModule", "runCommand": *source1[0]}

	assert.Equal(t, expectProp1, pluginInfo1.Configuration.Properties)
	assert.Equal(t, expectProp2, pluginInfo2.Configuration.Properties)
}

func readFile(fileName string) string {
	file, e := ioutil.ReadFile(fileName)
	if e != nil {
		fmt.Printf("File error: %v\n", e)
		os.Exit(1)
	}
	return string(file)
}
