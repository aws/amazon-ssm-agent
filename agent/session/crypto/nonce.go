// Copyright 2024 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
	"crypto/rand"
	"encoding/binary"
	"sync"
)

type NonceGenerator struct {
	state []uint32
	lock  sync.Mutex
}

// Allocates a slice and fills it with a random nonce. Technically this does not need to be random - initializing with
// zeros would still preseve the properties of the cryptosystem. However, this gives us extra protection if
// accidental key reuse were to occur, as we will very likely get a different keystream regardless.
func (nonce *NonceGenerator) InitializeNonce() error {
	nonce.lock.Lock()
	defer nonce.lock.Unlock()
	// Use uint32 to reduce the number of software carries required compared to bytes. Nonce size is not divisible by 8 so avoid uint64.
	nonce.state = make([]uint32, nonceSize/4)
	err := binary.Read(rand.Reader, binary.LittleEndian, nonce.state)
	return err
}

// Increments the state stored in the byte array, handling carry as if it were a little-endian integer. The previous state is returned.
// NIST SP 800-38D suggests an incrementing message counter as an appropriate method for generating these nonces.
// This will give keys a greater lifetime than random generation as it avoids the birthday problem.
// A mutex is used here as a race condition may otherwise cause the same nonce to be used twice, which could compromise security.
func (nonce *NonceGenerator) GenerateNonce() ([]byte, error) {
	nonce.lock.Lock()
	defer nonce.lock.Unlock()

	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, nonce.state)
	if err != nil {
		return nil, err
	}

	for i := 0; i < nonceSize/4; i++ {
		nonce.state[i] += 1
		if nonce.state[i] != 0 {
			break
		}
	}

	return buf.Bytes(), err
}
