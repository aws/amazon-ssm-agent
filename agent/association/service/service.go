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

// Package service wraps SSM service
package service

import (
	"time"

	"github.com/aws/amazon-ssm-agent/agent/association/model"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	ssmsvc "github.com/aws/amazon-ssm-agent/agent/ssm"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/twinj/uuid"
)

// ListAssociations will get the Association and related document string
func ListAssociations(log log.T, ssmSvc ssmsvc.Service, instanceID string) (*model.AssociationRawData, error) {
	uuid.SwitchFormat(uuid.CleanHyphen)
	assoc := &model.AssociationRawData{}

	response, err := ssmSvc.ListAssociations(log, instanceID)
	if err != nil {
		log.Errorf("unable to retrieve associations %v", err)
		return assoc, err
	}

	// Parse the association from the response of the ListAssociations call
	if assoc.Association, err = parseListAssociationsResponse(response, log); err != nil {
		log.Errorf("unable to parse association %v", err)
		return assoc, err
	}

	assoc.ID = uuid.NewV4().String()
	assoc.CreateDate = time.Now().String()

	return assoc, nil
}

// LoadAssociationDetail loads document contents and parameters for the given association
func LoadAssociationDetail(log log.T, ssmSvc ssmsvc.Service, assoc *model.AssociationRawData) error {
	var (
		documentResponse  *ssm.GetDocumentOutput
		parameterResponse *ssm.DescribeAssociationOutput
		err               error
	)

	// Call getDocument and retrieve the document json string
	if documentResponse, err = ssmSvc.GetDocument(log, *assoc.Association.Name); err != nil {
		log.Errorf("unable to retrieve document, %v", err)
		return err
	}

	// Call descriptionAssociation and retrieve the parameter json string
	if parameterResponse, err = ssmSvc.DescribeAssociation(log, *assoc.Association.InstanceId, *assoc.Association.Name); err != nil {
		log.Errorf("unable to retrieve association, %v", err)
		return err
	}

	assoc.Document = documentResponse.Content
	assoc.Parameter = parameterResponse.AssociationDescription
	return nil
}

// UpdateAssociationStatus update association status
func UpdateAssociationStatus(log log.T,
	ssmSvc ssmsvc.Service,
	instanceID string,
	name string,
	status string,
	message string,
	agentInfo *contracts.AgentInfo,
	processorStopPolicy *sdkutil.StopPolicy) (*ssm.UpdateAssociationStatusOutput, error) {
	var result *ssm.UpdateAssociationStatusOutput

	agentInfoContent, err := jsonutil.Marshal(agentInfo)
	if err != nil {
		log.Error("could not marshal agentInfo! ", err)
		return nil, err
	}
	log.Debug("Update association status")
	log.Debug("AgentInfo content ", jsonutil.Indent(agentInfoContent))
	currentTime := time.Now().UTC()
	associationStatus := ssm.AssociationStatus{
		Name:           aws.String(status),
		Message:        aws.String(message),
		Date:           &currentTime,
		AdditionalInfo: &agentInfoContent,
	}

	// Call getDocument and retrieve the document json string
	if result, err = ssmSvc.UpdateAssociationStatus(log,
		instanceID,
		name,
		&associationStatus); err != nil {

		log.Errorf("unable to update association status, %v", err)
		sdkutil.HandleAwsError(log, err, processorStopPolicy)
		return nil, err
	}

	return result, nil
}

func parseListAssociationsResponse(
	response *ssm.ListAssociationsOutput,
	log log.T) (association *ssm.Association, err error) {

	if response == nil || len(response.Associations) < 1 {
		return nil, log.Error("ListAssociationsResponse is empty")
	}

	return response.Associations[0], nil
}
