package proc

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/identity"
	"github.com/aws/amazon-ssm-agent/common/identity/mocks"
	"github.com/aws/amazon-ssm-agent/common/runtimeconfig"
	runtimeConfigMocks "github.com/aws/amazon-ssm-agent/common/runtimeconfig/mocks"
	"github.com/stretchr/testify/assert"
)

func TestParseArgs_DocumentWorker_MissingChannel_Fail(t *testing.T) {
	input := []string{appconfig.DefaultDocumentWorker}
	channelName, err := parseArgv(input)
	assert.Error(t, err)
	assert.Equal(t, "", channelName)
}

func TestParseArgs_SessionWorker_MissingChannel_Fail(t *testing.T) {
	input := []string{appconfig.DefaultSessionWorker}
	channelName, err := parseArgv(input)
	assert.Error(t, err)
	assert.Equal(t, "", channelName)
}

func TestParseArgs_Success_OnlyChannel_Success(t *testing.T) {
	input := []string{"documentID"}
	channelName, err := parseArgv(input)
	assert.NoError(t, err)
	assert.Equal(t, "documentID", channelName)
}

func TestParseArgs_DocumentWorker_TwoArgs_Success(t *testing.T) {
	input := []string{appconfig.DefaultDocumentWorker, "documentID"}
	channelName, err := parseArgv(input)
	assert.NoError(t, err)
	assert.Equal(t, "documentID", channelName)
}

func TestParseArgs_SessionWorker_TwoArgs_Success(t *testing.T) {
	input := []string{appconfig.DefaultSessionWorker, "documentID"}
	channelName, err := parseArgv(input)
	assert.NoError(t, err)
	assert.Equal(t, "documentID", channelName)
}

func TestParseArgs_TwoArgs_Success(t *testing.T) {
	input := []string{"documentID", "something"}
	channelName, err := parseArgv(input)
	assert.NoError(t, err)
	assert.Equal(t, "documentID", channelName)
}

func TestParseArgs_DocumentWorker_ThreeArgs_Success(t *testing.T) {
	input := []string{appconfig.DefaultDocumentWorker, "documentID", "somethingElse"}
	channelName, err := parseArgv(input)
	assert.NoError(t, err)
	assert.Equal(t, "documentID", channelName)
}

func TestParseArgs_SessionWorker_ThreeArgs_Success(t *testing.T) {
	input := []string{appconfig.DefaultSessionWorker, "documentID", "somethingElse"}
	channelName, err := parseArgv(input)
	assert.NoError(t, err)
	assert.Equal(t, "documentID", channelName)
}

func TestParseArgs_ThreeArgs_Fail(t *testing.T) {
	input := []string{"documentID", "something", "else"}
	channelName, err := parseArgv(input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "number mismatch")
	assert.Equal(t, "", channelName)
}

func TestParseArgs_DocumentWorker_MultipleArgs_Fail(t *testing.T) {
	input := []string{appconfig.DefaultDocumentWorker, "documentID", "something", "else"}
	channelName, err := parseArgv(input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "number mismatch")
	assert.Equal(t, "", channelName)
}

func TestParseArgs_DocumentWorker_NilArgs_Fail(t *testing.T) {
	channelName, err := parseArgv(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "number mismatch")
	assert.Equal(t, "", channelName)
}

func TestParseArgs_SessionWorker_MultipleArgs_Fail(t *testing.T) {
	input := []string{appconfig.DefaultSessionWorker, "documentID", "something", "else"}
	channelName, err := parseArgv(input)
	assert.Error(t, err)
	assert.Equal(t, "", channelName)
}

func TestInitializeWorkerDependencies_GetConfigFailed(t *testing.T) {
	oldGetAppConfig := getAppConfig
	defer func() {
		getAppConfig = oldGetAppConfig
	}()

	getAppConfig = func(bool) (appconfig.SsmagentConfig, error) {
		return appconfig.DefaultConfig(), fmt.Errorf("SomeConfigError")
	}

	cfg, agentIdentity, channel, err := InitializeWorkerDependencies(log.NewMockLog(), []string{appconfig.DefaultSessionWorker, "documentID"})
	assert.Nil(t, cfg)
	assert.Nil(t, agentIdentity)
	assert.Empty(t, channel)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to initialize config")
}

func TestInitializeWorkerDependencies_ParseArgsFailedFailed(t *testing.T) {
	oldGetAppConfig := getAppConfig
	defer func() {
		getAppConfig = oldGetAppConfig
	}()

	getAppConfig = func(bool) (appconfig.SsmagentConfig, error) {
		return appconfig.DefaultConfig(), nil
	}

	cfg, agentIdentity, channel, err := InitializeWorkerDependencies(log.NewMockLog(), []string{"too", "many", "args", "passed"})
	assert.Nil(t, cfg)
	assert.Nil(t, agentIdentity)
	assert.Empty(t, channel)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse args")
}

