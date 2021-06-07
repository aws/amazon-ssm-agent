/*
 * Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"). You may not
 * use this file except in compliance with the License. A copy of the
 * License is located at
 *
 * http://aws.amazon.com/apache2.0/
 *
 * or in the "license" file accompanying this file. This file is distributed
 * on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing
 * permissions and limitations under the License.
 */

// Package mock defines the struct and its corresponding methods for mocking handler.IGitHandler
package mock

import (
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/gitresource/privategit/handler/core"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/stretchr/testify/mock"
)

type GitHandlerMock struct {
	mock.Mock
}

func (mock *GitHandlerMock) Validate() (bool, error) {
	args := mock.Called()
	return args.Bool(0), args.Error(1)
}

func (mock *GitHandlerMock) GetAuthMethod(log log.T) (transport.AuthMethod, error) {
	args := mock.Called(log)
	return args.Get(0).(transport.AuthMethod), args.Error(1)

}
func (mock *GitHandlerMock) PerformCheckout(repository core.IGitRepository) error {
	args := mock.Called(repository)
	return args.Error(0)

}
func (mock *GitHandlerMock) CloneRepository(log log.T, authMethod transport.AuthMethod, destPath string) (repository *gogit.Repository, err error) {
	args := mock.Called(log, authMethod, destPath)
	return args.Get(0).(*gogit.Repository), args.Error(1)
}
