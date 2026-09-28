package sharedprovider

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/mocks"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/mock"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/common/runtimeconfig"
	runtimeMock "github.com/aws/amazon-ssm-agent/common/runtimeconfig/mocks"
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

	creds, err := s.Retrieve(context.TODO())
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

	creds, err := s.Retrieve(context.TODO())
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

	newSharedCredentials = func(_ string, _ string) (aws.CredentialsProvider, error) {
		provider := &mocks.CredentialsProvider{}
		provider.On("Retrieve", mock.Anything).Return(aws.Credentials{}, fmt.Errorf("SomeGetCredsErr")).Once()
		return provider, nil
	}

	creds, err := s.Retrieve(context.TODO())
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

	newSharedCredentials = func(_ string, _ string) (aws.CredentialsProvider, error) {
		provider := &mocks.CredentialsProvider{}
		provider.On("Retrieve", mock.Anything).Return(aws.Credentials{SecretAccessKey: "SomeAccessKey"}, nil).Once()
		return provider, nil
	}

	creds, err := s.Retrieve(context.TODO())
	assert.NoError(t, err)
	assert.NotEqual(t, emptyCredential, creds)

	assert.True(t, config.CredentialsExpiresAt.After(s.ExpiresAt()))
}

func TestRetrieve_Success_CredsExpireLessThanRefreshBeforeExpiry(t *testing.T) {
	os.Setenv("AWS_EC2_METADATA_DISABLED", "false")
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

	newSharedCredentials = func(_ string, _ string) (aws.CredentialsProvider, error) {
		provider := &mocks.CredentialsProvider{}
		provider.On("Retrieve", mock.Anything).Return(aws.Credentials{SecretAccessKey: "SomeAccessKey"}, nil).Once()
		return provider, nil
	}

	creds, err := s.Retrieve(context.TODO())
	assert.NoError(t, err)
	assert.NotEqual(t, emptyCredential, creds)
	assert.True(t, config.CredentialsExpiresAt.Equal(s.ExpiresAt()))
}

// TestNewSharedCredentials_EmptyProfileFallsBackToDefault reproduces the deployer-test
// regression: the write side (agent/managedInstances/sharedCredentials.Store) normalizes
// an empty profile to "default" before writing the [default] section, but the read side
// (LoadSharedConfigProfile) does not. Without matching normalization here, workers with
// runtime-config ShareProfile="" (the EC2RoleProvider default) fail to load credentials
// from the file the core agent just wrote, enter SSM Agent hibernation, and every
// subsequent SendCommand returns Undeliverable.
func TestNewSharedCredentials_EmptyProfileFallsBackToDefault(t *testing.T) {
	// Write a credentials file with only [default] (the shape the core agent writes).
	f, err := os.CreateTemp("", "ssm-agent-creds-*")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString("[default]\naws_access_key_id = AKID\naws_secret_access_key = SAK\naws_session_token = TOKEN\n"); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}

	provider, err := NewSharedCredentials(f.Name(), "")
	if err != nil {
		t.Fatalf("NewSharedCredentials(profile=\"\") returned error: %v", err)
	}

	creds, err := provider.Retrieve(context.TODO())
	if err != nil {
		t.Fatalf("Retrieve returned error: %v", err)
	}
	assert.Equal(t, "AKID", creds.AccessKeyID)
	assert.Equal(t, "SAK", creds.SecretAccessKey)
	assert.Equal(t, "TOKEN", creds.SessionToken)
}
