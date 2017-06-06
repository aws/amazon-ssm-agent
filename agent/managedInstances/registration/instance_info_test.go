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
	vault = vaultStub{rKey: sampleRegistrationKey, data: sampleJson}
	loadServerInfo() // load info with mocked vault
	region := Region()
	fmt.Println(region)
	// Output:
	// us-west-1
}

func ExampleInstanceID() {
	file = fileStub{}
	vault = vaultStub{rKey: sampleRegistrationKey, data: sampleJson}
	loadServerInfo() // load info with mocked vault
	instanceID := InstanceID()
	fmt.Println(instanceID)
	// Output:
	// mi-e6c6f145e6c6f145
}

func ExamplePrivateKey() {
	file = fileStub{}
	vault = vaultStub{rKey: sampleRegistrationKey, data: sampleJson}
	loadServerInfo() // load info with mocked vault
	privateKey := PrivateKey()
	fmt.Println(privateKey)
	// Output:
	// KEYe6c6f145e6c6f145
}

// TODO: Add more tests once we finalize the store

// stubs

type fileStub struct {
	err error
}

func (f fileStub) WriteAllText(filePath string, text string) (err error) {
	return f.err
}

type vaultStub struct {
	rKey string
	data []byte
	err  error
}

func (v vaultStub) Store(key string, data []byte) error {
	return v.err
}

func (v vaultStub) Retrieve(key string) ([]byte, error) {
	return v.data, v.err
}
