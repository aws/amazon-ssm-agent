// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the AWS Customer Agreement (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/agreement/

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

func TestCompareHardwareHash_FailOnEmpty(t *testing.T) {
	var actual bool
	savedHwHash := make(map[string]string)
	currentHwHash := make(map[string]string)

	actual = compareHardwareHash(savedHwHash, currentHwHash, minimumMatchPercent)
	assert.False(t, actual, "should return false when both map are empty")

	// add one item to saved
	savedHwHash["first"] = "first"
	actual = compareHardwareHash(savedHwHash, currentHwHash, minimumMatchPercent)
	assert.False(t, actual, "should return false when curr map is empty")

	// add one item to current
	delete(savedHwHash, "first")
	currentHwHash["first"] = "first"
	actual = compareHardwareHash(savedHwHash, currentHwHash, minimumMatchPercent)
	assert.False(t, actual, "should return false when saved map is empty")
}

func TestCompareHardwareHash_FailWhenNoMatch(t *testing.T) {
	var actual bool
	savedHwHash := make(map[string]string)
	currentHwHash := make(map[string]string)

	// fail when items don't match
	savedHwHash[hardwareID] = "second"
	currentHwHash[hardwareID] = "first"
	actual = compareHardwareHash(savedHwHash, currentHwHash, minimumMatchPercent)
	assert.False(t, actual, "should return false when items don't match")

	// fail when items don't exist
	savedHwHash["second"] = "second"
	actual = compareHardwareHash(savedHwHash, currentHwHash, minimumMatchPercent)
	assert.False(t, actual, "should return false when items don't match")

	// fail when items don't exist
	currentHwHash["third"] = "third"
	actual = compareHardwareHash(savedHwHash, currentHwHash, minimumMatchPercent)
	assert.False(t, actual, "should return false when items don't match")
}

func TestCompareHardwareHash_SucceedWhenMatch(t *testing.T) {
	var actual bool
	savedHwHash := make(map[string]string)
	currentHwHash := make(map[string]string)

	// succeed when items match
	savedHwHash["first"] = "first"
	currentHwHash["first"] = "first"
	actual = compareHardwareHash(savedHwHash, currentHwHash, minimumMatchPercent)
	assert.True(t, actual, "should return true when items match")

	// succeed when items match
	savedHwHash["second"] = "second"
	currentHwHash["second"] = "second"
	savedHwHash["third"] = "third"
	currentHwHash["third"] = "third"
	savedHwHash["fourth"] = "fourth"
	actual = compareHardwareHash(savedHwHash, currentHwHash, minimumMatchPercent)
	assert.True(t, actual, "should return true when items match at least 70%")
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
