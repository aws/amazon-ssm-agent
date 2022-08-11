package rsaauth

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/ssm/authtokenrequest"
	"github.com/aws/amazon-ssm-agent/agent/ssm/rsaauth/mocks"
	iirprovidermocks "github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/iirprovider/mocks"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNewRsaService(t *testing.T) {
	// Arrange
	awsConfig := &aws.Config{
		Region:   aws.String("us-west-2"),
		Endpoint: aws.String("resolved.ssm.domain"),
	}

	agentConfig := &appconfig.SsmagentConfig{
		Ssm: appconfig.SsmCfg{
			Endpoint: "ssm.domain.override",
		},
	}

	credentials := &credentials.Credentials{}
	ssmSdk := &ssm.SSM{
		Client: &client.Client{Handlers: request.Handlers{Sign: request.HandlerList{}}},
	}

	session := &session.Session{
		Handlers: request.Handlers{
			Build: request.HandlerList{},
		},
	}

	authTokenClient := &authtokenrequest.Client{}
	mockDependencies := &mocks.IRsaClientDeps{}
	mockDependencies.On("AwsConfig", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(awsConfig)
	mockDependencies.On("NewStaticCredentials", mock.Anything, mock.Anything, mock.Anything).Return(credentials)
	mockDependencies.On("NewSession", mock.Anything).Return(session, nil)
	mockDependencies.On("MakeAddToUserAgentHandler", mock.Anything, mock.Anything).Return(func(*request.Request) {})
	mockDependencies.On("NewSsmSdk", mock.Anything).Return(ssmSdk)
	mockDependencies.On("NewAuthTokenClient", mock.Anything).Return(authTokenClient)
	deps = mockDependencies
	// Act
	svc := NewRsaClient(log.NewMockLog(), agentConfig, "i-123456789012", "SomeRegion", "SomePrivateKey")

	// Assert
	assert.NotNil(t, svc)
	assert.Equal(t, &agentConfig.Ssm.Endpoint, awsConfig.Endpoint, "app config endpoint should overwrite awsConfig endpoint")
}

func TestNewIirRsaAuth(t *testing.T) {
	// Arrange
	awsConfig := &aws.Config{
		Region:   aws.String("us-west-2"),
		Endpoint: aws.String("resolved.ssm.domain"),
	}

	agentConfig := &appconfig.SsmagentConfig{
		Ssm: appconfig.SsmCfg{
			Endpoint: "ssm.domain.override",
		},
	}

	credentials := &credentials.Credentials{}
	ssmSdk := &ssm.SSM{
		Client: &client.Client{Handlers: request.Handlers{Sign: request.HandlerList{}}},
	}

	session := &session.Session{
		Handlers: request.Handlers{
			Build: request.HandlerList{},
		},
	}

	authTokenService := &authtokenrequest.Client{}
	mockImdsClient := &iirprovidermocks.IEC2MdsSdkClient{}
	mockDependencies := &mocks.IRsaClientDeps{}
	mockDependencies.On("AwsConfig", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(awsConfig)
	mockDependencies.On("NewCredentials", mock.Anything).Return(credentials)
	mockDependencies.On("NewSession", mock.Anything).Return(session, nil)
	mockDependencies.On("MakeAddToUserAgentHandler", mock.Anything, mock.Anything).Return(func(*request.Request) {})
	mockDependencies.On("NewSsmSdk", mock.Anything).Return(ssmSdk)
	mockDependencies.On("NewAuthTokenClient", mock.Anything).Return(authTokenService)
	deps = mockDependencies

	// Act
	svc := NewIirRsaClient(log.NewMockLog(), agentConfig, mockImdsClient, "SomeRegion", "SomePrivateKey")

	// Assert
	assert.NotNil(t, svc)
	assert.Equal(t, &agentConfig.Ssm.Endpoint, awsConfig.Endpoint)
	assert.Equal(t, &agentConfig.Ssm.Endpoint, awsConfig.Endpoint, "app config endpoint should overwrite awsConfig endpoint")
}
