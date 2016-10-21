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
	"fmt"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/association/cache"
	"github.com/aws/amazon-ssm-agent/agent/association/model"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	ssmsvc "github.com/aws/amazon-ssm-agent/agent/ssm"
	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/twinj/uuid"
)

const stopPolicyErrorThreshold = 10

// T represents interface for association
type T interface {
	CreateNewServiceIfUnHealthy(log log.T)
	ListInstanceAssociations(log log.T, instanceID string) ([]*model.AssociationRawData, error)
	LoadAssociationDetail(log log.T, assoc *model.AssociationRawData) error
	UpdateAssociationStatus(
		log log.T,
		instanceID string,
		name string,
		status string,
		message string,
		agentInfo *contracts.AgentInfo) (*ssm.UpdateAssociationStatusOutput, error)
	UpdateInstanceAssociationStatus(
		log log.T,
		associationID string,
		instanceID string,
		status string,
		errorCode string,
		executionDate string,
		executionSummary string) (*ssm.UpdateInstanceAssociationStatusOutput, error)
}

// AssociationService wraps the Ssm Service
type AssociationService struct {
	ssmSvc     ssmsvc.Service
	stopPolicy *sdkutil.StopPolicy
	name       string
}

// NewAssociationService returns a new association service
func NewAssociationService(name string) *AssociationService {
	ssmService := ssmsvc.NewService()
	policy := sdkutil.NewStopPolicy(name, stopPolicyErrorThreshold)
	svc := AssociationService{
		ssmSvc:     ssmService,
		stopPolicy: policy,
		name:       name,
	}

	return &svc
}

// CreateNewServiceIfUnHealthy checks service healthy and create new service if original is unhealthy
func (s *AssociationService) CreateNewServiceIfUnHealthy(log log.T) {
	if s.stopPolicy == nil {
		log.Debugf("creating new stop-policy.")
		s.stopPolicy = sdkutil.NewStopPolicy(s.name, stopPolicyErrorThreshold)
	}

	log.Debugf("assocProcessor's stoppolicy before polling is %v", s.stopPolicy)
	if !s.stopPolicy.IsHealthy() {
		log.Errorf("assocProcessor stopped temporarily due to internal failure. We will retry automatically")

		// reset stop policy and let the scheduler start the polling after pollMessageFrequencyMinutes timeout
		s.stopPolicy.ResetErrorCount()
		s.ssmSvc = ssmsvc.NewService()
		return
	}
}

// ListInstanceAssociations will get the Association and related document string
func (s *AssociationService) ListInstanceAssociations(log log.T, instanceID string) ([]*model.AssociationRawData, error) {
	uuid.SwitchFormat(uuid.CleanHyphen)
	results := []*model.AssociationRawData{}

	response, err := s.ssmSvc.ListInstanceAssociations(log, instanceID, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve associations %v", err)
	}

	for {
		// Get the association from the response of the ListAssociations call
		for _, assoc := range response.Associations {
			rawData := &model.AssociationRawData{}
			rawData.Association = assoc
			rawData.CreateDate = time.Now()
			results = append(results, rawData)
		}

		if response.NextToken == nil || *response.NextToken == "" {
			break
		}

		if response, err = s.ssmSvc.ListInstanceAssociations(log, instanceID, response.NextToken); err != nil {
			return results, fmt.Errorf("unable to retrieve associations %v", err)
		}
	}

	log.Debug("Number of associations is ", len(results))
	return results, nil
}

// UpdateInstanceAssociationStatus will get the Association and related document string
func (s *AssociationService) UpdateInstanceAssociationStatus(
	log log.T,
	associationID string,
	instanceID string,
	status string,
	errorCode string,
	executionDate string,
	executionSummary string) (*ssm.UpdateInstanceAssociationStatusOutput, error) {

	date := times.ParseIso8601UTC(executionDate)
	executionResult := ssm.InstanceAssociationExecutionResult{
		Status:           aws.String(status),
		ErrorCode:        aws.String(errorCode),
		ExecutionDate:    aws.Time(date),
		ExecutionSummary: aws.String(executionSummary),
	}

	executionResultContent, err := jsonutil.Marshal(executionResult)
	if err != nil {
		log.Error("could not marshal associationStatus! ", err)
		return nil, err
	}
	log.Debug("Update association status content is ", jsonutil.Indent(executionResultContent))

	response, err := s.ssmSvc.UpdateInstanceAssociationStatus(log, associationID, instanceID, &executionResult)
	if err != nil {
		return nil, fmt.Errorf("unable to update association status %v", err)
	}

	return response, nil
}

// LoadAssociationDetail loads document contents and parameters for the given association
func (s *AssociationService) LoadAssociationDetail(log log.T, assoc *model.AssociationRawData) error {
	associationCache := cache.GetCache()
	associationID := assoc.Association.AssociationId

	// check if the association details have been cached
	if associationCache.IsCached(*associationID) {
		rawData := associationCache.Get(*associationID)
		assoc.Document = rawData.Document
		return nil
	}

	// if not cached before
	var (
		documentResponse *ssm.GetDocumentOutput
		err              error
	)

	// Call getDocument and retrieve the document json string
	if documentResponse, err = s.ssmSvc.GetDocument(log, *assoc.Association.Name, *assoc.Association.DocumentVersion); err != nil {
		log.Errorf("unable to retrieve document, %v", err)
		return err
	}

	assoc.Document = documentResponse.Content

	if err = associationCache.Add(*associationID, assoc); err != nil {
		return err
	}

	return nil
}

// UpdateAssociationStatus update association status
func (s *AssociationService) UpdateAssociationStatus(
	log log.T,
	instanceID string,
	name string,
	status string,
	message string,
	agentInfo *contracts.AgentInfo) (*ssm.UpdateAssociationStatusOutput, error) {
	var result *ssm.UpdateAssociationStatusOutput

	agentInfoContent, err := jsonutil.Marshal(agentInfo)
	if err != nil {
		log.Error("could not marshal agentInfo! ", err)
		return nil, err
	}
	log.Debug("Update association status")

	currentTime := time.Now().UTC()
	associationStatus := ssm.AssociationStatus{
		Name:           aws.String(status),
		Message:        aws.String(message),
		Date:           &currentTime,
		AdditionalInfo: &agentInfoContent,
	}

	associationStatusContent, err := jsonutil.Marshal(associationStatus)
	if err != nil {
		log.Error("could not marshal associationStatus! ", err)
		return nil, err
	}
	log.Debug("Update association status content is ", jsonutil.Indent(associationStatusContent))

	// Call getDocument and retrieve the document json string
	if result, err = s.ssmSvc.UpdateAssociationStatus(
		log,
		instanceID,
		name,
		&associationStatus); err != nil {
		log.Errorf("unable to update association status, %v", err)
		sdkutil.HandleAwsError(log, err, s.stopPolicy)
		return nil, err
	}

	return result, nil
}
