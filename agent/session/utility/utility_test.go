// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// utility package implements all the shared methods between clients.
package utility

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGeneratePassword(t *testing.T) {
	var lastPassword string
	for i := 0; i < 10; i++ {
		// generate new password
		newPassword := testPasswordComposition(t)

		// compare with the last generated password
		assert.False(t, strings.Compare(lastPassword, newPassword) == 0)

		// set new password as old
		lastPassword = newPassword
	}
}

func testPasswordComposition(t *testing.T) string {
	var u = &SessionUtil{}
	password, err := u.GeneratePasswordForDefaultUser()
	assert.Nil(t, err)
	assert.True(t, u.MinPasswordLength < u.MaxPasswordLength)

	// check if the password has atleast one uppercase, lowercase, symbol and digit.
	upperCaseCount := 0
	lowerCaseCount := 0
	symbolCount := 0
	digitCount := 0
	for i := 0; i < len(password); i++ {
		if strings.Contains(upperCaseLetters, string(password[i])) {
			upperCaseCount++
		}
		if strings.Contains(lowerCaseLetters, string(password[i])) {
			lowerCaseCount++
		}
		if strings.Contains(digits, string(password[i])) {
			digitCount++
		}
		if strings.Contains(symbols, string(password[i])) {
			symbolCount++
		}
	}
	totalCount := upperCaseCount + lowerCaseCount + digitCount + symbolCount

	assert.True(t, upperCaseCount > 0)
	assert.True(t, lowerCaseCount > 0)
	assert.True(t, digitCount > 0)
	assert.True(t, symbolCount > 0)
	assert.Equal(t, len(password), totalCount)
	assert.True(t, len(password) >= u.MinPasswordLength)
	assert.True(t, len(password) <= u.MaxPasswordLength)
	return password
}

func TestInvalidMinAndMaxPasswordLength(t *testing.T) {
	var u = &SessionUtil{
		MinPasswordLength: 30,
		MaxPasswordLength: 10,
	}

	_, err := u.GeneratePasswordForDefaultUser()
	assert.NotNil(t, err)
}
