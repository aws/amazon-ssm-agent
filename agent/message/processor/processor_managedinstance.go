// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Amazon Software License (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/asl/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package processor implements MDS plugin processor
// processor_managedinstance removes dependency on instance meta data for managed instances.
package processor

import (
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/managedInstances/registration"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/message/contracts"
)

type ManagedInstanceDocumentProperties struct {
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

// Our intent here is to look for array of commands which will be executed as a part of this document and replace the incompatible code.
func removeDependencyOnInstanceMetadataForManagedInstance(context context.T, parsedMessage messageContracts.SendCommandPayload) (messageContracts.SendCommandPayload, error) {
	log := context.Log()
	var properties []interface{}
	var parsedDocumentProperties ManagedInstanceDocumentProperties

	err := jsonutil.Remarshal(parsedMessage.DocumentContent.RuntimeConfig[appconfig.PluginNameAwsRunScript].Properties, &properties)
	if err != nil {
		log.Errorf("Invalid format of properties in %v document. error: %v", parsedMessage.DocumentName, err)
		return parsedMessage, err
	}

	// Since 'Properties' is an array and we use only one property block for the above documents, array location '0' of 'Properties' is used.
	err = jsonutil.Remarshal(properties[0], &parsedDocumentProperties)
	if err != nil {
		log.Errorf("Invalid format of properties in %v document. error: %v", parsedMessage.DocumentName, err)
		return parsedMessage, err
	}

	// Comment or replace the incompatible code from this document.
	log.Info("Replacing managed instance incompatible code for AWS SSM Document.")
	for i, command := range parsedDocumentProperties.RunCommand {
		if strings.Contains(command, "$metadataLocation = 'http://169.254.169.254/latest/dynamic/instance-identity/document/region'") {
			parsedDocumentProperties.RunCommand[i] = strings.Replace(command, "$metadataLocation = 'http://169.254.169.254/latest/dynamic/instance-identity/document/region'", "# $metadataLocation = 'http://169.254.169.254/latest/dynamic/instance-identity/document/region' (This is done to make it managed instance compatible)", 1)
		}

		if strings.Contains(command, "$metadata = (New-Object Net.WebClient).DownloadString($metadataLocation)") {
			parsedDocumentProperties.RunCommand[i] = strings.Replace(command, "$metadata = (New-Object Net.WebClient).DownloadString($metadataLocation)", "# $metadata = (New-Object Net.WebClient).DownloadString($metadataLocation) (This is done to make it managed instance compatible)", 1)
		}

		if strings.Contains(command, "$region = (ConvertFrom-JSON $metadata).region") {
			parsedDocumentProperties.RunCommand[i] = strings.Replace(command, "$region = (ConvertFrom-JSON $metadata).region", "$region = '"+registration.Region()+"'", 1)
		}
	}

	// Plug-in the compatible 'Properties' block back to the document.
	properties[0] = parsedDocumentProperties
	var documentProperties interface{} = properties
	parsedMessage.DocumentContent.RuntimeConfig[appconfig.PluginNameAwsRunScript].Properties = documentProperties

	// For debug purposes.
	parsedMessageContent, err := jsonutil.Marshal(parsedMessage)
	if err != nil {
		log.Errorf("Error marshalling %v document. error: %v", parsedMessage.DocumentName, err)
		return parsedMessage, err
	}
	log.Debug("ParsedMessage after removing dependency on instance metadata for managed instance is ", jsonutil.Indent(parsedMessageContent))
	return parsedMessage, nil
}

func isManagedInstanceIncompatibleAWSSSMDocument(documentName string) bool {
	return managedInstanceIncompatibleAWSSSMDocuments[documentName]
}

func isManagedInstance() bool {
	if strings.Contains(registration.InstanceID(), "mi-") {
		return true
	}
	return false
}
