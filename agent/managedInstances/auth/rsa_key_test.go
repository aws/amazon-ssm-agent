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

package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSignVerify(t *testing.T) {
	var err error
	var key RsaKey
	var test string = "This is a test string to sign"

	key, err = CreateKeypair()

	signature, err := key.Sign(test)

	err = key.VerifySignature(test, signature)

	assert.NoError(t, err, "Unexpected error")
}

func TestSignVerifyDifferentStrings(t *testing.T) {
	var err error
	var key RsaKey
	var test string = "This is a test string to sign"
	var testVerify string = "This is a different test string to verify"

	key, err = CreateKeypair()

	signature, err := key.Sign(test)
	assert.NoError(t, err, "Unexpected error")
	err = key.VerifySignature(testVerify, signature)

	assert.EqualError(t, err, "crypto/rsa: verification error", "Unexpected error")
}

func TestEncoding(t *testing.T) {
	var err error
	var key, key2 RsaKey
	var test string = "This is a test string to sign"

	key, err = CreateKeypair()

	encodedKey, err := key.EncodePrivateKey()

	key2, err = DecodePrivateKey(encodedKey)

	encodedPublicKey, err := key.EncodePublicKey()
	encodedPublicKey2, err := key2.EncodePublicKey()

	assert.Equal(t, encodedPublicKey, encodedPublicKey2, "Encoded public keys do not match")

	signature, err := key2.Sign(test)

	err = key.VerifySignature(test, signature)

	assert.NoError(t, err, "Unexpected error")
}
