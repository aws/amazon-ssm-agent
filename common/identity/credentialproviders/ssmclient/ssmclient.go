// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License").
// You may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ssmclient

import (
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/ssm/util"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
)

// ISSMClient defines the functions needed for role providers send health pings to Systems Manager
type ISSMClient interface {
	UpdateInstanceInformation(input *ssm.UpdateInstanceInformationInput) (*ssm.UpdateInstanceInformationOutput, error)
}

// Initializer is a function that initializes and returns an ISSMClient
type Initializer func(log log.T, appConfig *appconfig.SsmagentConfig, credentials *credentials.Credentials, region, defaultEndpoint string) ISSMClient

// NewV4ServiceWithCreds creates a ssm.SSM that is configured to sign requests to the SSM API with the given credentials
func NewV4ServiceWithCreds(log log.T, appConfig *appconfig.SsmagentConfig, credentials *credentials.Credentials, region, defaultEndpoint string) ISSMClient {
	awsConfig := util.AwsConfig(log, *appConfig, "ssm", region)

	awsConfig.Region = &region
	awsConfig.Credentials = credentials
	if appConfig.Ssm.Endpoint != "" {
		awsConfig.Endpoint = &appConfig.Ssm.Endpoint
	} else if defaultEndpoint != "" {
		awsConfig.Endpoint = &defaultEndpoint
	}

	// Create a session to share service client Config and handlers with
	ssmSess, _ := session.NewSession(awsConfig)
	ssmSess.Handlers.Build.PushBack(request.MakeAddToUserAgentHandler(appConfig.Agent.Name, appConfig.Agent.Version))

	return ssm.New(ssmSess)
}
