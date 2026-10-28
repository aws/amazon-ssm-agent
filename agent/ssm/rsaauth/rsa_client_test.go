package rsaauth

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/agent/ssm/authtokenrequest"
	"github.com/aws/amazon-ssm-agent/agent/ssm/rsaauth/mocks"
	iirprovidermocks "github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/iirprovider/mocks"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNewRsaService(t *testing.T) {
	// Arrange
	awsConfig := aws.Config{
		Region:       "us-west-2",
		BaseEndpoint: aws.String("resolved.ssm.domain"),
	}

	agentConfig := &appconfig.SsmagentConfig{
		Ssm: appconfig.SsmCfg{
			Endpoint: "ssm.domain.override",
		},
	}

	credentials := credentials.StaticCredentialsProvider{}
	ssmClient := ssm.NewFromConfig(awsConfig)
	ssmSdk := ssmClient

	var capturedConfig aws.Config

	authTokenClient := &authtokenrequest.Client{}
	mockDependencies := &mocks.IRsaClientDeps{}
	mockDependencies.On("AwsConfig", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(awsConfig)
	mockDependencies.On("NewStaticCredentialsProvider", mock.Anything, mock.Anything, mock.Anything).Return(credentials)
	mockDependencies.On("NewSsmSdk", mock.MatchedBy(func(cfg aws.Config) bool {
		capturedConfig = cfg
		return true
	})).Return(ssmSdk)
	mockDependencies.On("NewAuthTokenClient", mock.Anything).Return(authTokenClient)
	deps = mockDependencies
	// Act
	svc := NewRsaClient(log.NewMockLog(), agentConfig, "i-123456789012", "SomeRegion", "SomePrivateKey")

	// Assert
	assert.NotNil(t, svc)
	assert.Equal(t, "https://"+agentConfig.Ssm.Endpoint, *capturedConfig.BaseEndpoint, "app config endpoint should overwrite awsConfig endpoint")
}

func TestNewIirRsaAuth(t *testing.T) {
	// Arrange
	awsConfig := aws.Config{
		Region:       "us-west-2",
		BaseEndpoint: aws.String("resolved.ssm.domain"),
	}

	agentConfig := &appconfig.SsmagentConfig{
		Ssm: appconfig.SsmCfg{
			Endpoint: "ssm.domain.override",
		},
	}

	var capturedConfig aws.Config

	credentials := &credentials.StaticCredentialsProvider{}
	ssmClient := ssm.NewFromConfig(awsConfig)
	ssmSdk := ssmClient

	authTokenService := &authtokenrequest.Client{}
	mockImdsClient := &iirprovidermocks.IEC2MdsSdkClient{}
	mockDependencies := &mocks.IRsaClientDeps{}

	mockDependencies.On("AwsConfig", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(awsConfig)
	mockDependencies.On("NewCredentials", mock.Anything).Return(credentials)

	mockDependencies.On("NewSsmSdk", mock.MatchedBy(func(cfg aws.Config) bool {
		capturedConfig = cfg
		return true
	})).Return(ssmSdk)

	mockDependencies.On("NewAuthTokenClient", mock.Anything).Return(authTokenService)
	deps = mockDependencies

	// Act
	svc := NewIirRsaClient(log.NewMockLog(), agentConfig, mockImdsClient, "SomeRegion", "SomePrivateKey")

	// Assert
	assert.NotNil(t, svc)
	assert.Equal(t, "https://"+agentConfig.Ssm.Endpoint, *capturedConfig.BaseEndpoint, "app config endpoint should overwrite awsConfig endpoint")
}

// endpointCapturingDeps uses the real dependencies and records the SSM client's resolved BaseEndpoint.
type endpointCapturingDeps struct {
	rsaClientDeps
	resolvedEndpoint string
}

func (d *endpointCapturingDeps) NewSsmSdk(cfg aws.Config) *ssm.Client {
	client := d.rsaClientDeps.NewSsmSdk(cfg)
	d.resolvedEndpoint = aws.ToString(client.Options().BaseEndpoint)
	return client
}

func TestRsaClients_SchemelessSsmEndpoint_ResolvesToHttps(t *testing.T) {
	vpceEndpoint := "vpce-0123456789abcdef0-abcdefgh.ssm.cn-north-1.vpce.amazonaws.com.cn"
	agentConfig := appconfig.DefaultConfig()
	agentConfig.Ssm.Endpoint = vpceEndpoint

	originalDeps := deps
	defer func() { deps = originalDeps }()

	t.Run("NewRsaClient", func(t *testing.T) {
		capturingDeps := &endpointCapturingDeps{}
		deps = capturingDeps

		NewRsaClient(log.NewMockLog(), &agentConfig, "mi-0123456789abcdef0", "cn-north-1", "SomePrivateKey")

		assert.Equal(t, "https://"+vpceEndpoint, capturingDeps.resolvedEndpoint)
	})

	t.Run("NewIirRsaClient", func(t *testing.T) {
		capturingDeps := &endpointCapturingDeps{}
		deps = capturingDeps

		NewIirRsaClient(log.NewMockLog(), &agentConfig, &iirprovidermocks.IEC2MdsSdkClient{}, "cn-north-1", "SomePrivateKey")

		assert.Equal(t, "https://"+vpceEndpoint, capturingDeps.resolvedEndpoint)
	})
}
