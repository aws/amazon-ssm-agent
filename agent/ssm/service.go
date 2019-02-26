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

package ssm

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
)

// Service is an interface to the SSM service.
type Service interface {
	ListAssociations(log log.T, instanceID string) (response *ssm.ListAssociationsOutput, err error)
	ListInstanceAssociations(log log.T, instanceID string, nextToken *string) (response *ssm.ListInstanceAssociationsOutput, err error)
	UpdateAssociationStatus(
		log log.T,
		instanceID string,
		name string,
		associationStatus *ssm.AssociationStatus) (response *ssm.UpdateAssociationStatusOutput, err error)
	UpdateInstanceAssociationStatus(
		log log.T,
		associationID string,
		instanceID string,
		executionResult *ssm.InstanceAssociationExecutionResult) (response *ssm.UpdateInstanceAssociationStatusOutput, err error)
	PutComplianceItems(
		log log.T,
		executionTime *time.Time,
		executionType string,
		executionId string,
		instanceId string,
		complianceType string,
		itemContentHash string,
		items []*ssm.ComplianceItemEntry) (response *ssm.PutComplianceItemsOutput, err error)
	SendCommand(log log.T,
		documentName string,
		instanceIDs []string,
		parameters map[string][]*string,
		timeoutSeconds *int64,
		outputS3BucketName *string,
		outputS3KeyPrefix *string) (response *ssm.SendCommandOutput, err error)
	ListCommands(log log.T, instanceID string) (response *ssm.ListCommandsOutput, err error)
	ListCommandInvocations(log log.T, instanceID string, commandID string) (response *ssm.ListCommandInvocationsOutput, err error)
	CancelCommand(log log.T, commandID string, instanceIDs []string) (response *ssm.CancelCommandOutput, err error)
	CreateDocument(log log.T, docName string, docContent string) (response *ssm.CreateDocumentOutput, err error)
	GetDocument(log log.T, docName string, docVersion string) (response *ssm.GetDocumentOutput, err error)
	DeleteDocument(log log.T, instanceID string) (response *ssm.DeleteDocumentOutput, err error)
	DescribeAssociation(log log.T, instanceID string, docName string) (response *ssm.DescribeAssociationOutput, err error)
	UpdateInstanceInformation(log log.T, agentVersion, agentStatus, agentName string) (response *ssm.UpdateInstanceInformationOutput, err error)
	UpdateEmptyInstanceInformation(log log.T, agentVersion, agentName string) (response *ssm.UpdateInstanceInformationOutput, err error)
	GetParameters(log log.T, paramNames []string) (response *ssm.GetParametersOutput, err error)
	GetDecryptedParameters(log log.T, paramNames []string) (response *ssm.GetParametersOutput, err error)
}

var ssmStopPolicy *sdkutil.StopPolicy

// sdkService is an service wrapper that delegates to the ssm sdk.
type sdkService struct {
	sdk ssmiface.SSMAPI
}

// NewService creates a new SSM service instance.
func NewService() Service {
	if ssmStopPolicy == nil {
		// create a stop policy where we will stop after 10 consecutive errors and if time period expires.
		ssmStopPolicy = sdkutil.NewStopPolicy("ssmService", 10)
	}

	awsConfig := sdkutil.AwsConfig()
	// parse appConfig overrides
	appConfig, err := appconfig.Config(false)
	if err == nil {
		if appConfig.Ssm.Endpoint != "" {
			awsConfig.Endpoint = &appConfig.Ssm.Endpoint
		} else {
			if region, err := platform.Region(); err == nil {
				if defaultEndpoint := appconfig.GetDefaultEndPoint(region, "ssm"); defaultEndpoint != "" {
					awsConfig.Endpoint = &defaultEndpoint
				}
			}
		}
		if appConfig.Agent.Region != "" {
			awsConfig.Region = &appConfig.Agent.Region
		}

		// TODO: test hook, can be removed before release
		// this is to skip ssl verification for the beta self signed certs
		if appConfig.Ssm.InsecureSkipVerify {
			tr := &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
			awsConfig.HTTPClient = &http.Client{Transport: tr}
		}
	}
	sess := session.New(awsConfig)
	sess.Handlers.Build.PushBack(request.MakeAddToUserAgentHandler(appConfig.Agent.Name, appConfig.Agent.Version))

	ssmService := ssm.New(sess)
	return NewSSMService(ssmService)
}

func NewSSMService(ssmService ssmiface.SSMAPI) Service {
	return &sdkService{sdk: ssmService}
}

func makeAwsStrings(strings []string) []*string {
	out := make([]*string, len(strings))
	for i, s := range strings {
		out[i] = aws.String(s)
	}
	return out
}

