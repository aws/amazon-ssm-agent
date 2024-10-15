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
	"encoding/binary"
	"sync"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type NonceTestSuite struct {
	suite.Suite
	mockLog log.T
}

func (suite *NonceTestSuite) SetupTest() {
	suite.mockLog = logmocks.NewMockLog()
}

// Execute the test suite
func TestNonceTestSuite(t *testing.T) {
	suite.Run(t, new(NonceTestSuite))
}

// Test initialization of nonce
func (suite *NonceTestSuite) TestInitializeNonce() {

	var nonce NonceGenerator
	err := nonce.InitializeNonce()

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), nonce)

	// nonce should be 12 bytes = 3 uint32s
	assert.Equal(suite.T(), 3, len(nonce.state))

	// check nonce is not all zero (highly unlikely)
	assert.Condition(suite.T(), func() bool {
		for _, v := range nonce.state {
			if v > 0 {
				return true
			}
		}
		return false
	})
}

// Test incrementing nonce
func (suite *NonceTestSuite) TestUpdateNonce() {
	// initialize nonce generator with known state
	initialState := []uint32{0x00000001, 0x00000002, 0x00000003}
	nonce := NonceGenerator{initialState, sync.Mutex{}}

	// record state as bytes for later comparison
	initialStateBytes := new(bytes.Buffer)
	binary.Write(initialStateBytes, binary.LittleEndian, initialState)

	// generate a nonce
	returnedNonce, err := nonce.GenerateNonce()
	assert.Nil(suite.T(), err)

	// returnedNonce should be the initial state
	assert.Equal(suite.T(), nonceSize, len(returnedNonce))
	assert.Equal(suite.T(), initialStateBytes.Bytes(), returnedNonce)

	// new nonce should have been incremented
	assert.Equal(suite.T(), uint32(0x00000002), nonce.state[0])
	assert.Equal(suite.T(), uint32(0x00000002), nonce.state[1])
	assert.Equal(suite.T(), uint32(0x00000003), nonce.state[2])
}

// Test incrementing nonce with carry
func (suite *NonceTestSuite) TestUpdateNonceCarry() {
	initialState := []uint32{0xFFFFFFFF, 0x00000002, 0x00000003}
	nonce := NonceGenerator{initialState, sync.Mutex{}}
	_, err := nonce.GenerateNonce()
	assert.Nil(suite.T(), err)

	assert.Equal(suite.T(), uint32(0x00000000), nonce.state[0])
	assert.Equal(suite.T(), uint32(0x00000003), nonce.state[1])
	assert.Equal(suite.T(), uint32(0x00000003), nonce.state[2])
}

// Test incrementing nonce with rollover
func (suite *NonceTestSuite) TestUpdateNonceRollover() {
	initialState := []uint32{0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF}
	nonce := NonceGenerator{initialState, sync.Mutex{}}
	_, err := nonce.GenerateNonce()
	assert.Nil(suite.T(), err)

	assert.Equal(suite.T(), uint32(0x00000000), nonce.state[0])
	assert.Equal(suite.T(), uint32(0x00000000), nonce.state[1])
	assert.Equal(suite.T(), uint32(0x00000000), nonce.state[2])
}
