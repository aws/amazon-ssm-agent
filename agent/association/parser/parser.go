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

// Package parser contains utilities for parsing and encoding MDS/SSM messages.
package parser

import (
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/association/model"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docparser"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/runcommand/contracts"
	"github.com/aws/amazon-ssm-agent/agent/times"
)

// ErrorMsg represents the error message to be sent to the customer
const ErrorMsg = "Encountered error while parsing input - internal error"

// ParseDocumentForPayload parses an document and replaces the parameters where needed.
func ParseDocumentForPayload(log log.T,
	rawData *model.InstanceAssociation) (*messageContracts.SendCommandPayload, error) {

	rawDataContent, err := jsonutil.Marshal(rawData)
	if err != nil {
		log.Debugf("Could not marshal association! ", err)
		return nil, fmt.Errorf("%v", ErrorMsg)
	}
	log.Debug("Processing association ", jsonutil.Indent(rawDataContent))

	payload := &messageContracts.SendCommandPayload{}

	if err = json.Unmarshal([]byte(*rawData.Document), &payload.DocumentContent); err != nil {
		log.Debugf("Could not unmarshal parameters ", err)
		return nil, fmt.Errorf("%v", ErrorMsg)
	}

	payload.DocumentName = *rawData.Association.Name
	payload.CommandID = *rawData.Association.AssociationId

	if rawData.Association.OutputLocation != nil && rawData.Association.OutputLocation.S3Location != nil {
		if rawData.Association.OutputLocation.S3Location.OutputS3KeyPrefix != nil {
			payload.OutputS3KeyPrefix = *rawData.Association.OutputLocation.S3Location.OutputS3KeyPrefix
		}
		if rawData.Association.OutputLocation.S3Location.OutputS3BucketName != nil {
			payload.OutputS3BucketName = *rawData.Association.OutputLocation.S3Location.OutputS3BucketName
		}
	}

	payload.Parameters = docparser.ParseParameters(log, rawData.Association.Parameters, payload.DocumentContent.Parameters)

	return payload, nil
}

// InitializeDocumentState - an interim state that is used around during an execution of a document
func InitializeDocumentState(context context.T,
	payload *messageContracts.SendCommandPayload,
	rawData *model.InstanceAssociation) (contracts.DocumentState, error) {

	//initialize document information with relevant values extracted from msg
	documentInfo := newDocumentInfo(rawData, payload)
	// adapt plugin configuration format from MDS to plugin expected format
	s3KeyPrefix := path.Join(payload.OutputS3KeyPrefix, documentInfo.InstanceID, documentInfo.AssociationID, documentInfo.RunID)

	orchestrationRootDir := filepath.Join(
		appconfig.DefaultDataStorePath,
		documentInfo.InstanceID,
		appconfig.DefaultDocumentRootDirName,
		context.AppConfig().Agent.OrchestrationRootDir)

	orchestrationDir := filepath.Join(orchestrationRootDir, documentInfo.AssociationID, documentInfo.RunID)

	parserInfo := docparser.DocumentParserInfo{
		OrchestrationDir: orchestrationDir,
		S3Bucket:         payload.OutputS3BucketName,
		S3Prefix:         s3KeyPrefix,
		MessageId:        documentInfo.MessageID,
		DocumentId:       documentInfo.DocumentID,
	}

	docContent := &docparser.DocContent{
		SchemaVersion: payload.DocumentContent.SchemaVersion,
		Description:   payload.DocumentContent.Description,
		RuntimeConfig: payload.DocumentContent.RuntimeConfig,
		MainSteps:     payload.DocumentContent.MainSteps,
		Parameters:    payload.DocumentContent.Parameters,
	}
	return docparser.InitializeDocState(context.Log(), contracts.Association, docContent, documentInfo, parserInfo, payload.Parameters)
}

// newDocumentInfo initializes new DocumentInfo object
func newDocumentInfo(rawData *model.InstanceAssociation, payload *messageContracts.SendCommandPayload) contracts.DocumentInfo {

	documentInfo := new(contracts.DocumentInfo)

	documentInfo.AssociationID = *(rawData.Association.AssociationId)
	documentInfo.InstanceID = *(rawData.Association.InstanceId)
	documentInfo.MessageID = fmt.Sprintf("aws.ssm.%v.%v", documentInfo.AssociationID, documentInfo.InstanceID)
	documentInfo.RunID = times.ToIsoDashUTC(time.Now())
	documentInfo.DocumentID = *(rawData.Association.AssociationId) + "." + documentInfo.RunID
	rawData.DocumentID = documentInfo.DocumentID
	documentInfo.CreatedDate = times.ToIso8601UTC(rawData.CreateDate)
	documentInfo.DocumentName = payload.DocumentName
	documentInfo.DocumentVersion = *(rawData.Association.DocumentVersion)
	documentInfo.DocumentStatus = contracts.ResultStatusInProgress

	return *documentInfo
}
