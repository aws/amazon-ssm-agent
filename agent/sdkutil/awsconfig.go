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

// Package sdkutil provides utilities used to call awssdk.
package sdkutil

import (
	"net/http"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/managedInstances/registration"
	"github.com/aws/amazon-ssm-agent/agent/managedInstances/rolecreds"
	"github.com/aws/amazon-ssm-agent/agent/network"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil/retryer"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/defaults"
)

// AwsConfig returns the default aws.Config object with the appropriate
// credentials. Callers should override returned config properties with any
// values they want for service specific overrides.
func AwsConfig(log log.T) (awsConfig *aws.Config) {
	region, _ := platform.Region()
	return AwsConfigForRegion(log, region)
}

// AwsConfigForRegion returns the default aws.Config object with the appropriate
// credentials and the specified region. Callers should override returned config
// properties with any values they want for service specific overrides.
func AwsConfigForRegion(log log.T, region string) (awsConfig *aws.Config) {
	// create default config
	awsConfig = &aws.Config{
		Retryer:    newRetryer(),
		SleepDelay: sleepDelay,
	}

	// update region if given
	if region != "" {
		awsConfig.Region = &region
	}

	// set Http Client
	awsConfig.HTTPClient = &http.Client{
		Transport: network.GetDefaultTransport(log),
	}

	//load Task IAM credentials if applicable
	config, _ := appconfig.Config(false)
	if config.Agent.ContainerMode {
		cfg := defaults.Config()
		handlers := defaults.Handlers()
		awsConfig.Credentials = defaults.CredChain(cfg, handlers)
	}

	// load managed credentials if applicable
	if isManaged, err := registration.HasManagedInstancesCredentials(); isManaged && err == nil {
		awsConfig.Credentials =
			rolecreds.ManagedInstanceCredentialsInstance()
		return
	}

	// default credentials will be ec2/ecs credentials
	awsConfig.Credentials = defaultRemoteCredentials()

	return
}

// This will return the same remote credential provider as the SDK
// We are creating this explicitly and passing it to the SDK
// because we do not care for the shared credentials / ENV credentials in the
// default SDK credential chain.
func defaultRemoteCredentials() *credentials.Credentials {
	cfg := defaults.Config()
	handlers := defaults.Handlers()
	remotecreds := defaults.RemoteCredProvider(*cfg, handlers)

	return credentials.NewCredentials(remotecreds)
}

var newRetryer = func() aws.RequestRetryer {
	r := retryer.SsmRetryer{}
	r.NumMaxRetries = 3
	return r
}

var sleepDelay = func(d time.Duration) {
	time.Sleep(d)
}
