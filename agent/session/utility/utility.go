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
	"crypto/rand"
	"errors"
	"math/big"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
)

const (
	lowerCaseLetters = "abcdefghijklmnopqrstuvwxyz"
	upperCaseLetters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digits           = "0123456789"

	// https://docs.microsoft.com/en-us/previous-versions/windows/it-pro/windows-server-2008-R2-and-2008/hh994562(v=ws.10)
	// https://docs.microsoft.com/en-us/windows/security/threat-protection/security-policy-settings/password-must-meet-complexity-requirements
	symbols = "!#%"

	defaultMinPasswordLength = 30
	defaultMaxPasswordLength = 63
)

type ISessionUtil interface {
	GeneratePasswordForDefaultUser() (string, error)
	ChangePassword(username string, password string) (userExists bool, err error)
	ResetPasswordIfDefaultUserExists(context context.T) (err error)
	AddNewUser(username string, password string) (userExists bool, err error)
	AddUserToLocalAdministratorsGroup(username string) (adminGroupName string, err error)
	IsInstanceADomainController(log log.T) (isDCServiceRunning bool)
	CreateLocalAdminUser(log log.T) (string, error)
	EnableLocalUser(log log.T) error
	DisableLocalUser(log log.T) error
}

type SessionUtil struct {
	MinPasswordLength int
	MaxPasswordLength int
}

// GeneratePasswordForDefaultUser generates a random password using go lang crypto rand package.
// Public docs: https://golang.org/pkg/crypto/rand/
// On Windows systems, it uses the CryptGenRandom API.
func (u *SessionUtil) GeneratePasswordForDefaultUser() (string, error) {
	// Set password min and max length to default if not provided.
	if u.MinPasswordLength <= 0 || u.MaxPasswordLength <= 0 {
		u.MinPasswordLength = defaultMinPasswordLength
		u.MaxPasswordLength = defaultMaxPasswordLength
	}

	// Enforce max length to be greater than min length of the password.
	if u.MaxPasswordLength <= u.MinPasswordLength {
		return "", errors.New("Max password length should be greater than Min password length")
	}

	// Generate a set of characters that comply with the Microsoft password policy.
	// References:
	// https://docs.microsoft.com/en-us/previous-versions/windows/it-pro/windows-server-2008-R2-and-2008/hh994562(v=ws.10)
	// https://docs.microsoft.com/en-us/windows/security/threat-protection/security-policy-settings/password-must-meet-complexity-requirements
	letters := lowerCaseLetters
	letters += upperCaseLetters
	letters += digits
	letters += symbols

	// Select a random length between min and max
	length, err := randomLength(u.MinPasswordLength, u.MaxPasswordLength)
	if err != nil {
		return "", err
	}

	var result string

	// Enforce adding atleast one of all the 4 categories.
	// atleast one uppercase letter
	ch, err := randomElement(upperCaseLetters)
	if err != nil {
		return "", err
	}
	result = result + ch

	// atleast one lowercase letter
	ch, err = randomElement(lowerCaseLetters)
	if err != nil {
		return "", err
	}
	result = result + ch

	// atleast one digit
	ch, err = randomElement(digits)
	if err != nil {
		return "", err
	}
	result = result + ch

	// atleast one symbol
	ch, err = randomElement(symbols)
	if err != nil {
		return "", err
	}
	result = result + ch

	// mix up the rest
	// We call randomInsert as a part of this for loop which will also randomize the first 4 added characters.
	for i := len(result); i < length; i++ {
		// Randomly select an element from the character set.
		ch, err := randomElement(letters)
		if err != nil {
			return "", err
		}

		// Now, insert this new selected element at a random position in the final string.
		result, err = randomInsert(result, ch)
		if err != nil {
			return "", err
		}
	}

	return result, nil
}

// randomLength selects a random number between min and max.
func randomLength(minLength, maxLength int) (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(maxLength-minLength)))
	if err != nil {
		return 0, err
	}
	return int(n.Int64() + int64(minLength)), nil
}

// randomInsert randomly inserts the given value into the given string.
func randomInsert(s, val string) (string, error) {
	if s == "" {
		return val, nil
	}

	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(s))))
	if err != nil {
		return "", err
	}
	i := n.Int64()
	return s[0:i] + val + s[i:], nil
}

// randomElement extracts a random element from the given string.
func randomElement(s string) (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(s))))
	if err != nil {
		return "", err
	}
	return string(s[n.Int64()]), nil
}
