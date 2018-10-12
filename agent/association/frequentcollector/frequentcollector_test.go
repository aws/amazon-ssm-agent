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

// Package frequentcollector_test provides the unit test code for the FrequentCollector
package frequentcollector_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/association/frequentcollector"
	"github.com/aws/amazon-ssm-agent/agent/association/rateexpr"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/stretchr/testify/assert"

	AssociationModel "github.com/aws/amazon-ssm-agent/agent/association/model"
)

func TestIsFrequentCollectorEnabled_True(t *testing.T) {
	properties := make(map[string]interface{})
	properties["applications"] = "Enabled"
	properties["changeDetectionFrequency"] = "2"

	strTypes := []string{"AWS:Application", "AWS:AWSComponent"}
	types := make([]interface{}, len(strTypes))
	for i, s := range strTypes {
		types[i] = s
	}
	properties["changeDetectionTypes"] = types

	docState := buildInventoryDocumentState(properties)
	assocRawData := buildRatedInstanceAssociation(30)

	frequentCollector := frequentcollector.GetFrequentCollector()
	result := frequentCollector.IsFrequentCollectorEnabled(&docState, &assocRawData)

	assert.Equal(t, result, true, "frequent collector enabled")
}

func TestIsFrequentCollectorEnabled_False_AbsenceOfDetectionTypes(t *testing.T) {
	properties := make(map[string]interface{})
	properties["applications"] = "Enabled"
	properties["changeDetectionFrequency"] = "2"

	docState := buildInventoryDocumentState(properties)
	assocRawData := buildRatedInstanceAssociation(30)

	frequentCollector := frequentcollector.GetFrequentCollector()
	result := frequentCollector.IsFrequentCollectorEnabled(&docState, &assocRawData)

	assert.Equal(t, result, false, "frequent collector is not enabled")
}

func TestIsFrequentCollectorEnabled_False_AbsenceOfDetectionFrequency(t *testing.T) {
	properties := make(map[string]interface{})
	properties["applications"] = "Enabled"

	strTypes := []string{"AWS:Application", "AWS:AWSComponent"}
	types := make([]interface{}, len(strTypes))
	for i, s := range strTypes {
		types[i] = s
	}
	properties["changeDetectionTypes"] = types

	docState := buildInventoryDocumentState(properties)
	assocRawData := buildRatedInstanceAssociation(30)

	frequentCollector := frequentcollector.GetFrequentCollector()
	result := frequentCollector.IsFrequentCollectorEnabled(&docState, &assocRawData)

	assert.Equal(t, result, false, "frequent collector is not enabled")
}

func TestIsFrequentCollectorEnabled_False_ZeroDetectionFrequency(t *testing.T) {
	properties := make(map[string]interface{})
	properties["applications"] = "Enabled"
	properties["changeDetectionFrequency"] = "0"

	strTypes := []string{"AWS:Application", "AWS:AWSComponent"}
	types := make([]interface{}, len(strTypes))
	for i, s := range strTypes {
		types[i] = s
	}
	properties["changeDetectionTypes"] = types

	docState := buildInventoryDocumentState(properties)
	assocRawData := buildRatedInstanceAssociation(30)

	frequentCollector := frequentcollector.GetFrequentCollector()
	result := frequentCollector.IsFrequentCollectorEnabled(&docState, &assocRawData)

	assert.Equal(t, result, false, "frequent collector is not enabled")
}

func TestIsFrequentCollectorEnabled_False_NegativeDetectionFrequency(t *testing.T) {
	properties := make(map[string]interface{})
	properties["applications"] = "Enabled"
	properties["changeDetectionFrequency"] = "-3"

	strTypes := []string{"AWS:Application", "AWS:AWSComponent"}
	types := make([]interface{}, len(strTypes))
	for i, s := range strTypes {
		types[i] = s
	}
	properties["changeDetectionTypes"] = types

	docState := buildInventoryDocumentState(properties)
	assocRawData := buildRatedInstanceAssociation(30)

	frequentCollector := frequentcollector.GetFrequentCollector()
	result := frequentCollector.IsFrequentCollectorEnabled(&docState, &assocRawData)

	assert.Equal(t, result, false, "frequent collector is not enabled")
}

func TestIsFrequentCollectorEnabled_False_NonIntegerInputForDetectionFrequency(t *testing.T) {
	properties := make(map[string]interface{})
	properties["applications"] = "Enabled"
	properties["changeDetectionFrequency"] = "3.56"

	strTypes := []string{"AWS:Application", "AWS:AWSComponent"}
	types := make([]interface{}, len(strTypes))
	for i, s := range strTypes {
		types[i] = s
	}
	properties["changeDetectionTypes"] = types

	docState := buildInventoryDocumentState(properties)
	assocRawData := buildRatedInstanceAssociation(30)

	frequentCollector := frequentcollector.GetFrequentCollector()
	result := frequentCollector.IsFrequentCollectorEnabled(&docState, &assocRawData)

	assert.Equal(t, result, false, "frequent collector is not enabled")
}

