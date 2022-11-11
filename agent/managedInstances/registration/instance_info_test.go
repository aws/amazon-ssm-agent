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

// package registration provides managed instance information
package registration

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
)

const (
	sampleError            = "registration error occurred"
	sampleRegion           = "us-west-1"
	sampleID               = "mi-e6c6f145e6c6f145"
	samplePrivateKey       = "KEYe6c6f145e6c6f145"
	sampleRegistrationCode = "CODEe6c6f145e6c6f145"
	sampleRegistrationKey  = "e4fe609b-fc93-4536-aef3-9a1a5d2647d6"
)

var (
	sampleDest = instanceInfo{InstanceID: sampleID, Region: sampleRegion, PrivateKey: samplePrivateKey}
	sampleJson = []byte(`{"instanceID":"mi-e6c6f145e6c6f145","region":"us-west-1","privateKey":"KEYe6c6f145e6c6f145","registrationCode":"CODEe6c6f145e6c6f145"}`)
)

func ExampleRegion() {
	file = fileStub{}
	vault = vaultStub{rKey: sampleRegistrationKey, data: sampleJson, exists: true}
	loadServerInfo("", RegVaultKey) // load info with mocked vault
	region := Region(log.NewMockLog(), "", RegVaultKey)
	fmt.Println(region)
	// Output:
	// us-west-1
}

func ExampleInstanceID() {
	file = fileStub{}
	vault = vaultStub{rKey: sampleRegistrationKey, data: sampleJson, exists: true}
	loadServerInfo("", RegVaultKey) // load info with mocked vault
	instanceID := InstanceID(log.NewMockLog(), "", RegVaultKey)
	fmt.Println(instanceID)
	// Output:
	// mi-e6c6f145e6c6f145
}

func ExamplePrivateKey() {
	file = fileStub{}
	vault = vaultStub{rKey: sampleRegistrationKey, data: sampleJson, exists: true}
	loadServerInfo("", RegVaultKey) // load info with mocked vault
	privateKey := PrivateKey(log.NewMockLog(), "", RegVaultKey)
	fmt.Println(privateKey)
	// Output:
	// KEYe6c6f145e6c6f145
}

func TestPrivateKeyEC2(t *testing.T) {
	file = fileStub{}
	vault = vaultStub{rKey: sampleRegistrationKey, data: sampleJson, exists: true, manifestFileNamePrefix: "EC2"}
	loadServerInfo("EC2", EC2RegistrationVaultKey) // load info with mocked vault
	privateKey := PrivateKey(log.NewMockLog(), "EC2", EC2RegistrationVaultKey)
	assert.Equal(t, "KEYe6c6f145e6c6f145", privateKey)
}

func TestGeneratePublicKey(t *testing.T) {
	p1, privateKey, _, err := GenerateKeyPair()
	assert.NoError(t, err)

	p2, err := GeneratePublicKey(privateKey)
	assert.NoError(t, err)
	assert.Equal(t, p1, p2)
}

func TestShouldRotatePrivateKey(t *testing.T) {
	var rotate bool
	var err error

	vault = vaultStub{rKey: sampleRegistrationKey, data: sampleJson, exists: true, manifestFileNamePrefix: ""}
	// Test not ssm-agent-worker
	rotate, err = ShouldRotatePrivateKey(log.NewMockLog(), "ssm-agent-worker", 0, true, "", RegVaultKey)
	assert.False(t, rotate)
	assert.NoError(t, err)

	// Test ssm-agent-worker and service says should rotate
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"ssm-agent-worker.exe"}
	rotate, err = ShouldRotatePrivateKey(log.NewMockLog(), "ssm-agent-worker", 0, true, "", RegVaultKey)
	assert.True(t, rotate)
	assert.NoError(t, err)

	// Test ssm-agent-worker and service says should rotate but only amazon-ssm-agent should rotate
	os.Args = []string{"ssm-agent-worker.exe"}
	rotate, err = ShouldRotatePrivateKey(log.NewMockLog(), "amazon-ssm-agent", 0, true, "", RegVaultKey)
	assert.False(t, rotate)
	assert.NoError(t, err)

	// Test amazon-ssm-agent and service says should rotate but only amazon-ssm-agent should rotate
	os.Args = []string{"amazon-ssm-agent.exe"}
	rotate, err = ShouldRotatePrivateKey(log.NewMockLog(), "amazon-ssm-agent", 0, true, "", RegVaultKey)
	assert.True(t, rotate)
	assert.NoError(t, err)

	// Test private key max age less or equal to 0
	os.Args = []string{"amazon-ssm-agent.exe"}
	rotate, err = ShouldRotatePrivateKey(log.NewMockLog(), "amazon-ssm-agent", 0, false, "", RegVaultKey)
	assert.False(t, rotate)
	assert.NoError(t, err)
	rotate, err = ShouldRotatePrivateKey(log.NewMockLog(), "amazon-ssm-agent", -1, false, "", RegVaultKey)
	assert.False(t, rotate)
	assert.NoError(t, err)

	loadedServerInfo.InstanceID = "notempty"
	loadedServerInfoKey = RegVaultKey

	//Test empty created date
	loadedServerInfo.PrivateKeyCreatedDate = ""
	rotate, err = ShouldRotatePrivateKey(log.NewMockLog(), "amazon-ssm-agent", 1, false, "", RegVaultKey)
	assert.True(t, rotate)
	assert.NoError(t, err)

	//Test incorrect date format
	loadedServerInfo.PrivateKeyCreatedDate = "2006-01-02T15:04:05.999999999 PST"
	rotate, err = ShouldRotatePrivateKey(log.NewMockLog(), "amazon-ssm-agent", 1, false, "", RegVaultKey)
	assert.False(t, rotate)
	assert.Error(t, err)

	//Test correct date format with recently rotated key
	loadedServerInfo.PrivateKeyCreatedDate = time.Now().Format(defaultDateStringFormat)
	rotate, err = ShouldRotatePrivateKey(log.NewMockLog(), "amazon-ssm-agent", 1, false, "", RegVaultKey)
	assert.False(t, rotate)
	assert.NoError(t, err)

	//Test correct date format with old key
	loadedServerInfo.PrivateKeyCreatedDate = time.Now().Add(-24 * time.Hour).Format(defaultDateStringFormat)
	rotate, err = ShouldRotatePrivateKey(log.NewMockLog(), "amazon-ssm-agent", 1, false, "", RegVaultKey)
	assert.True(t, rotate)
	assert.NoError(t, err)
}

// stubs

type fileStub struct {
	err error
}

func (f fileStub) WriteAllText(filePath string, text string) (err error) {
	return f.err
}

type vaultStub struct {
	rKey                   string
	data                   []byte
	err                    error
	exists                 bool
	manifestFileNamePrefix string
}

func (v vaultStub) Store(manifestFileNamePrefix string, key string, data []byte) error {
	if manifestFileNamePrefix != v.manifestFileNamePrefix {
		return fmt.Errorf("incorrect manifestFileNamePrefix passed")
	}

	return v.err
}

func (v vaultStub) Retrieve(manifestFileNamePrefix string, key string) ([]byte, error) {
	if manifestFileNamePrefix != v.manifestFileNamePrefix {
		return nil, fmt.Errorf("incorrect manifestFileNamePrefix passed")
	}

	return v.data, v.err
}

func (v vaultStub) IsManifestExists(manifestFileNamePrefix string) bool {
	if manifestFileNamePrefix != v.manifestFileNamePrefix {
		panic(fmt.Errorf("incorrect manifestFileNamePrefix passed"))
	}
	return v.exists
}