//ListAssociations calls the ListAssociations SSM API.
func (svc *sdkService) ListAssociations(log log.T, instanceID string) (response *ssm.ListAssociationsOutput, err error) {
	params := ssm.ListAssociationsInput{
		AssociationFilterList: []*ssm.AssociationFilter{
			{
				Key:   aws.String("InstanceId"),
				Value: aws.String(instanceID),
			},
		},
		MaxResults: aws.Int64(1),
	}
	response, err = svc.sdk.ListAssociations(&params)
	if err != nil {
		sdkutil.HandleAwsError(log, err, ssmStopPolicy)
		return
	}
	log.Debug("ListAssociations Response", response)
	return
}

//ListInstanceAssociations calls the ListAssociations SSM API.
func (svc *sdkService) ListInstanceAssociations(log log.T, instanceID string, nextToken *string) (response *ssm.ListInstanceAssociationsOutput, err error) {
	params := ssm.ListInstanceAssociationsInput{
		InstanceId: &instanceID,
		MaxResults: aws.Int64(20),
		NextToken:  nextToken,
	}

	response, err = svc.sdk.ListInstanceAssociations(&params)
	if err != nil {
		errCode := sdkutil.GetAwsErrorCode(err)
		if errCode != "UnknownOperationException" {
			sdkutil.HandleAwsError(log, err, ssmStopPolicy)
		}
		return
	}
	log.Debug("ListInstanceAssociations Response", response)
	return
}

// PutComplianceItem calls to PutComplianceItem SSM API.
func (svc *sdkService) PutComplianceItems(
	log log.T,
	executionTime *time.Time,
	executionType string,
	executionId string,
	instanceId string,
	complianceType string,
	itemContentHash string,
	items []*ssm.ComplianceItemEntry) (response *ssm.PutComplianceItemsOutput, err error) {

	executionSummary := &ssm.ComplianceExecutionSummary{
		ExecutionId:   aws.String(executionId),
		ExecutionType: aws.String(executionType),
		ExecutionTime: executionTime}
	params := &ssm.PutComplianceItemsInput{
		ResourceId:       aws.String(instanceId),
		ResourceType:     aws.String("ManagedInstance"),
		ComplianceType:   aws.String(complianceType),
		ExecutionSummary: executionSummary,
		ItemContentHash:  aws.String(itemContentHash),
		Items:            items,
	}

	response, err = svc.sdk.PutComplianceItems(params)
	if err != nil {
		errCode := sdkutil.GetAwsErrorCode(err)
		if errCode != "UnknownOperationException" && errCode != "AccessDeniedException" {
			sdkutil.HandleAwsError(log, err, ssmStopPolicy)
		}
		return
	}
	log.Debug("PutComplianceItems Response ", response)
	return
}

//UpdateInstanceAssociationStatus calls the ListAssociations SSM API.
func (svc *sdkService) UpdateInstanceAssociationStatus(log log.T, associationID string, instanceID string, executionResult *ssm.InstanceAssociationExecutionResult) (response *ssm.UpdateInstanceAssociationStatusOutput, err error) {
	params := ssm.UpdateInstanceAssociationStatusInput{
		InstanceId:      &instanceID,
		AssociationId:   &associationID,
		ExecutionResult: executionResult,
	}

	response, err = svc.sdk.UpdateInstanceAssociationStatus(&params)
	if err != nil {
		sdkutil.HandleAwsError(log, err, ssmStopPolicy)
		return
	}
	log.Debug("UpdateInstanceAssociationStatus Response ", response)
	return
}

//UpdateAssociationStatus calls the UpdateAssociationStatus SSM API.
func (svc *sdkService) UpdateAssociationStatus(
	log log.T,
	instanceID string,
	name string,
	associationStatus *ssm.AssociationStatus) (response *ssm.UpdateAssociationStatusOutput, err error) {

	input := ssm.UpdateAssociationStatusInput{
		InstanceId:        aws.String(instanceID),
		Name:              aws.String(name),
		AssociationStatus: associationStatus,
	}
	response, err = svc.sdk.UpdateAssociationStatus(&input)
	if err != nil {
		sdkutil.HandleAwsError(log, err, ssmStopPolicy)
		return
	}
	log.Debug("UpdateAssociationStatus Response", response)
	return
}

