package runtimeconfighandler

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/core/workerprovider/longrunningprovider/datastore/filesystem"
	fileSystemMock "github.com/aws/amazon-ssm-agent/core/workerprovider/longrunningprovider/datastore/filesystem/mocks"
	"github.com/stretchr/testify/mock"
)

func Test_runtimeConfigHandler_ConfigExists(t *testing.T) {
	configExistsPath := "configExistExample.json"
	configNotExistPath := "configNotExistExample.json"
	configSomeErrorPath := "someOtherError.json"

	fsMock := &fileSystemMock.FileSystem{}
	fileNotExistError := fmt.Errorf("FileNotExistError")
	someOtherError := fmt.Errorf("SomeRandomError")

	fsMock.On("Stat", filepath.Join(appconfig.RuntimeConfigFolderPath, configExistsPath)).Return(nil, nil)
	fsMock.On("Stat", filepath.Join(appconfig.RuntimeConfigFolderPath, configNotExistPath)).Return(nil, fileNotExistError)
	fsMock.On("Stat", filepath.Join(appconfig.RuntimeConfigFolderPath, configSomeErrorPath)).Return(nil, someOtherError)

	fsMock.On("IsNotExist", fileNotExistError).Return(true)
	fsMock.On("IsNotExist", nil).Return(false)
	fsMock.On("IsNotExist", someOtherError).Return(false)

	type fields struct {
		configName string
		fileSystem filesystem.IFileSystem
	}
	tests := []struct {
		name    string
		fields  fields
		want    bool
		wantErr bool
	}{
		{
			"FileNotExist",
			fields{
				configNotExistPath,
				fsMock,
			},
			false,
			false,
		},
		{
			"UnexpectedError",
			fields{
				configSomeErrorPath,
				fsMock,
			},
			false,
			true,
		},
		{
			"DoesExist",
			fields{
				configExistsPath,
				fsMock,
			},
			true,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &runtimeConfigHandler{
				configName: tt.fields.configName,
				fileSystem: tt.fields.fileSystem,
			}
			got, err := r.ConfigExists()
			if (err != nil) != tt.wantErr {
				t.Errorf("ConfigExists() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ConfigExists() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_runtimeConfigHandler_GetConfig(t *testing.T) {
	configExistsPath := "configExistExample.json"
	configSomeErrorPath := "someOtherError.json"

	fsMock := &fileSystemMock.FileSystem{}
	someError := fmt.Errorf("SomeRandomError")

	successReadBytes := []byte("SomeString")
	fsMock.On("ReadFile", filepath.Join(appconfig.RuntimeConfigFolderPath, configExistsPath)).Return(successReadBytes, nil)
	fsMock.On("ReadFile", filepath.Join(appconfig.RuntimeConfigFolderPath, configSomeErrorPath)).Return(nil, someError)

	type fields struct {
		configName string
		fileSystem filesystem.IFileSystem
	}
	tests := []struct {
		name    string
		fields  fields
		want    []byte
		wantErr bool
	}{
		{
			"FailedRead",
			fields{
				configSomeErrorPath,
				fsMock,
			},
			nil,
			true,
		},
		{
			"SuccessRead",
			fields{
				configExistsPath,
				fsMock,
			},
			successReadBytes,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &runtimeConfigHandler{
				configName: tt.fields.configName,
				fileSystem: tt.fields.fileSystem,
			}
			got, err := r.GetConfig()
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

func Test_runtimeConfigHandler_SaveConfig(t *testing.T) {
	configExistsPath := "configExistExample.json"
	configSomeErrorPath := "someOtherError.json"

	fsMock := &fileSystemMock.FileSystem{}
	someError := fmt.Errorf("SomeRandomError")

	fsMock.On("WriteFile", filepath.Join(appconfig.RuntimeConfigFolderPath, configExistsPath), mock.Anything, mock.Anything).Return(nil)
	fsMock.On("WriteFile", filepath.Join(appconfig.RuntimeConfigFolderPath, configSomeErrorPath), mock.Anything, mock.Anything).Return(someError)
	fsMock.On("MkdirAll", mock.Anything, mock.Anything).Return(nil)

	fsMockFailMkdir := &fileSystemMock.FileSystem{}
	fsMockFailMkdir.On("MkdirAll", mock.Anything, mock.Anything).Return(someError)

	type fields struct {
		configName string
		fileSystem filesystem.IFileSystem
	}
	type args struct {
		content []byte
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			"FailedMkdir",
			fields{
				"",
				fsMockFailMkdir,
			},
			args{
				[]byte{},
			},
			true,
		},
		{
			"FailedWriteFile",
			fields{
				configSomeErrorPath,
				fsMock,
			},
			args{
				[]byte{},
			},
			true,
		},
		{
			"SuccessWriteFile",
			fields{
				configExistsPath,
				fsMock,
			},
			args{
				[]byte{},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &runtimeConfigHandler{
				configName: tt.fields.configName,
				fileSystem: tt.fields.fileSystem,
			}
			if err := r.SaveConfig(tt.args.content); (err != nil) != tt.wantErr {
				t.Errorf("SaveConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
