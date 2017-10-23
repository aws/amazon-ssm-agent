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

// Package contracts provides model definitions for document state
package contracts

import (
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/platform"
)

//TODO deprecate this functionality once we update the windows update document
type managedInstanceDocumentProperties struct {
	RunCommand []string
	ID         string
}

type documentListType map[string]bool

var managedInstanceIncompatibleAWSSSMDocuments documentListType

func init() {
	var documentList = documentListType{}
	documentList["AWS-ConfigureWindowsUpdate"] = true
	documentList["AWS-FindWindowsUpdates"] = true
	documentList["AWS-InstallMissingWindowsUpdates"] = true
	documentList["AWS-InstallSpecificWindowsUpdates"] = true
	documentList["AWS-ListWindowsInventory"] = true

	managedInstanceIncompatibleAWSSSMDocuments = documentList
}

// RemoveDependencyOnInstanceMetadata looks for array of commands which will be executed as a part of this document and replace the incompatible code.
func RemoveDependencyOnInstanceMetadata(context context.T, docState *DocumentState) error {
	log := context.Log()
	var properties []interface{}
	var parsedDocumentProperties managedInstanceDocumentProperties

	for index, pluginState := range docState.InstancePluginsInformation {
		if pluginState.Name == appconfig.PluginNameAwsRunPowerShellScript {
			err := jsonutil.Remarshal(pluginState.Configuration.Properties, &properties)
			if err != nil {
				log.Debugf("properties format unmatch in %v document. error: %v", docState.DocumentInformation.DocumentName, err)
				return nil
			}

			// Since 'Properties' is an array and we use only one property block for the above documents, array location '0' of 'Properties' is used.
			err = jsonutil.Remarshal(properties[0], &parsedDocumentProperties)
			if err != nil {
				log.Debugf("property format unmatch in %v document. error: %v", docState.DocumentInformation.DocumentName, err)
				return nil
			}

			region, err := platform.Region()
			if err != nil {
				log.Errorf("Error retrieving agent region. error: %v", err)
				return err
			}

			// Comment or replace the incompatible code from this document.
			log.Info("Replacing managed instance incompatible code for AWS SSM Document.")
			for i, command := range parsedDocumentProperties.RunCommand {
				// remove the call to metadata service to retrieve the region for onprem instances
				if strings.Contains(command, "$metadataLocation = 'http://169.254.169.254/latest/dynamic/instance-identity/document/region'") {
					parsedDocumentProperties.RunCommand[i] = strings.Replace(command, "$metadataLocation = 'http://169.254.169.254/latest/dynamic/instance-identity/document/region'", "# $metadataLocation = 'http://169.254.169.254/latest/dynamic/instance-identity/document/region' (This is done to make it managed instance compatible)", 1)
				}

				if strings.Contains(command, "$metadata = (New-Object Net.WebClient).DownloadString($metadataLocation)") {
					parsedDocumentProperties.RunCommand[i] = strings.Replace(command, "$metadata = (New-Object Net.WebClient).DownloadString($metadataLocation)", "# $metadata = (New-Object Net.WebClient).DownloadString($metadataLocation) (This is done to make it managed instance compatible)", 1)
				}

				if strings.Contains(command, "$region = (ConvertFrom-JSON $metadata).region") {
					parsedDocumentProperties.RunCommand[i] = strings.Replace(command, "$region = (ConvertFrom-JSON $metadata).region", "$region = '"+region+"'", 1)
				}
			}

			// Plug-in the compatible 'Properties' block back to the document.
			properties[0] = parsedDocumentProperties
			var documentProperties interface{} = properties
			pluginState.Configuration.Properties = documentProperties

			docState.InstancePluginsInformation[index] = pluginState
		}
	}

	return nil
}

// IsManagedInstanceIncompatibleAWSSSMDocument checks if doc could contain incompatible code for managed instance
func IsManagedInstanceIncompatibleAWSSSMDocument(documentName string) bool {
	return managedInstanceIncompatibleAWSSSMDocuments[documentName]
}
