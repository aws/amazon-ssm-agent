// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package role

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/stretchr/testify/assert"
)

var testRoleOutput = "[{\"Name\": \"AD-Certificate\", \"DisplayName\": \"Active Directory Certificate Services\", \"Description\": \"<starte0ca162a>Active Directory Certificate Services (AD CS) is used<endf0d99a4f>\", \"Installed\": \"False\", \"InstalledState\": \"\", \"FeatureType\": \"Role\", \"Path\": \"<starte0ca162a>Active Directory Certificate Services<endf0d99a4f>\", \"SubFeatures\": \"ADCS-Cert-Authority ADCS-Enroll-Web-Pol\", \"ServerComponentDescriptor\": \"ServerComponent_AD_Certificate\", \"DependsOn\": \"\", \"Parent\": \"\"}, {\"Name\": \"AD-Certificate2\", \"DisplayName\": \"Active Directory Certificate Services\", \"Description\": \"<starte0ca162a>Active Directory Certificate Services (AD CS) is used<endf0d99a4f>\", \"Installed\": \"True\", \"InstalledState\": \"\", \"FeatureType\": \"Role\", \"Path\": \"<starte0ca162a>Active Directory Certificate Services<endf0d99a4f>\", \"SubFeatures\": \"ADCS-Cert-Authority ADCS-Enroll-Web-Pol\", \"ServerComponentDescriptor\": \"ServerComponent_AD_Certificate\", \"DependsOn\": \"\", \"Parent\": \"\"}]"

var testRoleOutputData = []model.RoleData{
	{
		Name:                      "AD-Certificate",
		DisplayName:               "Active Directory Certificate Services",
		Description:               "Active Directory Certificate Services (AD CS) is used",
		Installed:                 "False",
		InstalledState:            "",
		FeatureType:               "Role",
		Path:                      "Active Directory Certificate Services",
		SubFeatures:               "ADCS-Cert-Authority ADCS-Enroll-Web-Pol",
		ServerComponentDescriptor: "ServerComponent_AD_Certificate",
		DependsOn:                 "",
		Parent:                    "",
	},
	{
		Name:                      "AD-Certificate2",
		DisplayName:               "Active Directory Certificate Services",
		Description:               "Active Directory Certificate Services (AD CS) is used",
		Installed:                 "True",
		InstalledState:            "",
		FeatureType:               "Role",
		Path:                      "Active Directory Certificate Services",
		SubFeatures:               "ADCS-Cert-Authority ADCS-Enroll-Web-Pol",
		ServerComponentDescriptor: "ServerComponent_AD_Certificate",
		DependsOn:                 "",
		Parent:                    "",
	},
}

func testExecuteCommand(command string, args ...string) ([]byte, error) {
	return []byte(testRoleOutput), nil
}

func TestGetRoleData(t *testing.T) {

	contextMock := context.NewMockDefault()
	cmdExecutor = testExecuteCommand
	startMarker = "<starte0ca162a>"
	endMarker = "<endf0d99a4f>"

	data, err := collectRoleData(contextMock, model.Config{})

	assert.Nil(t, err)
	assert.Equal(t, testRoleOutputData, data)
}
