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
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"

	"github.com/aws/amazon-ssm-agent/agent/log"
)

const nonceSize = 12

type IBlockCipher interface {
	UpdateEncryptionKey(log log.T, cipherTextKey []byte, sessionId string, instanceId string) error
	EncryptWithAESGCM(plainText []byte) (cipherText []byte, err error)
	DecryptWithAESGCM(cipherText []byte) (plainText []byte, err error)
	GetCipherTextKey() (cipherTextKey []byte)
	GetKMSKeyId() (kmsKey string)
}

type BlockCipher struct {
	kmsKeyId         string
	kmsService       IKMSService
	cipherTextKey    []byte
	encryptionKey    []byte
	decryptionKey    []byte
	encryptionCipher cipher.AEAD
	decryptionCipher cipher.AEAD
}

// NewBlockCipher creates a new block cipher
func NewBlockCipher(log log.T, kmsKeyId string) (blockCipher *BlockCipher, err error) {
	var kmsService *KMSService
	if kmsService, err = NewKMSService(log); err != nil {
		return nil, fmt.Errorf("Unable to get new KMSService, %v", err)
	}
	return NewBlockCipherKMS(log, kmsKeyId, kmsService)
}

// NewBlockCipherKMS creates a new block cipher with a provided IKMService instance
func NewBlockCipherKMS(log log.T, kmsKeyId string, kmsService IKMSService) (blockCipher *BlockCipher, err error) {
	// NewBlockCipher creates a new instance of BlockCipher
	blockCipher = &BlockCipher{
		kmsKeyId:   kmsKeyId,
		kmsService: kmsService,
	}
	return blockCipher, nil
}

// UpdateEncryptionKey receives cipherTextBlob and calls kms::Decrypt to receive the encryption data key
func (blockCipher *BlockCipher) UpdateEncryptionKey(log log.T, cipherTextBlob []byte, sessionId string, instanceId string) error {
	// NewBlockCipher creates a new instance of BlockCipher
	var (
		plainTextKey []byte
		err          error
	)
	var encryptionContext = make(map[string]*string)
	const encryptionContextSessionIdKey = "aws:ssm:SessionId"
	encryptionContext[encryptionContextSessionIdKey] = &sessionId
	const encryptionContextTargetIdKey = "aws:ssm:TargetId"
	encryptionContext[encryptionContextTargetIdKey] = &instanceId

	if plainTextKey, err = blockCipher.kmsService.Decrypt(cipherTextBlob, encryptionContext); err != nil {
		return fmt.Errorf("Unable to retrieve data key, %v", err)
	}
	// cryptoKeySizeInBytes is half of PlainTextKey size fetched from KMS. PlainTextKey is split in two two halves of cryptoKeySizeInBytes
	// First half will be used by agent for encryption and second half by clients like cli/console for encryption
	cryptoKeySizeInBytes := KMSKeySizeInBytes / 2
	blockCipher.cipherTextKey = cipherTextBlob
	blockCipher.encryptionKey = plainTextKey[:cryptoKeySizeInBytes]
	blockCipher.decryptionKey = plainTextKey[cryptoKeySizeInBytes:]
	if blockCipher.encryptionCipher, err = getAEAD(blockCipher.encryptionKey); err != nil {
		return err
	}
	if blockCipher.decryptionCipher, err = getAEAD(blockCipher.decryptionKey); err != nil {
		return err
	}
	return nil
}

// getAEAD gets AEAD which is a GCM cipher mode providing authenticated encryption with associated data
func getAEAD(plainTextKey []byte) (aesgcm cipher.AEAD, err error) {
	var block cipher.Block
	if block, err = aes.NewCipher(plainTextKey); err != nil {
		return nil, fmt.Errorf("Error creating NewCipher, %v", err)
	}

	if aesgcm, err = cipher.NewGCM(block); err != nil {
		return nil, fmt.Errorf("Error creating NewGCM, %v", err)
	}

	return aesgcm, nil
}

// EncryptWithGCM encrypts plain text using AES block cipher GCM mode
func (blockCipher *BlockCipher) EncryptWithAESGCM(plainText []byte) (cipherText []byte, err error) {
	var aesgcm = blockCipher.encryptionCipher

	cipherText = make([]byte, nonceSize+len(plainText))
	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("Error when generating nonce for encryption, %v", err)
	}

	// Encrypt plain text using given key and newly generated nonce
	cipherTextWithoutNonce := aesgcm.Seal(nil, nonce, plainText, nil)

	// Append nonce to the beginning of the cipher text to be used while decrypting
	cipherText = append(cipherText[:nonceSize], nonce...)
	cipherText = append(cipherText[nonceSize:], cipherTextWithoutNonce...)
	return cipherText, nil
}

// DecryptWithGCM decrypts cipher text using AES block cipher GCM mode
func (blockCipher *BlockCipher) DecryptWithAESGCM(cipherText []byte) (plainText []byte, err error) {
	var aesgcm = blockCipher.decryptionCipher

	// Pull the nonce out of the cipherText
	nonce := cipherText[:nonceSize]
	cipherTextWithoutNonce := cipherText[nonceSize:]

	// Decrypt just the actual cipherText using nonce extracted above
	if plainText, err = aesgcm.Open(nil, nonce, cipherTextWithoutNonce, nil); err != nil {
		return nil, fmt.Errorf("Error decrypting encrypted text, %v", err)
	}
	return plainText, nil
}

// GetCipherTextKey returns cipherTextKey from BlockCipher
func (blockCipher *BlockCipher) GetCipherTextKey() (cipherTextKey []byte) {
	return blockCipher.cipherTextKey
}

// GetKMSKeyId returns kmsKeyId from BlockCipher
func (blockCipher *BlockCipher) GetKMSKeyId() (kmsKey string) {
	return blockCipher.kmsKeyId
}
