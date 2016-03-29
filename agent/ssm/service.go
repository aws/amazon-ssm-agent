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

package ssm

import (
	"runtime"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
)

// Service is an interface to the SSM service.
type Service interface {
	ListAssociations(log log.T, instanceID string) (response *ssm.ListAssociationsOutput, err error)
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
	DeleteDocument(log log.T, instanceID string) (response *ssm.DeleteDocumentOutput, err error)
	UpdateInstanceInformation(log log.T, instanceID string, agentVersion string, agentStatus string) (response *ssm.UpdateInstanceInformationOutput, err error)
}

var ssmStopPolicy *sdkutil.StopPolicy

// sdkService is an service wrapper that delegates to the ssm sdk.
type sdkService struct {
	sdk *ssm.SSM
}

// NewService creates a new SSM service instance.
func NewService(log log.T) Service {
	appConfig, err := appconfig.GetConfig(false)
	if err != nil {
		log.Error("failed to read appconfig.")
	}

	if ssmStopPolicy == nil {
		// create a stop policy where we will stop after 10 consecutive errors and if time period expires.
		ssmStopPolicy = sdkutil.NewStopPolicy("ssmService", 10)
	}

	awsConfig := sdkutil.GetAwsConfig()

	if appConfig.Agent.Region != "" {
		awsConfig.Region = &appConfig.Agent.Region
	}

	if appConfig.Ssm.Endpoint != "" {
		awsConfig.Endpoint = &appConfig.Ssm.Endpoint
	}
	ssmService := ssm.New(session.New(awsConfig))
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

//UpdateInstanceInformation calls the UpdateInstanceInformation SSM API.
func (svc *sdkService) UpdateInstanceInformation(log log.T,
	instanceID string,
	agentVersion string,
	agentStatus string) (response *ssm.UpdateInstanceInformationOutput, err error) {

	platformType := ssm.PlatformTypeLinux
	platformName := ""
	platformVersion := ""
	switch runtime.GOOS {
	case "windows":
		platformType = ssm.PlatformTypeWindows
	default:
	}

	platformName, err = platform.GetPlatformName(log)
	if err != nil {
		sdkutil.HandleAwsError(log, err, ssmStopPolicy)
		return
	}
	platformVersion, err = platform.GetPlatformVersion(log)
	if err != nil {
		sdkutil.HandleAwsError(log, err, ssmStopPolicy)
		return
	}

	params := ssm.UpdateInstanceInformationInput{
		InstanceId:      aws.String(instanceID),
		AgentVersion:    aws.String(agentVersion),
		AgentStatus:     aws.String(agentStatus),
		PlatformType:    aws.String(platformType),
		PlatformName:    aws.String(platformName),
		PlatformVersion: aws.String(platformVersion),
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
