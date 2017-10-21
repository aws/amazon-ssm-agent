// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package filelock

import (
	"github.com/stretchr/testify/mock"
)

type FileLockerMock struct {
	mock.Mock
}

func (fl *FileLockerMock) Lock(lockPath string, ownerId string, timeoutSeconds int) (locked bool, err error) {
	args := fl.Called(lockPath, ownerId, timeoutSeconds)
	return args.Bool(0), args.Error(1)
}

func (fl *FileLockerMock) Unlock(lockPath string, ownerId string) (hadLock bool, err error) {
	args := fl.Called(lockPath, ownerId)
	return args.Bool(0), args.Error(1)
}

func ExpectLockUnlock(fileLockerMock *FileLockerMock, lockPath string, ownerId string) {
	fileLockerMock.On("Lock", lockPath, ownerId, mock.Anything).Return(true, nil)
	fileLockerMock.On("Unlock", lockPath, ownerId).Return(true, nil)
}

type FileLockerNoop struct{}

func (fl *FileLockerNoop) Lock(lockPath string, ownerId string, timeoutSeconds int) (locked bool, err error) {
	return true, nil
}

func (fl *FileLockerNoop) Unlock(lockPath string, ownerId string) (hadLock bool, err error) {
	return true, nil
}