func TestInitializeWorkerDependencies_GetAgentIdentityFailed(t *testing.T) {
	oldGetAppConfig := getAppConfig
	oldNewAgentIdentity := newAgentIdentity
	defer func() {
		getAppConfig = oldGetAppConfig
		newAgentIdentity = oldNewAgentIdentity
	}()

	oldGetAppConfig = func(bool) (appconfig.SsmagentConfig, error) {
		return appconfig.DefaultConfig(), nil
	}

	newAgentIdentity = func(log.T, *appconfig.SsmagentConfig, identity.IAgentIdentitySelector) (identity identity.IAgentIdentity, err error) {
		return nil, fmt.Errorf("FailedGetIdentity")
	}

	cfg, agentIdentity, channel, err := InitializeWorkerDependencies(log.NewMockLog(), []string{appconfig.DefaultSessionWorker, "documentID"})
	assert.Nil(t, cfg)
	assert.Nil(t, agentIdentity)
	assert.Empty(t, channel)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get identity")
}

func TestInitializeWorkerDependencies_RuntimeConfigIdentity_Success(t *testing.T) {
	oldGetAppConfig := getAppConfig
	oldNewAgentIdentity := newAgentIdentity
	defer func() {
		getAppConfig = oldGetAppConfig
		newAgentIdentity = oldNewAgentIdentity
	}()

	oldGetAppConfig = func(bool) (appconfig.SsmagentConfig, error) {
		return appconfig.DefaultConfig(), nil
	}

	runtimeConfigClientCreator = func() runtimeconfig.IIdentityRuntimeConfigClient {
		runtimeConfigClient := &runtimeConfigMocks.IIdentityRuntimeConfigClient{}
		runtimeConfigClient.On("ConfigExists").Return(true, nil)
		return runtimeConfigClient
	}
	runtimeSelectorVisited := false
	runtimeSelector := runtimeConfigIdentitySelectorCreator(log.NewMockLog())
	runtimeConfigIdentitySelectorCreator = func(log log.T) identity.IAgentIdentitySelector {
		runtimeSelectorVisited = true
		return runtimeSelector
	}

	expectedAgentIdentity := &mocks.IAgentIdentity{}
	newAgentIdentity = func(log.T, *appconfig.SsmagentConfig, identity.IAgentIdentitySelector) (identity identity.IAgentIdentity, err error) {
		return expectedAgentIdentity, nil
	}

	cfg, agentIdentity, channel, err := InitializeWorkerDependencies(log.NewMockLog(), []string{appconfig.DefaultSessionWorker, "documentID"})
	assert.NotNil(t, cfg)
	assert.Equal(t, appconfig.DefaultConfig(), *cfg)
	assert.Equal(t, expectedAgentIdentity, agentIdentity)
	assert.True(t, runtimeSelectorVisited, "did not visit runtime identity selector")
	assert.Equal(t, "documentID", channel)
	assert.NoError(t, err)
}

func TestInitializeWorkerDependencies_DefaultIdentity_Success(t *testing.T) {
	oldGetAppConfig := getAppConfig
	oldNewAgentIdentity := newAgentIdentity
	defer func() {
		getAppConfig = oldGetAppConfig
		newAgentIdentity = oldNewAgentIdentity
	}()

	oldGetAppConfig = func(bool) (appconfig.SsmagentConfig, error) {
		return appconfig.DefaultConfig(), nil
	}

	runtimeConfigClientCreator = func() runtimeconfig.IIdentityRuntimeConfigClient {
		runtimeConfigClient := &runtimeConfigMocks.IIdentityRuntimeConfigClient{}
		runtimeConfigClient.On("ConfigExists").Return(false, nil)
		return runtimeConfigClient
	}

	defaultIdentitySelectorVisited := false
	defaultIdentitySelector := defaultAgentIdentitySelectorCreator(log.NewMockLog())

	defaultAgentIdentitySelectorCreator = func(log log.T) identity.IAgentIdentitySelector {
		defaultIdentitySelectorVisited = true
		return defaultIdentitySelector
	}

	expectedAgentIdentity := &mocks.IAgentIdentity{}
	newAgentIdentity = func(log.T, *appconfig.SsmagentConfig, identity.IAgentIdentitySelector) (identity identity.IAgentIdentity, err error) {
		return expectedAgentIdentity, nil
	}

	cfg, agentIdentity, channel, err := InitializeWorkerDependencies(log.NewMockLog(), []string{appconfig.DefaultSessionWorker, "documentID"})
	assert.NotNil(t, cfg)
	assert.Equal(t, appconfig.DefaultConfig(), *cfg)
	assert.Equal(t, expectedAgentIdentity, agentIdentity)
	assert.True(t, defaultIdentitySelectorVisited, "did not visit default identity selector")
	assert.Equal(t, "documentID", channel)
	assert.NoError(t, err)
}
