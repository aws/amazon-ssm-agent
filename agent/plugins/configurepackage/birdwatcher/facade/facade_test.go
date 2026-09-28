package facade

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	mockContext "github.com/aws/amazon-ssm-agent/agent/mocks/context"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/stretchr/testify/assert"
)

func TestNewBirdwatcherFacade(t *testing.T) {
	testCases := []struct {
		name          string
		setupContext  func() context.T
		expectedCheck func(*ssm.Client) bool
	}{
		{
			name: "Default Configuration",
			setupContext: func() context.T {
				mockContext := mockContext.NewMockDefault()
				mockContext.On("AppConfig").Return(appconfig.SsmagentConfig{
					Agent: appconfig.AgentInfo{
						Name: "TestAgent",
					},
				})
				return mockContext
			},
			expectedCheck: func(ssmClient *ssm.Client) bool {
				return ssmClient != nil
			},
		},
		{
			name: "Custom Endpoint Configuration",
			setupContext: func() context.T {
				mockContext := mockContext.NewMockDefault()
				mockContext.On("AppConfig").Return(appconfig.SsmagentConfig{
					Ssm: appconfig.SsmCfg{
						Endpoint: "https://custom-ssm-endpoint.aws",
					},
					Agent: appconfig.AgentInfo{
						Name:   "TestAgent",
						Region: "us-west-2",
					},
				})
				return mockContext
			},
			expectedCheck: func(ssmClient *ssm.Client) bool {
				return ssmClient != nil
			},
		},
		{
			name: "Empty Configuration",
			setupContext: func() context.T {
				mockContext := mockContext.NewMockDefault()
				mockContext.On("AppConfig").Return(appconfig.SsmagentConfig{})
				return mockContext
			},
			expectedCheck: func(ssmClient *ssm.Client) bool {
				return ssmClient != nil
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			mockContext := tc.setupContext()

			// Execute
			birdwatcherFacade := NewBirdwatcherFacade(mockContext)

			// Assert
			ssmClient, ok := birdwatcherFacade.(*ssm.Client)
			assert.True(t, ok, "Expected SSM client type")
			assert.True(t, tc.expectedCheck(ssmClient), "Client creation failed for test case")
		})
	}
}
