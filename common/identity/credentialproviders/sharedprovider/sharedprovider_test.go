package sharedprovider

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/mocks"
	"github.com/aws/amazon-ssm-agent/common/runtimeconfig"
	runtimeMock "github.com/aws/amazon-ssm-agent/common/runtimeconfig/mocks"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/stretchr/testify/assert"
)

func TestRetrieve_ErrGetConfig(t *testing.T) {
	expErr := fmt.Errorf("SomeGetConfigError")
	newRuntimeConfig = func() runtimeconfig.IIdentityRuntimeConfigClient {
		runtimeConfigClient := &runtimeMock.IIdentityRuntimeConfigClient{}
		runtimeConfigClient.On("GetConfig").Return(runtimeconfig.IdentityRuntimeConfig{}, expErr).Once()
		return runtimeConfigClient
	}

	var s = sharedCredentialsProvider{
		log: log.NewMockLog(),
	}

	creds, err := s.Retrieve()
	assert.ErrorIs(t, err, expErr)
	assert.Equal(t, emptyCredential, creds)
}

func TestRetrieve_ErrCredsExpired(t *testing.T) {
	config := runtimeconfig.IdentityRuntimeConfig{
		ShareFile: "SomeShareFile",
	}
	config.CredentialsExpiresAt = time.Time{}
	newRuntimeConfig = func() runtimeconfig.IIdentityRuntimeConfigClient {
		runtimeConfigClient := &runtimeMock.IIdentityRuntimeConfigClient{}
		runtimeConfigClient.On("GetConfig").Return(config, nil).Once()
		return runtimeConfigClient
	}

	var s = sharedCredentialsProvider{
		getTimeNow: func() time.Time {
			return time.Now()
		},
	}

	creds, err := s.Retrieve()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "shared credentials are already expired")
	assert.Equal(t, emptyCredential, creds)
}

func TestRetrieve_ErrShareCredsGet(t *testing.T) {
	config := runtimeconfig.IdentityRuntimeConfig{
		ShareFile: "SomeShareFile",
	}
	config.CredentialsExpiresAt = time.Now()
	newRuntimeConfig = func() runtimeconfig.IIdentityRuntimeConfigClient {
		runtimeConfigClient := &runtimeMock.IIdentityRuntimeConfigClient{}
		runtimeConfigClient.On("GetConfig").Return(config, nil).Once()
		return runtimeConfigClient
	}

	var s = sharedCredentialsProvider{
		getTimeNow: func() time.Time {
			return time.Now().Add(-time.Hour)
		},
	}

	newSharedCredentials = func(string, string) *credentials.Credentials {
		provider := &mocks.Provider{}
		provider.On("Retrieve").Return(credentials.Value{}, fmt.Errorf("SomeGetCredsErr")).Once()
		return credentials.NewCredentials(provider)
	}

	creds, err := s.Retrieve()
	assert.Error(t, err)
	assert.EqualError(t, err, "SomeGetCredsErr")
	assert.Equal(t, emptyCredential, creds)
}

func TestRetrieve_Success_CredsExpireGreaterThanRefreshBeforeExpiry(t *testing.T) {
	config := runtimeconfig.IdentityRuntimeConfig{
		ShareFile: "SomeShareFile",
	}
	config.CredentialsExpiresAt = time.Now().Add(time.Hour)
	newRuntimeConfig = func() runtimeconfig.IIdentityRuntimeConfigClient {
		runtimeConfigClient := &runtimeMock.IIdentityRuntimeConfigClient{}
		runtimeConfigClient.On("GetConfig").Return(config, nil).Once()
		return runtimeConfigClient
	}

	var s = sharedCredentialsProvider{
		getTimeNow: func() time.Time {
			return time.Now()
		},
	}

	newSharedCredentials = func(string, string) *credentials.Credentials {
		provider := &mocks.Provider{}
		provider.On("Retrieve").Return(credentials.Value{SecretAccessKey: "SomeAccessKey"}, nil).Once()
		return credentials.NewCredentials(provider)
	}

	creds, err := s.Retrieve()
	assert.NoError(t, err)
	assert.NotEqual(t, emptyCredential, creds)

	assert.True(t, config.CredentialsExpiresAt.After(s.ExpiresAt()))
}

func TestRetrieve_Success_CredsExpireLessThanRefreshBeforeExpiry(t *testing.T) {
	config := runtimeconfig.IdentityRuntimeConfig{
		ShareFile: "SomeShareFile",
	}
	config.CredentialsExpiresAt = time.Now().Add(time.Second)

	newRuntimeConfig = func() runtimeconfig.IIdentityRuntimeConfigClient {
		runtimeConfigClient := &runtimeMock.IIdentityRuntimeConfigClient{}
		runtimeConfigClient.On("GetConfig").Return(config, nil).Once()
		return runtimeConfigClient
	}

	var s = sharedCredentialsProvider{
		getTimeNow: func() time.Time {
			return time.Now()
		},
	}

	newSharedCredentials = func(string, string) *credentials.Credentials {
		provider := &mocks.Provider{}
		provider.On("Retrieve").Return(credentials.Value{SecretAccessKey: "SomeAccessKey"}, nil).Once()
		return credentials.NewCredentials(provider)
	}

	creds, err := s.Retrieve()
	assert.NoError(t, err)
	assert.NotEqual(t, emptyCredential, creds)
	assert.True(t, config.CredentialsExpiresAt.Equal(s.ExpiresAt()))
}
