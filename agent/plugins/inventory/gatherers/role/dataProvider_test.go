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
	"errors"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
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

var testServerManagerOutput = `
  		<ServerManagerConfigurationQuery>
			<Role DisplayName="Active Directory Certificate Services" Installed="false" Id="AD-Certificate">
  				<RoleService DisplayName="Certification Authority" Installed="false" Id="ADCS-Cert-Authority" Default="true" >
					<RoleService DisplayName="Certification Authority" Installed="false" Id="ADCS-Cert-Authority" Default="true" />
				</RoleService>
  				<RoleService DisplayName="Certification Authority Web Enrollment" Installed="false" Id="ADCS-Web-Enrollment" />
  				<RoleService DisplayName="Online Responder" Installed="false" Id="ADCS-Online-Cert" />
  				<RoleService DisplayName="Network Device Enrollment Service" Installed="true" Id="ADCS-Device-Enrollment" />
			</Role>

			<Feature DisplayName="BitLocker Drive Encryption" Installed="false" Id="BitLocker" />
			<Role DisplayName="NotActive Directory Certificate Services" Installed="false" Id="AD-Certificate">
  				<RoleService DisplayName="Certification Authority" Installed="false" Id="ADCS-Cert-Authority" Default="true" />
  				<RoleService DisplayName="Certification Authority Web Enrollment" Installed="false" Id="ADCS-Web-Enrollment" />
  				<RoleService DisplayName="Online Responder" Installed="false" Id="ADCS-Online-Cert" />
  				<RoleService DisplayName="Network Device Enrollment Service" Installed="false" Id="ADCS-Device-Enrollment" />
			</Role>
			<Feature DisplayName="Remote Server Administration Tools" Installed="false" Id="RSAT">
      			<Feature DisplayName="Role Administration Tools" Installed="false" Id="RSAT-Role-Tools">
         			<Feature DisplayName="Active Directory Certificate Services Tools" Installed="false" Id="RSAT-ADCS"/>
            		<Feature DisplayName="Certification Authority Tools" Installed="false" Id="RSAT-ADCS-Mgmt" />
            		<Feature DisplayName="Online Responder Tools" Installed="false" Id="RSAT-Online-Responder" />
         		</Feature>
			</Feature>
  		</ServerManagerConfigurationQuery>
  	`

func createMockTestExecuteCommand(output string, err error) func(string, ...string) ([]byte, error) {

	return func(string, ...string) ([]byte, error) {
		return []byte(output), err
	}
}

func createMockReadAllText(output string, err error) func(string) (string, error) {
	return func(string) (string, error) {
		return output, err
	}
}

func testResultPath(log log.T) (path string, err error) {
	return "path", nil
}

func testResultPathErr(log log.T) (path string, err error) {
	return "", errors.New("error")
}

func testReadAllText(path string) (xmlData string, err error) {
	return testServerManagerOutput, nil
}

func TestGetRoleData(t *testing.T) {

	contextMock := context.NewMockDefault()
	cmdExecutor = createMockTestExecuteCommand(testRoleOutput, nil)
	startMarker = "<starte0ca162a>"
	endMarker = "<endf0d99a4f>"

	data, err := collectRoleData(contextMock, model.Config{})

	assert.Nil(t, err)
	assert.Equal(t, testRoleOutputData, data)
}

func TestGetRoleDataUsingServerManager(t *testing.T) {
	cmdExecutor = createMockTestExecuteCommand("testOutput", nil)
	resultPath = testResultPath
	readFile = createMockReadAllText(testServerManagerOutput, nil)

	contextMock := context.NewMockDefault()
	mockLog := contextMock.Log()

	var roleInfo []model.RoleData

	err := collectDataUsingServerManager(mockLog, &roleInfo)

	assert.Nil(t, err)
	assert.Equal(t, 17, len(roleInfo))
	assert.Equal(t, "AD-Certificate", roleInfo[0].Name)
	assert.Equal(t, "False", roleInfo[0].Installed)
	assert.Equal(t, "Active Directory Certificate Services", roleInfo[0].DisplayName)
}

func TestGetRoleDataUsingServerManagerCmdError(t *testing.T) {
	cmdExecutor = createMockTestExecuteCommand("testOutput", errors.New("Error"))
	resultPath = testResultPath
	readFile = createMockReadAllText(testServerManagerOutput, nil)

	contextMock := context.NewMockDefault()
	mockLog := contextMock.Log()

	var roleInfo []model.RoleData

	err := collectDataUsingServerManager(mockLog, &roleInfo)

	assert.NotNil(t, err)
}

func TestGetRoleDataUsingServerManagerXmlErr(t *testing.T) {
	cmdExecutor = createMockTestExecuteCommand("testOutput", nil)
	resultPath = testResultPath
	readFile = createMockReadAllText("unexpected output", nil)

	contextMock := context.NewMockDefault()
	mockLog := contextMock.Log()

	var roleInfo []model.RoleData

	err := collectDataUsingServerManager(mockLog, &roleInfo)

	assert.NotNil(t, err)
}

func TestGetRoleDataUsingServerManagerReadError(t *testing.T) {
	cmdExecutor = createMockTestExecuteCommand("testOutput", nil)
	resultPath = testResultPath
	readFile = createMockReadAllText("", errors.New("error"))

	contextMock := context.NewMockDefault()
	mockLog := contextMock.Log()

	var roleInfo []model.RoleData

	err := collectDataUsingServerManager(mockLog, &roleInfo)

	assert.NotNil(t, err)
}

func TestGetRoleDataUsingServerManagerFilePathError(t *testing.T) {
	cmdExecutor = createMockTestExecuteCommand("testOutput", nil)
	resultPath = testResultPathErr
	readFile = createMockReadAllText("", errors.New("error"))

	contextMock := context.NewMockDefault()
	mockLog := contextMock.Log()

	var roleInfo []model.RoleData

	err := collectDataUsingServerManager(mockLog, &roleInfo)

	assert.NotNil(t, err)
}

func TestGetRoleDataCmdExeError(t *testing.T) {
	contextMock := context.NewMockDefault()
	cmdExecutor = createMockTestExecuteCommand("", errors.New("error"))
	startMarker = "<starte0ca162a>"
	endMarker = "<endf0d99a4f>"

	data, err := collectRoleData(contextMock, model.Config{})

	assert.NotNil(t, err)
	assert.Nil(t, data)
}

func TestGetRoleDataCmdUnexpectedOutput(t *testing.T) {
	contextMock := context.NewMockDefault()
	cmdExecutor = createMockTestExecuteCommand("invalid output", nil)
	startMarker = "<starte0ca162a>"
	endMarker = "<endf0d99a4f>"

	data, err := collectRoleData(contextMock, model.Config{})

	assert.NotNil(t, err)
	assert.Nil(t, data)
}

func TestGetRoleDataInvalidMarkedFields(t *testing.T) {
	contextMock := context.NewMockDefault()
	cmdExecutor = createMockTestExecuteCommand(testRoleOutput, nil)
	startMarker = "<starte0ca162a>"
	endMarker = "<test>"

	data, err := collectRoleData(contextMock, model.Config{})

	assert.NotNil(t, err)
	assert.Nil(t, data)
}
