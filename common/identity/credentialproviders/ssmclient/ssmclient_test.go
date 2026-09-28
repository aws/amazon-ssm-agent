package ssmclient

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestSSMClient_AppConfigLoad_NoEndpointInConfig_Success(t *testing.T) {
	loadAppConfig = func(reload bool) (appconfig.SsmagentConfig, error) {
		return appconfig.SsmagentConfig{
			Agent: appconfig.AgentInfo{
				Name:    "amazon-ssm-agent-test",
				Version: "3.0.0.0",
			},
		}, nil
	}
	logger := log.NewMockLog()
	credentialsProvider := credentials.StaticCredentialsProvider{}
	region := "us-east-1"
	defaultSsmEndpoint := "ssm.com.test"
	ssmClient := NewV4ServiceWithCreds(logger, credentialsProvider, region, defaultSsmEndpoint).(*ssm.Client)
	ssmClientOptions := ssmClient.Options()
	assert.Equal(t, defaultSsmEndpoint, *ssmClientOptions.BaseEndpoint, "Endpoint mismatch")
	assert.Equal(t, credentialsProvider, ssmClientOptions.Credentials, "credential mismatch")
	assert.Equal(t, region, ssmClientOptions.Region, "region mismatch")
}

func TestSSMClient_AppConfigLoad_EndpointInConfig_Success(t *testing.T) {
	ssmEndpoint := "ssm.com.test.main"
	// NewV4ServiceWithCreds now normalizes scheme-less appconfig overrides so
	// SDK v2's URI validator accepts them (v1's endpoints.AddScheme used to do
	// this automatically). Expect the normalized form.
	expectedEndpoint := "https://" + ssmEndpoint
	loadAppConfig = func(reload bool) (appconfig.SsmagentConfig, error) {
		return appconfig.SsmagentConfig{
			Agent: appconfig.AgentInfo{
				Name:    "amazon-ssm-agent-test",
				Version: "3.0.0.0",
			},
			Ssm: appconfig.SsmCfg{Endpoint: ssmEndpoint},
		}, nil
	}
	logger := log.NewMockLog()
	credentialsProvider := credentials.StaticCredentialsProvider{}
	region := "us-east-1"
	defaultSsmEndpoint := "ssm.com.test"
	ssmClient := NewV4ServiceWithCreds(logger, credentialsProvider, region, defaultSsmEndpoint).(*ssm.Client)
	ssmClientOptions := ssmClient.Options()
	assert.Equal(t, expectedEndpoint, *ssmClientOptions.BaseEndpoint, "Endpoint mismatch")
	assert.Equal(t, credentialsProvider, ssmClientOptions.Credentials, "credential mismatch")
	assert.Equal(t, region, ssmClientOptions.Region, "region mismatch")
}

func TestSSMClient_AppConfigLoadErrorWithEmptyConfig_Success(t *testing.T) {
	loadAppConfig = func(reload bool) (appconfig.SsmagentConfig, error) {
		return appconfig.SsmagentConfig{}, fmt.Errorf("test")
	}
	logger := log.NewMockLog()
	credentialsProvider := credentials.StaticCredentialsProvider{}
	region := "us-east-1"
	defaultSsmEndpoint := "ssm.com.test"
	ssmClient := NewV4ServiceWithCreds(logger, credentialsProvider, region, defaultSsmEndpoint).(*ssm.Client)
	ssmClientOptions := ssmClient.Options()
	logger.AssertCalled(t, "Warnf", "Error while loading app config. Err: %v", mock.Anything)
	assert.Equal(t, defaultSsmEndpoint, *ssmClientOptions.BaseEndpoint, "Endpoint mismatch")
	assert.Equal(t, credentialsProvider, ssmClientOptions.Credentials, "credential mismatch")
	assert.Equal(t, region, ssmClientOptions.Region, "region mismatch")
}
