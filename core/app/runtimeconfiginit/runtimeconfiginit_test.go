package runtimeconfiginit

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/identity"
	mockIdentity "github.com/aws/amazon-ssm-agent/common/identity/mocks"
	"github.com/aws/amazon-ssm-agent/common/runtimeconfig"
	"github.com/aws/amazon-ssm-agent/common/runtimeconfig/mocks"
	"github.com/cenkalti/backoff/v4"
)

func TestNew(t *testing.T) {
	mockLog := log.NewMockLog()
	mockIdentity := mockIdentity.NewDefaultMockAgentIdentity()
	type args struct {
		log      log.T
		identity identity.IAgentIdentity
	}
	tests := []struct {
		name string
		args args
		want IRuntimeConfigInit
	}{
		{
			"Success",
			args{
				mockLog,
				mockIdentity,
			},
			&runtimeConfigInit{
				mockLog,
				nil,
				mockIdentity,
				runtimeconfig.NewIdentityRuntimeConfigClient()},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := New(tt.args.log, tt.args.identity); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("New() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_runtimeConfigInit_getCurrentIdentityRuntimeConfig(t *testing.T) {
	successIdentity := mockIdentity.NewDefaultMockAgentIdentity()
	successConfig := runtimeconfig.IdentityRuntimeConfig{
		InstanceId:   mockIdentity.MockInstanceID,
		IdentityType: mockIdentity.MockIdentityType,
	}

	failureIdentity := &mockIdentity.IAgentIdentity{}
	failureIdentity.On("IdentityType").Return("SomeIdentityType")
	failureIdentity.On("InstanceID").Return("", fmt.Errorf("SomeError"))
	failureConfig := runtimeconfig.IdentityRuntimeConfig{
		IdentityType: "SomeIdentityType",
	}

	type fields struct {
		log                  log.T
		backoffConfig        *backoff.ExponentialBackOff
		agentIdentity        identity.IAgentIdentity
		identityConfigClient runtimeconfig.IIdentityRuntimeConfigClient
	}
	tests := []struct {
		name    string
		fields  fields
		want    runtimeconfig.IdentityRuntimeConfig
		wantErr bool
	}{
		{
			"Success",
			fields{
				nil,
				nil,
				successIdentity,
				nil,
			},
			successConfig,
			false,
		},
		{
			"Failure",
			fields{
				nil,
				nil,
				failureIdentity,
				nil,
			},
			failureConfig,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &runtimeConfigInit{
				log:                  tt.fields.log,
				backoffConfig:        tt.fields.backoffConfig,
				agentIdentity:        tt.fields.agentIdentity,
				identityConfigClient: tt.fields.identityConfigClient,
			}
			got, err := r.getCurrentIdentityRuntimeConfig()
			if (err != nil) != tt.wantErr {
				t.Errorf("getCurrentIdentityRuntimeConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getCurrentIdentityRuntimeConfig() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_runtimeConfigInit_initIdentityRuntimeConfig(t *testing.T) {
	var emptyConfig runtimeconfig.IdentityRuntimeConfig
	successIdentity := mockIdentity.NewDefaultMockAgentIdentity()
	currentConfig := runtimeconfig.IdentityRuntimeConfig{
		InstanceId:   mockIdentity.MockInstanceID,
		IdentityType: mockIdentity.MockIdentityType,
	}
	otherConfig := runtimeconfig.IdentityRuntimeConfig{
		InstanceId:   "OtherInstanceID",
		IdentityType: "OtherIdentityType",
	}

	failureIdentity := &mockIdentity.IAgentIdentity{}
	failureIdentity.On("IdentityType").Return("SomeIdentityType")
	failureIdentity.On("InstanceID").Return("", fmt.Errorf("SomeError"))

	icc_equalConfig := &mocks.IIdentityRuntimeConfigClient{}
	icc_equalConfig.On("ConfigExists").Return(true, nil)
	icc_equalConfig.On("GetConfig").Return(currentConfig, nil)

	icc_notEqualConfig := &mocks.IIdentityRuntimeConfigClient{}
	icc_notEqualConfig.On("SaveConfig", currentConfig).Return(nil)
	icc_notEqualConfig.On("ConfigExists").Return(true, nil)
	icc_notEqualConfig.On("GetConfig").Return(otherConfig, nil)

	icc_configNotExist := &mocks.IIdentityRuntimeConfigClient{}
	icc_configNotExist.On("SaveConfig", currentConfig).Return(nil)
	icc_configNotExist.On("ConfigExists").Return(false, nil)

	icc_configExistErr := &mocks.IIdentityRuntimeConfigClient{}
	icc_configExistErr.On("SaveConfig", currentConfig).Return(nil)
	icc_configExistErr.On("ConfigExists").Return(false, fmt.Errorf("SomeError"))

	icc_errGetConfig := &mocks.IIdentityRuntimeConfigClient{}
	icc_errGetConfig.On("SaveConfig", currentConfig).Return(nil)
	icc_errGetConfig.On("ConfigExists").Return(true, nil)
	icc_errGetConfig.On("GetConfig").Return(emptyConfig, fmt.Errorf("SomeError"))

	icc_saveConfigErr := &mocks.IIdentityRuntimeConfigClient{}
	icc_saveConfigErr.On("SaveConfig", currentConfig).Return(fmt.Errorf("SomeError"))
	icc_saveConfigErr.On("ConfigExists").Return(true, nil)
	icc_saveConfigErr.On("GetConfig").Return(otherConfig, nil)

	backoffRetry = func(fun backoff.Operation, _ backoff.BackOff) error {
		return fun()
	}

	type fields struct {
		log                  log.T
		backoffConfig        *backoff.ExponentialBackOff
		agentIdentity        identity.IAgentIdentity
		identityConfigClient runtimeconfig.IIdentityRuntimeConfigClient
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			"FailedGetCurrentRuntimeConfig",
			fields{
				nil,
				nil,
				failureIdentity,
				nil,
			},
			true,
		},
		{
			"Success_ConfigNotEqual",
			fields{
				log.NewMockLog(),
				nil,
				successIdentity,
				icc_notEqualConfig,
			},
			false,
		},

		{
			"Success_ConfigEqual",
			fields{
				log.NewMockLog(),
				nil,
				successIdentity,
				icc_equalConfig,
			},
			false,
		},
		{
			"Success_ConfigNotExistEqual",
			fields{
				log.NewMockLog(),
				nil,
				successIdentity,
				icc_configNotExist,
			},
			false,
		},
		{
			"Success_ConfigExistErr",
			fields{
				log.NewMockLog(),
				nil,
				successIdentity,
				icc_configExistErr,
			},
			false,
		},
		{
			"Success_GetConfigErr",
			fields{
				log.NewMockLog(),
				nil,
				successIdentity,
				icc_errGetConfig,
			},
			false,
		},
		{
			"Failed_SaveConfigErr",
			fields{
				log.NewMockLog(),
				nil,
				successIdentity,
				icc_saveConfigErr,
			},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &runtimeConfigInit{
				log:                  tt.fields.log,
				backoffConfig:        tt.fields.backoffConfig,
				agentIdentity:        tt.fields.agentIdentity,
				identityConfigClient: tt.fields.identityConfigClient,
			}
			if err := r.initIdentityRuntimeConfig(); (err != nil) != tt.wantErr {
				t.Errorf("initIdentityRuntimeConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_runtimeConfigInit_saveIdentityConfigWithRetry(t *testing.T) {
	icc := &mocks.IIdentityRuntimeConfigClient{}

	successConfig := runtimeconfig.IdentityRuntimeConfig{
		InstanceId:   "InstanceId",
		IdentityType: "IdentityType",
	}
	failureConfig := runtimeconfig.IdentityRuntimeConfig{}

	icc.On("SaveConfig", failureConfig).Return(fmt.Errorf("SomeError"))
	icc.On("SaveConfig", successConfig).Return(nil)

	backoffRetry = func(fun backoff.Operation, _ backoff.BackOff) error {
		return fun()
	}

	type fields struct {
		log                  log.T
		backoffConfig        *backoff.ExponentialBackOff
		agentIdentity        identity.IAgentIdentity
		identityConfigClient runtimeconfig.IIdentityRuntimeConfigClient
	}
	type args struct {
		currentConfig runtimeconfig.IdentityRuntimeConfig
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			"Failed",
			fields{
				log.NewMockLog(),
				nil,
				nil,
				icc,
			},
			args{
				failureConfig,
			},
			true,
		},
		{
			"Success",
			fields{
				log.NewMockLog(),
				nil,
				nil,
				icc,
			},
			args{
				successConfig,
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &runtimeConfigInit{
				log:                  tt.fields.log,
				backoffConfig:        tt.fields.backoffConfig,
				agentIdentity:        tt.fields.agentIdentity,
				identityConfigClient: tt.fields.identityConfigClient,
			}
			if err := r.saveIdentityConfigWithRetry(tt.args.currentConfig); (err != nil) != tt.wantErr {
				t.Errorf("saveIdentityConfigWithRetry() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
