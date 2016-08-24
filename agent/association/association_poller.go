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

// Package association polls, persists, and processes associations
package association

import (
	"errors"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
)

// PollListAssociationService can poll a newSSM service for associations.
type PollListAssociationsService struct {
	context     context.T
	region      string
	instanceID  string
	association *ssm.Association
	document    *string
}

// NewPollListAssociationService returns a new PollListAssociationService with the given context, region, and
// instanceID.
func NewPollListAssociationsService(context context.T, region string, instanceID string) *PollListAssociationsService {
	return &PollListAssociationsService{
		context:    context,
		region:     region,
		instanceID: instanceID,
	}
}

// PollListAssociations will get the Association and related document string
func (p *PollListAssociationsService) PollListAssociations() {
	log := p.context.Log()

	params := getListAssociationsInput(p.instanceID)

	// Instantiate new service to ensure less possibility of corrupted services
	ssmService := newSSM(p.region)

	response, err := ssmService.ListAssociations(params)

	if err != nil {
		log.Errorf("Error in ListAssociations Call %v", err)
		return
	}

	// Parse the association from the response of the ListAssociations call
	p.association = parseListAssociationsResponse(response, log)
	// Call getDocument and retrieve the document json string
	p.document = getDocumentFromAssociation(p.association, p.region, log)
}

func getListAssociationsInput(instanceID string) *ssm.ListAssociationsInput {
	return &ssm.ListAssociationsInput{
		AssociationFilterList: []*ssm.AssociationFilter{
			{
				Key:   aws.String("InstanceId"),
				Value: aws.String(instanceID),
			},
		},
		MaxResults: aws.Int64(1),
		NextToken:  aws.String(""),
	}
}

func parseListAssociationsResponse(response *ssm.ListAssociationsOutput, log log.T) *ssm.Association {
	if response == nil {
		log.Error("ListAssociationsResponse is nil")
		return nil
	}
	if len(response.Associations) < 1 {
		log.Error("ListAssociationsResponse is empty")
		return nil
	}
	return response.Associations[0]
}

func getDocumentFromAssociation(association *ssm.Association, region string, log log.T) *string {
	if err := validate(association); err != nil {
		log.Error("association not valid, ignoring: ", err)
		return nil
	}
	ssmService := newSSM(region)
	params := &ssm.GetDocumentInput{
		Name: association.Name, // Required
	}
	response, err := ssmService.GetDocument(params)

	if err != nil {
		// Print the error, cast err to awserr.Error to get the Code and
		// Message from an error.
		log.Error("GetDocument error,", err)
		return nil
	}
	return response.Content
}

var newSSM = func(region string) ssmiface.SSMAPI {
	session := session.New(&aws.Config{Region: &region})
	ssmService := ssm.New(session)
	return ssmService
}

func validate(association *ssm.Association) error {
	if association == nil {
		return errors.New("Association is nil")
	}
	if empty(association.InstanceId) {
		return errors.New("InstanceId is missing")
	}
	if empty(association.Name) {
		return errors.New("Name is missing")
	}
	return nil
}

// empty returns true if string is empty
func empty(s *string) bool {
	return s == nil || *s == ""
}
