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

// crypto package provides methods to encrypt and decrypt data
package crypto

import (
	"encoding/hex"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/crypto/mocks"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type BlockCipherTestSuite struct {
	suite.Suite
	mockLog        log.T
	mockKMSService mocks.IKMSService
	kmsKeyId       string
	plainTextData  []byte
	cipherTextKey  []byte
	plainTextKey   []byte
	sessionId      string
}

func (suite *BlockCipherTestSuite) SetupTest() {
	suite.mockLog = log.NewMockLog()
	suite.mockKMSService = mocks.IKMSService{}

	suite.kmsKeyId = "kmsKeyId"
	suite.plainTextData = []byte("plainTextDataToBeEncrypted")
	suite.cipherTextKey = []byte("cipherTextKey")
	suite.plainTextKey, _ = hex.DecodeString("7775626261206c756262612064756220647562207775626261206c756262612064756220647562207775626261206c7562626120647562206475622077756262")
	suite.sessionId = "some-session-id"
}

//Execute the test suite
func TestBlockCipherTestSuite(t *testing.T) {
	suite.Run(t, new(BlockCipherTestSuite))
}

// Testing Encrypt and Decrypt functions
func (suite *BlockCipherTestSuite) TestEncryptDecrypt() {
	var encryptionContext = map[string]*string{"SessionId": &suite.sessionId}
	suite.mockKMSService.On("Decrypt", suite.cipherTextKey, encryptionContext).Return(suite.plainTextKey, nil)

	blockCipher, err := NewBlockCipherKMS(suite.mockLog, suite.kmsKeyId, &suite.mockKMSService)
	blockCipher.UpdateEncryptionKey(suite.mockLog, suite.cipherTextKey, suite.sessionId)
	assert.Nil(suite.T(), err)
	// Create another cipher with flipped encryption/decryption keys
	blockCipherReversed := BlockCipher(BlockCipher{})
	blockCipherReversed.decryptionKey = blockCipher.encryptionKey
	blockCipherReversed.encryptionKey = blockCipher.decryptionKey

	encryptedData, err := blockCipher.EncryptWithAESGCM(suite.plainTextData)
	assert.Nil(suite.T(), err)

	decryptedData, err := blockCipherReversed.DecryptWithAESGCM(encryptedData)
	assert.Nil(suite.T(), err)

	assert.Equal(suite.T(), suite.plainTextData, decryptedData)
}

func (suite *BlockCipherTestSuite) TestGetCipherTextKey() {
	var blockCipher IBlockCipher = &BlockCipher{cipherTextKey: suite.cipherTextKey}
	assert.Equal(suite.T(), suite.cipherTextKey, blockCipher.GetCipherTextKey())
}

func (suite *BlockCipherTestSuite) TestGetKMSKeyId() {
	var blockCipher IBlockCipher = &BlockCipher{kmsKeyId: suite.kmsKeyId}
	assert.Equal(suite.T(), suite.kmsKeyId, blockCipher.GetKMSKeyId())
}
