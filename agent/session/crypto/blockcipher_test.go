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
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/agent/session/crypto/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type BlockCipherTestSuite struct {
	suite.Suite
	mockLog              log.T
	mockKMSService       mocks.IKMSService
	kmsKeyId             string
	plainTextData        []byte
	cipherTextKey        []byte
	plainTextKey         []byte
	cipherTextKeyFlipped []byte
	plainTextKeyFlipped  []byte
	sessionId            string
	instanceId           string
}

func (suite *BlockCipherTestSuite) SetupTest() {
	suite.mockLog = logmocks.NewMockLog()
	suite.mockKMSService = mocks.IKMSService{}

	suite.kmsKeyId = "kmsKeyId"
	suite.plainTextData = []byte("plainTextDataToBeEncrypted")
	suite.cipherTextKey = []byte("cipherTextKey")
	suite.plainTextKey, _ = hex.DecodeString("7775626261206c756262612064756220647562207775626261206c756262612064756220647562207775626261206c7562626120647562206475622077756262")
	suite.cipherTextKeyFlipped = []byte("cipherTextKeyFlipped")
	suite.plainTextKeyFlipped, _ = hex.DecodeString("64756220647562207775626261206c75626261206475622064756220777562627775626261206c756262612064756220647562207775626261206c7562626120")
	suite.sessionId = "some-session-id"
	suite.instanceId = "some-instance-id"
}

// Execute the test suite
func TestBlockCipherTestSuite(t *testing.T) {
	suite.Run(t, new(BlockCipherTestSuite))
}

// Testing Encrypt and Decrypt functions
func (suite *BlockCipherTestSuite) TestEncryptDecrypt() {
	blockCipher, err := NewBlockCipherKMS(suite.kmsKeyId, &suite.mockKMSService)
	assert.Nil(suite.T(), err)

	challenge := blockCipher.GetRandomChallenge()

	var encryptionContext = map[string]*string{"aws:ssm:SessionId": &suite.sessionId, "aws:ssm:TargetId": &suite.instanceId, "aws:ssm:RandomChallenge": &challenge}
	suite.mockKMSService.On("Decrypt", suite.cipherTextKey, encryptionContext, suite.kmsKeyId).Return(suite.plainTextKey, nil)

	err = blockCipher.UpdateEncryptionKey(suite.mockLog, suite.cipherTextKey, suite.sessionId, suite.instanceId, true)
	assert.Nil(suite.T(), err)

	// Create another cipher with flipped encryption/decryption keys
	suite.mockKMSService.On("Decrypt", suite.cipherTextKeyFlipped, encryptionContext, suite.kmsKeyId).Return(suite.plainTextKeyFlipped, nil)
	blockCipherReversed := BlockCipher(*blockCipher)
	err = blockCipherReversed.UpdateEncryptionKey(suite.mockLog, suite.cipherTextKeyFlipped, suite.sessionId, suite.instanceId, true)

	encryptedData, err := blockCipher.EncryptWithAESGCM(suite.plainTextData)
	assert.Nil(suite.T(), err)

	decryptedData, err := blockCipherReversed.DecryptWithAESGCM(encryptedData)
	assert.Nil(suite.T(), err)

	assert.Equal(suite.T(), suite.plainTextData, decryptedData)
	suite.mockKMSService.AssertExpectations(suite.T())
}

// Testing cipher initialization without random challenge
func (suite *BlockCipherTestSuite) TestNoRandomChallenge() {
	blockCipher, err := NewBlockCipherKMS(suite.kmsKeyId, &suite.mockKMSService)
	assert.Nil(suite.T(), err)

	var encryptionContext = map[string]*string{"aws:ssm:SessionId": &suite.sessionId, "aws:ssm:TargetId": &suite.instanceId}
	suite.mockKMSService.On("Decrypt", suite.cipherTextKey, encryptionContext, suite.kmsKeyId).Return(suite.plainTextKey, nil)

	err = blockCipher.UpdateEncryptionKey(suite.mockLog, suite.cipherTextKey, suite.sessionId, suite.instanceId, false)
	assert.Nil(suite.T(), err)
}

// Test to ensure the nonce changes between two subsequent messages
func (suite *BlockCipherTestSuite) TestEncryptChangesNonce() {
	blockCipher, err := NewBlockCipherKMS(suite.kmsKeyId, &suite.mockKMSService)
	assert.Nil(suite.T(), err)

	challenge := blockCipher.GetRandomChallenge()

	var encryptionContext = map[string]*string{"aws:ssm:SessionId": &suite.sessionId, "aws:ssm:TargetId": &suite.instanceId, "aws:ssm:RandomChallenge": &challenge}
	suite.mockKMSService.On("Decrypt", suite.cipherTextKey, encryptionContext, suite.kmsKeyId).Return(suite.plainTextKey, nil)

	err = blockCipher.UpdateEncryptionKey(suite.mockLog, suite.cipherTextKey, suite.sessionId, suite.instanceId, true)
	assert.Nil(suite.T(), err)

	// Translate nonce internal state to bytes
	nonce1 := new(bytes.Buffer)
	binary.Write(nonce1, binary.LittleEndian, blockCipher.gcmNonce.state)

	// Encrypt and validate that the expected nonce is there
	envelope1, err := blockCipher.EncryptWithAESGCM(suite.plainTextData)
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), nonce1.Bytes(), envelope1[:nonceSize])

	// Get next nonce state
	nonce2 := new(bytes.Buffer)
	binary.Write(nonce2, binary.LittleEndian, blockCipher.gcmNonce.state)

	// Encrypt again and check that this nonce is now present
	envelope2, err := blockCipher.EncryptWithAESGCM(suite.plainTextData)
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), nonce2.Bytes(), envelope2[:nonceSize])

	// Final sanity check that the two nonces are not equal
	assert.NotEqual(suite.T(), nonce1.Bytes(), nonce2.Bytes())
}

func (suite *BlockCipherTestSuite) TestGetCipherTextKey() {
	var blockCipher IBlockCipher = &BlockCipher{cipherTextKey: suite.cipherTextKey}
	assert.Equal(suite.T(), suite.cipherTextKey, blockCipher.GetCipherTextKey())
}

func (suite *BlockCipherTestSuite) TestGetKMSKeyId() {
	var blockCipher IBlockCipher = &BlockCipher{kmsKeyId: suite.kmsKeyId}
	assert.Equal(suite.T(), suite.kmsKeyId, blockCipher.GetKMSKeyId())
}
func (suite *BlockCipherTestSuite) TestGetRandomChallenge() {
	randomChallenge := "aaaabbbbccccdddd"
	var blockCipher IBlockCipher = &BlockCipher{randomChallenge: randomChallenge}
	assert.Equal(suite.T(), randomChallenge, blockCipher.GetRandomChallenge())
}
