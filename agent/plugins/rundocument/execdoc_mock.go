// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License..

// Package rundocument implements the aws:runDocument plugin
package rundocument

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/mock"
)

func NewExecMock() ExecMock {
	return ExecMock{}
}

type ExecMock struct {
	mock.Mock
}

func (e ExecMock) ParseDocument(log log.T, documentRaw []byte, orchestrationDir string, s3Bucket string, s3KeyPrefix string, messageID string, documentID string, defaultWorkingDirectory string, params map[string]interface{}) (pluginsInfo []contracts.PluginState, err error) {
	args := e.Called(log, documentRaw, orchestrationDir, s3Bucket, s3KeyPrefix, messageID, documentID, defaultWorkingDirectory, params)
	return args.Get(0).([]contracts.PluginState), args.Error(1)
}

func (e ExecMock) ExecuteDocument(config contracts.Configuration, context context.T, pluginInput []contracts.PluginState, documentID string, documentCreatedDate string) (chan contracts.DocumentResult, error) {
	args := e.Called(context, pluginInput, documentID, documentCreatedDate)
	return args.Get(0).(chan contracts.DocumentResult), args.Error(1)
}