//UpdateInstanceInformation calls the UpdateInstanceInformation SSM API.
func (svc *sdkService) UpdateInstanceInformation(
	log log.T,
	agentVersion,
	agentStatus,
	agentName string,
) (response *ssm.UpdateInstanceInformationOutput, err error) {

	params := ssm.UpdateInstanceInformationInput{
		AgentName:    aws.String(agentName),
		AgentStatus:  aws.String(agentStatus),
		AgentVersion: aws.String(agentVersion),
	}

	goOS := runtime.GOOS
	switch goOS {
	case "windows":
		params.PlatformType = aws.String(ssm.PlatformTypeWindows)
	case "linux", "freebsd", "darwin":
		// darwin masquerades as Linux to bypass OS validation on
		// the backend until official support can be added.
		params.PlatformType = aws.String(ssm.PlatformTypeLinux)
	default:
		return nil, fmt.Errorf("Cannot report platform type of unrecognized OS. %v", goOS)
	}

	if ip, err := platform.IP(); err == nil {
		params.IPAddress = aws.String(ip)
	} else {
		log.Warn(err)
	}

	if h, err := platform.Hostname(log); err == nil {
		params.ComputerName = aws.String(h)
	} else {
		log.Warn(err)
	}
	if instID, err := platform.InstanceID(); err == nil {
		params.InstanceId = aws.String(instID)
	} else {
		log.Warn(err)
	}

	if n, err := platform.PlatformName(log); err == nil {
		params.PlatformName = aws.String(n)
	} else {
		log.Warn(err)
	}

	if v, err := platform.PlatformVersion(log); err == nil {
		params.PlatformVersion = aws.String(v)
	} else {
		log.Warn(err)
	}

	log.Debug("Calling UpdateInstanceInformation with params", params)
	response, err = svc.sdk.UpdateInstanceInformation(&params)
	if err != nil {
		sdkutil.HandleAwsError(log, err, ssmStopPolicy)
		return
	}
	log.Debug("UpdateInstanceInformation Response", response)
	return
}

//UpdateEmptyInstanceInformation calls the UpdateInstanceInformation SSM API with an empty ping.
func (svc *sdkService) UpdateEmptyInstanceInformation(
	log log.T,
	agentVersion,
	agentName string,
) (response *ssm.UpdateInstanceInformationOutput, err error) {
	//TODO: combine this with UpdateInstanceInfo
	params := ssm.UpdateInstanceInformationInput{
		AgentName:    aws.String(agentName),
		AgentVersion: aws.String(agentVersion),
	}

	goOS := runtime.GOOS
	switch goOS {
	case "windows":
		params.PlatformType = aws.String(ssm.PlatformTypeWindows)
	case "linux", "freebsd", "darwin":
		// darwin masquerades as Linux to bypass OS validation on
		// the backend until official support can be added.
		params.PlatformType = aws.String(ssm.PlatformTypeLinux)
	}

	// InstanceId is a required parameter for UpdateInstanceInformation
	if instID, err := platform.InstanceID(); err == nil {
		params.InstanceId = aws.String(instID)
	} else {
		return nil, err
	}

	log.Debug("Calling UpdateInstanceInformation with params", params)
	response, err = svc.sdk.UpdateInstanceInformation(&params)
	if err == nil {
		log.Debug("UpdateInstanceInformation Response", response)
	}
	return
}

func (svc *sdkService) CreateDocument(log log.T, docName string, docContent string) (response *ssm.CreateDocumentOutput, err error) {
	params := ssm.CreateDocumentInput{
		Content: aws.String(docContent),
		Name:    aws.String(docName),
	}
	response, err = svc.sdk.CreateDocument(&params)
	if err != nil {
		sdkutil.HandleAwsError(log, err, ssmStopPolicy)
		return
	}
	log.Debug("CreateDocument Response", response)
	return
}

//GetDocument calls the GetDocument SSM API to retrieve document with given document name
func (svc *sdkService) GetDocument(log log.T, docName string, docVersion string) (response *ssm.GetDocumentOutput, err error) {
	params := ssm.GetDocumentInput{
		Name: aws.String(docName),
	}

	if docVersion != "" {
		params.DocumentVersion = aws.String(docVersion)
	}

	response, err = svc.sdk.GetDocument(&params)
	if err != nil {
		sdkutil.HandleAwsError(log, err, ssmStopPolicy)
		return
	}
	log.Debug("GetDocument Response", response)
	return
}

//DescribeAssociation calls the DescribeAssociation SSM API to retrieve parameters information
func (svc *sdkService) DescribeAssociation(log log.T, instanceID string, docName string) (response *ssm.DescribeAssociationOutput, err error) {
	params := ssm.DescribeAssociationInput{
		InstanceId: aws.String(instanceID),
		Name:       aws.String(docName),
	}
	response, err = svc.sdk.DescribeAssociation(&params)
	if err != nil {
		sdkutil.HandleAwsError(log, err, ssmStopPolicy)
		return
	}
	log.Debug("DescribeAssociation Response", response)
	return
}

func (svc *sdkService) DeleteDocument(log log.T, docName string) (response *ssm.DeleteDocumentOutput, err error) {
	params := ssm.DeleteDocumentInput{
		Name: aws.String(docName), // Required
	}
	response, err = svc.sdk.DeleteDocument(&params)
	if err != nil {
		sdkutil.HandleAwsError(log, err, ssmStopPolicy)
		return
	}
	log.Debug("DeleteDocument Response", response)
	return
}

