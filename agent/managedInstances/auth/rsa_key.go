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

//package auth provides methods to implement managed instances auth support
package auth

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"errors"
)

const (
	keySize  int = 2048
	saltSize int = 32

	// KeyType returns the RSA Key Type
	KeyType = "Rsa"
)

type RsaKey struct {
	privateKey *rsa.PrivateKey
}

//CreateKeypair creates a new RSA keypair
func CreateKeypair() (rsaKey RsaKey, err error) {
	rsaKey.privateKey, err = rsa.GenerateKey(rand.Reader, keySize)

	return
}

//EncodePublicKey encodes a public key to a base 64 DER encoded string
func (rsaKey *RsaKey) EncodePublicKey() (publicKey string, err error) {
	var publicKeyBytes []byte
	publicKeyBytes, err = x509.MarshalPKIXPublicKey(&rsaKey.privateKey.PublicKey)
	if err != nil {
		return
	}
	publicKey = base64.StdEncoding.EncodeToString(publicKeyBytes)

	return
}

//EncodePrivateKey encodes a private key to a base 64 DER encoded string
func (rsaKey *RsaKey) EncodePrivateKey() (privateKey string, err error) {
	var privateKeyBytes []byte
	privateKeyBytes = x509.MarshalPKCS1PrivateKey(rsaKey.privateKey)

	privateKey = base64.StdEncoding.EncodeToString(privateKeyBytes)

	return
}

//DecodePrivateKey decodes a private key from a base 64 DER encoded string
func DecodePrivateKey(privateKey string) (rsaKey RsaKey, err error) {
	var privateKeyBytes []byte
	privateKeyBytes, err = base64.StdEncoding.DecodeString(privateKey)
	if err != nil {
		return
	}
	rsaKey.privateKey, err = x509.ParsePKCS1PrivateKey(privateKeyBytes)
	if err != nil {
		return
	}

	return
}

//Sign creates the signature for a message
func (rsaKey *RsaKey) Sign(message string) (signature string, err error) {
	var signatureBytes []byte
	hashAlgorithm := crypto.SHA256

	messageBytes := bytes.NewBufferString(message)

	//hash
	hasher := hashAlgorithm.New()
	hasher.Write(messageBytes.Bytes())
	messageHash := hasher.Sum(nil)

	//sign
	var pssOptions rsa.PSSOptions
	pssOptions.SaltLength = saltSize
	pssOptions.Hash = hashAlgorithm
	signatureBytes, err = rsa.SignPSS(rand.Reader, rsaKey.privateKey, hashAlgorithm, messageHash, &pssOptions)
	if err != nil {
		return
	}
	signature = base64.StdEncoding.EncodeToString(signatureBytes)

	return
}

//VerifySignature verifies the signature of a message
func (rsaKey *RsaKey) VerifySignature(message string, signature string) (err error) {
	hashAlgorithm := crypto.SHA256
	if rsaKey.privateKey == nil {
		err = errors.New("privateKey is nil")
		return
	}
	messageBytes := bytes.NewBufferString(message)

	//hash
	pssh := hashAlgorithm.New()
	pssh.Write(messageBytes.Bytes())
	messageHash := pssh.Sum(nil)

	var signatureBytes []byte
	signatureBytes, err = base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return
	}

	//Verify signature
	var opts rsa.PSSOptions
	opts.SaltLength = rsa.PSSSaltLengthAuto
	err = rsa.VerifyPSS(&rsaKey.privateKey.PublicKey, hashAlgorithm, messageHash, signatureBytes, &opts)

	return
}
