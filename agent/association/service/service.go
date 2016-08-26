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
	"github.com/aws/amazon-ssm-agent/agent/association/model"
	"github.com/aws/amazon-ssm-agent/agent/log"
	ssmsvc "github.com/aws/amazon-ssm-agent/agent/ssm"
	"github.com/aws/aws-sdk-go/service/ssm"
)

// ListAssociations will get the Association and related document string
func ListAssociations(log log.T, ssmSvc ssmsvc.Service, instanceID string) (model.AssociationDetail, error) {
	assocDetail := model.AssociationDetail{}
	var documentResponse *ssm.GetDocumentOutput

	response, err := ssmSvc.ListAssociations(log, instanceID)
	if err != nil {
		log.Errorf("unable to retrieve associations %v", err)
		return assocDetail, err
	}

	// Parse the association from the response of the ListAssociations call
	if assocDetail.Association, err = parseListAssociationsResponse(response, log); err != nil {
		log.Errorf("unable to parse association %v", err)
		return assocDetail, err
	}

	// Call getDocument and retrieve the document json string
	if documentResponse, err = ssmSvc.GetDocument(log, *assocDetail.Association.Name); err != nil {
		log.Errorf("unable to retrieve document, %v", err)
		return assocDetail, err
	}
	assocDetail.Document = documentResponse.Content
	return assocDetail, nil
}

func parseListAssociationsResponse(
	response *ssm.ListAssociationsOutput,
	log log.T) (association *ssm.Association, err error) {

	if response == nil || len(response.Associations) < 1 {
		return nil, log.Error("ListAssociationsResponse is empty")
	}

	return response.Associations[0], nil
}