func (svc *sdkService) SendCommand(log log.T,
	documentName string,
	instanceIDs []string,
	parameters map[string][]*string,
	timeoutSeconds *int64,
	outputS3BucketName *string,
	outputS3KeyPrefix *string) (response *ssm.SendCommandOutput, err error) {
	params := ssm.SendCommandInput{
		DocumentName:       aws.String(documentName),
		InstanceIds:        makeAwsStrings(instanceIDs),
		Comment:            aws.String("Comment"),
		OutputS3BucketName: outputS3BucketName,
		OutputS3KeyPrefix:  outputS3KeyPrefix,
		Parameters:         parameters,
		TimeoutSeconds:     timeoutSeconds,
	}

	log.Debug("SendCommand params:", params)
	response, err = svc.sdk.SendCommand(&params)
	if err != nil {
		sdkutil.HandleAwsError(log, err, ssmStopPolicy)
		return
	}
	log.Debug("SendCommand Response", response)
	return
}

func (svc *sdkService) ListCommands(log log.T, instanceID string) (response *ssm.ListCommandsOutput, err error) {
	params := ssm.ListCommandsInput{
		//		    Filters: []*ssm.CommandFilter{
		//		        { // Required
		//		            Key:   aws.String("CommandFilterKey"),   // Required
		//		            Value: aws.String("CommandFilterValue"), // Required
		//		        },
		//		    },
		InstanceId: aws.String(instanceID),
		MaxResults: aws.Int64(25),
	}
	response, err = svc.sdk.ListCommands(&params)
	if err != nil {
		sdkutil.HandleAwsError(log, err, ssmStopPolicy)
		return
	}
	log.Debug("ListCommands Response", response)
	return
}

func (svc *sdkService) ListCommandInvocations(log log.T, instanceID string, commandID string) (response *ssm.ListCommandInvocationsOutput, err error) {
	params := ssm.ListCommandInvocationsInput{
		CommandId: aws.String(commandID),
		Details:   aws.Bool(true),
		//    Filters: []*ssm.CommandFilter{
		//        { // Required
		//            Key:   aws.String("CommandFilterKey"),   // Required
		//            Value: aws.String("CommandFilterValue"), // Required
		//        },
		//        // More values...
		//    },
		InstanceId: aws.String(instanceID),
		MaxResults: aws.Int64(25),
		//    NextToken:  aws.String("NextToken"),
	}

	response, err = svc.sdk.ListCommandInvocations(&params)
	if err != nil {
		sdkutil.HandleAwsError(log, err, ssmStopPolicy)
		return
	}
	log.Debug("ListCommandInvocations Response", response)
	return
}

func (svc *sdkService) CancelCommand(log log.T, commandID string, instanceIDs []string) (response *ssm.CancelCommandOutput, err error) {
	params := ssm.CancelCommandInput{
		CommandId: aws.String(commandID),
	}
	if len(instanceIDs) > 0 {
		params.InstanceIds = makeAwsStrings(instanceIDs)
	}
	log.Debug("CancelCommand params:", params)
	response, err = svc.sdk.CancelCommand(&params)
	if err != nil {
		sdkutil.HandleAwsError(log, err, ssmStopPolicy)
		return
	}
	log.Debug("CancelCommand Response", response)
	return
}

func (svc *sdkService) GetParameters(log log.T, paramNames []string) (response *ssm.GetParametersOutput, err error) {
	serviceParams := ssm.GetParametersInput{
		Names:          aws.StringSlice(paramNames),
		WithDecryption: aws.Bool(false),
	}

	log.Debugf("Calling GetParameters API with params - %v", serviceParams)

	if response, err = svc.sdk.GetParameters(&serviceParams); err != nil {
		errorString := fmt.Errorf("Encountered error while calling GetParameters API. Error: %v", err)
		log.Debug(err)
		sdkutil.HandleAwsError(log, err, ssmStopPolicy)
		return nil, errorString
	}
	return
}

func (svc *sdkService) GetDecryptedParameters(log log.T, paramNames []string) (response *ssm.GetParametersOutput, err error) {
	serviceParams := ssm.GetParametersInput{
		Names:          aws.StringSlice(paramNames),
		WithDecryption: aws.Bool(true),
	}

	if response, err = svc.sdk.GetParameters(&serviceParams); err != nil {
		errorString := fmt.Errorf("Encountered error while calling GetParameters API. Error: %v", err)
		log.Debug(err)
		sdkutil.HandleAwsError(log, err, ssmStopPolicy)
		return nil, errorString
	}
	return
}
