// Copyright 2023 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

//go:build freebsd || linux || netbsd || openbsd
// +build freebsd linux netbsd openbsd

// Package verificationmanagers is used to verify the agent packages
package verificationmanagers

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/aws/amazon-ssm-agent/agent/log"
)

// armoredSignaturePrefix is the header of an ASCII-armored detached signature
const armoredSignaturePrefix = "-----BEGIN PGP SIGNATURE-----"

var (
	osOpen                 = os.Open
	ioReadFile             = ioutil.ReadFile
	inProcessKeyringSource = buildInProcessKeyring
)

// buildInProcessKeyring builds an OpenPGP keyring from the embedded armored signing keys
func buildInProcessKeyring() (openpgp.EntityList, error) {
	var keyring openpgp.EntityList
	for _, key := range getLinuxPublicKeysIndividual() {
		entities, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(key))
		if err != nil {
			return nil, fmt.Errorf("failed to load signing key: %v", err)
		}
		keyring = append(keyring, entities...)
	}
	if len(keyring) == 0 {
		return nil, fmt.Errorf("no signing keys loaded")
	}
	return keyring, nil
}

// verifySignatureInProcess verifies a detached signature against the embedded keys with no external tool
func (l *linuxManager) verifySignatureInProcess(log log.T, signaturePath string, binaryPath string) error {
	keyring, err := inProcessKeyringSource()
	if err != nil {
		return fmt.Errorf("in-process verify: %v", err)
	}

	sigBytes, err := ioReadFile(signaturePath)
	if err != nil {
		return fmt.Errorf("in-process verify: failed to read signature file %s: %v", signaturePath, err)
	}

	binFile, err := osOpen(binaryPath)
	if err != nil {
		return fmt.Errorf("in-process verify: failed to open binary file %s: %v", binaryPath, err)
	}
	defer binFile.Close()

	if bytes.HasPrefix(bytes.TrimSpace(sigBytes), []byte(armoredSignaturePrefix)) {
		_, err = openpgp.CheckArmoredDetachedSignature(keyring, binFile, bytes.NewReader(sigBytes), nil)
	} else {
		_, err = openpgp.CheckDetachedSignature(keyring, binFile, bytes.NewReader(sigBytes), nil)
	}
	if err != nil {
		return fmt.Errorf("in-process verify: failed to verify signature: %v", err)
	}

	log.Infof("Successfully verified signature in-process")
	return nil
}
