// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.

//go:build darwin

package utility

import (
	"github.com/stretchr/testify/assert"
	"os/user"
	"testing"
)

func Test_IsRunningElevatedPermissions_Success(t *testing.T) {
	userCurrent = func() (*user.User, error) {
		return &user.User{Username: ExpectedServiceRunningUser}, nil
	}
	err := IsRunningElevatedPermissions()
	assert.Nil(t, err)
}

func Test_IsRunningElevatedPermissions_Failure(t *testing.T) {
	userCurrent = func() (*user.User, error) {
		return &user.User{Username: "DummyUser"}, nil
	}
	err := IsRunningElevatedPermissions()
	assert.NotNil(t, err)
}
