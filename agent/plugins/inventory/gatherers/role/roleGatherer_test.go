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

var testRole = []model.RoleData{
	{
		Name:                      "AD-Certificate",
		DisplayName:               "Active Directory Certificate Services",
		Description:               "Active Directory Certificate Services (AD CS) is used",
		Installed:                 "False",
		InstalledState:            "",
		FeatureType:               "Role",
		Path:                      "Active Directory Certificate Services",
		SubFeatures:               "ADCS-Cert-Authority ADCS-Enroll-Web-Pol ADCS-Enroll-Web-Svc",
		ServerComponentDescriptor: "ServerComponent_AD_Certificate",
		DependsOn:                 "",
		Parent:                    "",
	},
	{
		Name:                      "ADCS-Cert-Authority",
		DisplayName:               "Certification Authority",
		Description:               "Certification Authority (CA) is used to issue and manage certificates.",
		Installed:                 "False",
		InstalledState:            "",
		FeatureType:               "Role Service",
		Path:                      "Active Directory Certificate Services\\Certification Authority",
		SubFeatures:               "",
		ServerComponentDescriptor: "ServerComponent_ADCS_Cert_Authority",
		DependsOn:                 "",
		Parent:                    "AD-Certificate",
	},
}

func testCollectRoleData(context context.T, config model.Config) (data []model.RoleData, err error) {
	return testRole, nil
}

func TestGatherer(t *testing.T) {
	contextMock := context.NewMockDefault()
	gatherer := Gatherer(contextMock)
	collectData = testCollectRoleData
	item, err := gatherer.Run(contextMock, model.Config{})
	assert.Nil(t, err)
	assert.Equal(t, 1, len(item))
	assert.Equal(t, GathererName, item[0].Name)
	assert.Equal(t, SchemaVersionOfRoleGatherer, item[0].SchemaVersion)
	assert.Equal(t, testRole, item[0].Content)
}
