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

package fingerprint

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	sampleFingerprint = "979b554b-0d67-42c6-9730-48443b3016dd"
)

func ExampleInstanceFingerprint() {
	currentHwHash = func() map[string]string {
		hwHash := make(map[string]string)
		hwHash["sample"] = "sample"
		return hwHash
	}

	savedHwHash := currentHwHash()

	saved := hwInfo{
		Fingerprint:  sampleFingerprint,
		HardwareHash: savedHwHash,
	}

	savedJson, _ := json.Marshal(saved)

	vault = vaultStub{
		rKey: vaultKey,
		err:  nil,
		data: savedJson,
	}

	val, _ := InstanceFingerprint()
	fmt.Println(val)

	// Output:
	// 979b554b-0d67-42c6-9730-48443b3016dd
}

type isSimilarHashTestData struct {
	saved     map[string]string
	current   map[string]string
	threshold int
	expected  bool
}

func TestIsSimilarHardwareHash(t *testing.T) {
	empty := make(map[string]string)

	origin := map[string]string{
		hardwareID:      "hardwareValue",
		ipAddressID:     "ipAddressValue",
		"somethingElse": "somethingElseValue",
	}

	hwChanged := deepCopy(origin)
	hwChanged[hardwareID] = "hardwareValueChanged"

	ipChanged := deepCopy(origin)
	ipChanged[ipAddressID] = "ipAddressValueChanged"

	ipAndElseChanged := deepCopy(origin)
	ipAndElseChanged[ipAddressID] = "ipAddressValueChanged"
	ipAndElseChanged["somethingElse"] = "somethingElseValueChanged"

	somethingElseChanged := deepCopy(origin)
	somethingElseChanged["somethingElse"] = "somethingElseValueChanged"

	testData := []isSimilarHashTestData{
		{origin, empty, 0, false},
		{empty, origin, 0, false},
		{origin, origin, 100, true},
		{origin, hwChanged, 0, false},
		{origin, ipChanged, 66, true},         // 2 out of 3 items matched > 66%
		{origin, ipChanged, 67, false},        // 2 out of 3 items matched < 67%
		{origin, ipAndElseChanged, 33, true},  // 1 out of 3 items matched > 33%
		{origin, ipAndElseChanged, 34, false}, // 1 out of 3 items matched < 34%
		{origin, somethingElseChanged, 100, true},
	}

	for _, test := range testData {
		assert.Equal(
			t,
			test.expected,
			isSimilarHardwareHash(test.saved, test.current, test.threshold),
			fmt.Sprintf("Test case %v did not return %t.", test, test.expected),
		)
	}
}

func deepCopy(original map[string]string) (copied map[string]string) {
	copied = make(map[string]string)
	for k, v := range original {
		copied[k] = v
	}
	return
}

func TestGenerateFingerprint_GenerateNewWhenNoneSaved(t *testing.T) {
	currentHwHash = func() map[string]string {
		hwHash := make(map[string]string)
		hwHash["sample"] = "sample"
		return hwHash
	}

	vault = vaultStub{
		rKey: vaultKey,
		err:  nil,
		data: nil,
	}

	actual, err := generateFingerprint()

	assert.NoError(t, err, "expected no error from the call")

	assert.NotEmpty(t, actual, "expected the instance to generate a fingerprint")
}

func TestGenerateFingerprint_ReturnSavedWhenMatched(t *testing.T) {
	currentHwHash = func() map[string]string {
		hwHash := make(map[string]string)
		hwHash["sample"] = "sample"
		return hwHash
	}

	savedHwHash := currentHwHash()

	saved := hwInfo{
		Fingerprint:  sampleFingerprint,
		HardwareHash: savedHwHash,
	}

	savedJson, _ := json.Marshal(saved)

	vault = vaultStub{
		rKey: vaultKey,
		err:  nil,
		data: savedJson,
	}

	actual, err := generateFingerprint()

	assert.NoError(t, err, "expected no error from the call")

	assert.Equal(t, sampleFingerprint, actual, "expected the instance to generate a fingerprint")
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