func TestIsFrequentCollectorEnabled_False_RandomStringInputForDetectionFrequency(t *testing.T) {
	properties := make(map[string]interface{})
	properties["applications"] = "Enabled"
	properties["changeDetectionFrequency"] = "hello,world"

	strTypes := []string{"AWS:Application", "AWS:AWSComponent"}
	types := make([]interface{}, len(strTypes))
	for i, s := range strTypes {
		types[i] = s
	}
	properties["changeDetectionTypes"] = types

	docState := buildInventoryDocumentState(properties)
	assocRawData := buildRatedInstanceAssociation(30)

	frequentCollector := frequentcollector.GetFrequentCollector()
	result := frequentCollector.IsFrequentCollectorEnabled(&docState, &assocRawData)

	assert.Equal(t, result, false, "frequent collector is not enabled")
}

func TestIsFrequentCollectorEnabled_False_ChangeDetectionTypeDisabled(t *testing.T) {
	properties := make(map[string]interface{})
	properties["applications"] = "Disabled"
	properties["changeDetectionFrequency"] = "2"

	strTypes := []string{"AWS:Application"}
	types := make([]interface{}, len(strTypes))
	for i, s := range strTypes {
		types[i] = s
	}
	properties["changeDetectionTypes"] = types

	docState := buildInventoryDocumentState(properties)
	assocRawData := buildRatedInstanceAssociation(30)

	frequentCollector := frequentcollector.GetFrequentCollector()
	result := frequentCollector.IsFrequentCollectorEnabled(&docState, &assocRawData)

	assert.Equal(t, result, false, "frequent collector is not enabled")
}

func TestGetIntervalInSeconds_InvalidFrequency(t *testing.T) {
	assocRawData := buildRatedInstanceAssociation(30)
	frequentCollector := frequentcollector.GetFrequentCollector()
	valid, _ := frequentCollector.GetIntervalInSeconds(0, assocRawData.ParsedExpression)

	assert.Equal(t, valid, false, "the frequency is invalid")
}

func TestGetIntervalInSeconds_NormalFrequency(t *testing.T) {
	assocRawData := buildRatedInstanceAssociation(30)
	frequentCollector := frequentcollector.GetFrequentCollector()
	valid, seconds := frequentCollector.GetIntervalInSeconds(3, assocRawData.ParsedExpression)

	assert.Equal(t, valid, true, "the frequency is valid")
	assert.Equal(t, seconds, 600, "the interval is correct")
}

func TestGetIntervalInSeconds_TooHighFrequency(t *testing.T) {
	assocRawData := buildRatedInstanceAssociation(30)
	frequentCollector := frequentcollector.GetFrequentCollector()
	valid, seconds := frequentCollector.GetIntervalInSeconds(60, assocRawData.ParsedExpression)

	assert.Equal(t, valid, true, "the frequency is valid")
	assert.Equal(t, seconds, 300, "the interval is correct")
}

func TestIsSoftwareInventoryAssociation_True(t *testing.T) {
	pluginState := contracts.PluginState{
		Name: appconfig.PluginNameAwsSoftwareInventory,
	}

	docState := contracts.DocumentState{
		InstancePluginsInformation: []contracts.PluginState{pluginState},
	}

	frequentCollector := frequentcollector.GetFrequentCollector()
	result := frequentCollector.IsSoftwareInventoryAssociation(&docState)
	assert.Equal(t, result, true, "This is a software inventory association")
}

func TestIsSoftwareInventoryAssociation_False(t *testing.T) {
	pluginState := contracts.PluginState{
		Name: appconfig.PluginNameAwsRunShellScript,
	}

	docState := contracts.DocumentState{
		InstancePluginsInformation: []contracts.PluginState{pluginState},
	}

	frequentCollector := frequentcollector.GetFrequentCollector()
	result := frequentCollector.IsSoftwareInventoryAssociation(&docState)
	assert.Equal(t, result, false, "This is NOT a software inventory association")
}

func buildRatedInstanceAssociation(rateInMinutes int) AssociationModel.InstanceAssociation {
	assocRawData := AssociationModel.InstanceAssociation{
		CreateDate: time.Now(),
	}

	assocRawData.Association = &ssm.InstanceAssociationSummary{}
	testRateExpression := fmt.Sprintf("rate(%d minutes)", rateInMinutes)
	assocRawData.Association.ScheduleExpression = &testRateExpression
	assocRawData.ParsedExpression, _ = rateexpr.Parse(testRateExpression)
	return assocRawData
}

func buildInventoryDocumentState(properties map[string]interface{}) contracts.DocumentState {
	var conf contracts.Configuration
	conf.Properties = properties

	pluginState := contracts.PluginState{
		Name: appconfig.PluginNameAwsSoftwareInventory,
	}
	pluginState.Configuration = conf

	docState := contracts.DocumentState{
		InstancePluginsInformation: []contracts.PluginState{pluginState},
	}
	return docState
}
