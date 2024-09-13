package ssmclient

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/ssm"
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
	credentials := &credentials.Credentials{}
	region := "us-east-1"
	defaultSsmEndpoint := "ssm.com.test"
	serviceSession := NewV4ServiceWithCreds(logger, credentials, region, defaultSsmEndpoint).(*ssm.SSM)
	assert.Equal(t, defaultSsmEndpoint, *serviceSession.Config.Endpoint, "Endpoint mismatch")
	assert.Equal(t, credentials, serviceSession.Config.Credentials, "credential mismatch")
	assert.Equal(t, region, *serviceSession.Config.Region, "region mismatch")
}

func TestSSMClient_AppConfigLoad_EndpointInConfig_Success(t *testing.T) {
	ssmEndpoint := "ssm.com.test.main"
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
	credentials := &credentials.Credentials{}
	region := "us-east-1"
	defaultSsmEndpoint := "ssm.com.test"
	serviceSession := NewV4ServiceWithCreds(logger, credentials, region, defaultSsmEndpoint).(*ssm.SSM)
	assert.Equal(t, ssmEndpoint, *serviceSession.Config.Endpoint, "Endpoint mismatch")
	assert.Equal(t, credentials, serviceSession.Config.Credentials, "credential mismatch")
	assert.Equal(t, region, *serviceSession.Config.Region, "region mismatch")
}

func TestSSMClient_AppConfigLoadErrorWithEmptyConfig_Success(t *testing.T) {
	loadAppConfig = func(reload bool) (appconfig.SsmagentConfig, error) {
		return appconfig.SsmagentConfig{}, fmt.Errorf("test")
	}
	logger := log.NewMockLog()
	credentials := &credentials.Credentials{}
	region := "us-east-1"
	defaultSsmEndpoint := "ssm.com.test"
	serviceSession := NewV4ServiceWithCreds(logger, credentials, region, defaultSsmEndpoint).(*ssm.SSM)
	logger.AssertCalled(t, "Warnf", "Error while loading app config. Err: %v", mock.Anything)
	assert.Equal(t, defaultSsmEndpoint, *serviceSession.Config.Endpoint, "Endpoint mismatch")
	assert.Equal(t, credentials, serviceSession.Config.Credentials, "credential mismatch")
	assert.Equal(t, region, *serviceSession.Config.Region, "region mismatch")
}
