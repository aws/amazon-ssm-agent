package runtimeconfig

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/common/runtimeconfig/runtimeconfighandler"
	"github.com/aws/amazon-ssm-agent/common/runtimeconfig/runtimeconfighandler/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func Test_identityRuntimeConfigClient_ConfigExists(t *testing.T) {
	handlerMock := &mocks.IRuntimeConfigHandler{}
	handlerMock.On("ConfigExists").Return(true, nil)

	i := &identityRuntimeConfigClient{
		configHandler: handlerMock,
	}

	exists, err := i.ConfigExists()
	assert.Nil(t, err)
	assert.True(t, exists)
}

func Test_identityRuntimeConfigClient_GetConfig(t *testing.T) {
	var emptyConfig IdentityRuntimeConfig
	parsedConfig := IdentityRuntimeConfig{
		"InstanceId",
		"IdentityType",
		"ShareFile",
		"ShareProfile",
		time.Time{},
		time.Time{},
	}
	handlerErrorMock := &mocks.IRuntimeConfigHandler{}
	handlerErrorMock.On("GetConfig").Return(nil, fmt.Errorf("SomeError"))

	handlerSuccessButBadFormatMock := &mocks.IRuntimeConfigHandler{}
	handlerSuccessButBadFormatMock.On("GetConfig").Return([]byte("SomeBadBytes"), nil)

	content, _ := json.Marshal(parsedConfig)
	handlerSuccessMock := &mocks.IRuntimeConfigHandler{}
	handlerSuccessMock.On("GetConfig").Return(content, nil)

	type fields struct {
		configHandler runtimeconfighandler.IRuntimeConfigHandler
	}
	tests := []struct {
		name    string
		fields  fields
		want    IdentityRuntimeConfig
		wantErr bool
	}{
		{
			"ErrorHandlerGetConfig",
			fields{
				handlerErrorMock,
			},
			emptyConfig,
			true,
		},
		{
			"ErrorUnmashal",
			fields{
				handlerSuccessButBadFormatMock,
			},
			emptyConfig,
			true,
		},
		{
			"Success",
			fields{
				handlerSuccessMock,
			},
			parsedConfig,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &identityRuntimeConfigClient{
				configHandler: tt.fields.configHandler,
			}
			got, err := i.GetConfig()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetConfig() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_identityRuntimeConfigClient_SaveConfig(t *testing.T) {
	successConfig := IdentityRuntimeConfig{
		"InstanceId",
		"IdentityType",
		"ShareFile",
		"ShareProfile",
		time.Now(),
		time.Now(),
	}
	successContent, _ := json.Marshal(successConfig)
	failContent, _ := json.Marshal(IdentityRuntimeConfig{})

	handlerMock := &mocks.IRuntimeConfigHandler{}
	handlerMock.On("SaveConfig", successContent).Return(nil)
	handlerMock.On("GetConfig").Return(successContent, nil)
	handlerMock.On("SaveConfig", failContent).Return(fmt.Errorf("SomeError"))

	type fields struct {
		configHandler runtimeconfighandler.IRuntimeConfigHandler
	}
	type args struct {
		config IdentityRuntimeConfig
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			"FailedSaveConfig",
			fields{
				handlerMock,
			},
			args{
				IdentityRuntimeConfig{},
			},
			true,
		},
		{
			"Success",
			fields{
				handlerMock,
			},
			args{
				successConfig,
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &identityRuntimeConfigClient{
				configHandler: tt.fields.configHandler,
			}
			if err := i.SaveConfig(tt.args.config); (err != nil) != tt.wantErr {
				t.Errorf("SaveConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_identityRuntimeConfigClient_SaveConfig_VerifyFailGetConfig(t *testing.T) {
	config := IdentityRuntimeConfig{
		"InstanceId",
		"IdentityType",
		"ShareFile",
		"ShareProfile",
		time.Now(),
		time.Now(),
	}
	byteConfig, _ := json.Marshal(config)

	handlerMock := &mocks.IRuntimeConfigHandler{}
	handlerMock.On("SaveConfig", byteConfig).Return(nil)
	handlerMock.On("GetConfig").Return(nil, fmt.Errorf("SomeErrorFailedGetConfig"))

	i := &identityRuntimeConfigClient{
		configHandler: handlerMock,
	}

	err := i.SaveConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to validate config is readable after writing")
	handlerMock.AssertExpectations(t)
}

func Test_identityRuntimeConfigClient_SaveConfig_VerifyFailConfigEquals(t *testing.T) {
	correctConfig := IdentityRuntimeConfig{
		"InstanceId",
		"IdentityType",
		"ShareFile",
		"ShareProfile",
		time.Now(),
		time.Now(),
	}

	wrongConfig := IdentityRuntimeConfig{
		"InstanceId",
		"SomeOtherIdentityType",
		"ShareFile",
		"ShareProfile",
		time.Now(),
		time.Now(),
	}
	wrongByteConfig, _ := json.Marshal(wrongConfig)

	handlerMock := &mocks.IRuntimeConfigHandler{}
	handlerMock.On("SaveConfig", mock.Anything).Return(nil)
	handlerMock.On("GetConfig").Return(wrongByteConfig, nil)

	i := &identityRuntimeConfigClient{
		configHandler: handlerMock,
	}

	err := i.SaveConfig(correctConfig)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to verify config on disk is equivalent to the config that was saved")

	handlerMock.AssertExpectations(t)
}

func TestIdentityRuntimeConfig_Equal(t *testing.T) {
	type fields struct {
		InstanceId             string
		IdentityType           string
		ShareFile              string
		ShareProfile           string
		CredentialsExpiresAt   time.Time
		CredentialsRetrievedAt time.Time
	}
	type args struct {
		config IdentityRuntimeConfig
	}

	baselineArg := args{
		IdentityRuntimeConfig{
			"InstanceId",
			"IdentityType",
			"ShareFile",
			"ShareProfile",
			time.Now(),
			time.Now(),
		},
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			"Success",
			fields{
				"InstanceId",
				"IdentityType",
				"ShareFile",
				"ShareProfile",
				time.Now(),
				time.Now(),
			},
			baselineArg,
			true,
		},
		{
			"NotSameInstanceId",
			fields{
				"InstanceId1",
				"IdentityType",
				"ShareFile",
				"ShareProfile",
				time.Now(),
				time.Now(),
			},
			baselineArg,
			false,
		},
		{
			"NotSameIdentityType",
			fields{
				"InstanceId",
				"IdentityType1",
				"ShareFile",
				"ShareProfile",
				time.Now(),
				time.Now(),
			},
			baselineArg,
			false,
		},
		{
			"NotSameShareFile",
			fields{
				"InstanceId",
				"IdentityType",
				"ShareFile1",
				"ShareProfile",
				time.Now(),
				time.Now(),
			},
			baselineArg,
			false,
		},

		{
			"NotSameShareProfile",
			fields{
				"InstanceId",
				"IdentityType",
				"ShareFile",
				"ShareProfile1",
				time.Now(),
				time.Now(),
			},
			baselineArg,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := IdentityRuntimeConfig{
				InstanceId:             tt.fields.InstanceId,
				IdentityType:           tt.fields.IdentityType,
				ShareFile:              tt.fields.ShareFile,
				ShareProfile:           tt.fields.ShareProfile,
				CredentialsExpiresAt:   tt.fields.CredentialsExpiresAt,
				CredentialsRetrievedAt: tt.fields.CredentialsRetrievedAt,
			}
			if got := i.Equal(tt.args.config); got != tt.want {
				t.Errorf("Equal() = %v, want %v", got, tt.want)
			}
		})
	}
}
