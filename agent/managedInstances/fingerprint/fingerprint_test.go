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
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
)

const (
	sampleFingerprint = "979b554b-0d67-42c6-9730-48443b3016dd"
	invalidUTF8String = "\xbd\xb2\x3d\xbc\x20\xe2\x8c\x98"
)

/*
func ExampleInstanceFingerprint() {
	setLogger(log.NewMockLog())

	currentHwHash = func() (map[string]string, error) {
		hwHash := make(map[string]string)
		hwHash[hardwareID] = "original"
		return hwHash, nil
	}

	savedHwHash, _ := currentHwHash()

	saved := hwInfo{
		Fingerprint:  sampleFingerprint,
		HardwareHash: savedHwHash,
	}

	savedJson, _ := json.Marshal(saved)

	vault = vaultStub{
		rKey:        vaultKey,
		data:        savedJson,
		storeErr:    nil,
		retrieveErr: nil,
	}

	val, _ := InstanceFingerprint()
	fmt.Println(val)
	// Output:
	// 979b554b-0d67-42c6-9730-48443b3016dd
}
*/

type isSimilarHashTestData struct {
	saved     map[string]string
	current   map[string]string
	threshold int
	expected  bool
}

func TestIsSimilarHardwareHash(t *testing.T) {
	empty := make(map[string]string)
	log := log.NewMockLog()

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
		{origin, empty, -1, true},
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
			isSimilarHardwareHash(log, test.saved, test.current, test.threshold),
			fmt.Sprintf("Test case %v did not return %t.", test, test.expected),
		)
	}

}

func TestIsSimilarHardwareHash_WritesFingerprintDiagnosticsInLog(t *testing.T) {

	logger := NewFakeLog(1)

	originalFingerprint := map[string]string{
		hardwareID:  "myHardwareID",
		ipAddressID: "myIpAddress",
		"key1":      "originalValue1",
		"key2":      "originalValue2",
		"key3":      "originalValue3",
		"key4":      "originalValue4",
	}

	currentFingerprint := map[string]string{
		hardwareID:  "myHardwareID",
		ipAddressID: "newIpAddress",
		"key1":      "newValue1",
		"key2":      "newValue2",
		"key3":      "originalValue3",
		"key4":      "newValue4",
	}

	assert.False(
		t,
		isSimilarHardwareHash(logger, originalFingerprint, currentFingerprint, 100),
		"isSimilarHardwareHash returned true when current and original are different")

	assert.True(
		t,
		arrayContainsSubstring(logger.errorMessages, "Cannot connect to AWS Systems Manager"),
		"isSimilarHardwareHash did not log an error saying it couldn't connect")

	assert.True(
		t,
		arrayContainsSubstring(logger.warnMessages, "key1"),
		"isSimilarHardwareHash did not log a warning saying that key1 was changed")

	assert.True(
		t,
		arrayContainsSubstring(logger.warnMessages, "key2"),
		"isSimilarHardwareHash did not log a warning saying that key2 was changed")

	assert.True(
		t,
		arrayContainsSubstring(logger.warnMessages, "key4"),
		"isSimilarHardwareHash did not log a warning saying that key4 was changed")

	assert.False(
		t,
		arrayContainsSubstring(logger.warnMessages, "key3"),
		"isSimilarHardwareHash logged a false warning saying that key3 was changed")
}

func deepCopy(original map[string]string) (copied map[string]string) {
	copied = make(map[string]string)
	for k, v := range original {
		copied[k] = v
	}
	return
}

func TestGenerateFingerprint_FailGenerateHwHash(t *testing.T) {
	// Arrange
	failedGenerateHwHashError := "Failed to generate hardware hash"
	currentHwHash = func() (map[string]string, error) {
		return make(map[string]string), fmt.Errorf(failedGenerateHwHashError)
	}

	// Act
	fingerprint, err := generateFingerprint(log.NewMockLog())

	// Assert
	assert.Error(t, err, "expected no error from the call")
	assert.Equal(t, "", fingerprint, "Expected empty fingerprint")
	assert.Equal(t, failedGenerateHwHashError, err.Error(), "Expected HwHash error")
}

func TestGenerateFingerprint_GenerateNewWhenNoneSaved(t *testing.T) {
	// Arrange
	currentHwHash = func() (map[string]string, error) {
		hwHash := make(map[string]string)
		hwHash[hardwareID] = "original"
		return hwHash, nil
	}

	vault = vaultStub{
		rKey:     vaultKey,
		storeErr: nil,
		data:     nil,
	}

	// Act
	actual, err := generateFingerprint(log.NewMockLog())

	// Assert
	assert.NoError(t, err, "expected no error from the call")
	assert.NotEmpty(t, actual, "expected the instance to generate a fingerprint")

}

func TestGenerateFingerprint_ReturnSavedWhenMatched(t *testing.T) {
	// Arrange
	currentHwHash = func() (map[string]string, error) {
		hwHash := make(map[string]string)
		hwHash[hardwareID] = "original"
		return hwHash, nil
	}

	savedHwHash, _ := currentHwHash()

	saved := hwInfo{
		Fingerprint:  sampleFingerprint,
		HardwareHash: savedHwHash,
	}

	savedJson, _ := json.Marshal(saved)

	vault = vaultStub{
		rKey:        vaultKey,
		data:        savedJson,
		storeErr:    nil,
		retrieveErr: nil,
	}

	// Act
	actual, err := generateFingerprint(log.NewMockLog())

	// Assert
	assert.NoError(t, err, "expected no error from the call")
	assert.Equal(t, sampleFingerprint, actual, "expected the instance to generate a fingerprint")

}

func TestGenerateFingerprint_ReturnUpdated_WhenHardwareHashesDontMatch(t *testing.T) {
	// Arrange
	currentHwHash = func() (map[string]string, error) {
		hwHash := make(map[string]string)
		hwHash[hardwareID] = "changed"
		return hwHash, nil
	}
	savedHwHash := getHwHash("original")

	saved := hwInfo{
		Fingerprint:  sampleFingerprint,
		HardwareHash: savedHwHash,
	}

	savedJson, _ := json.Marshal(saved)

	vault = vaultStub{
		rKey:        vaultKey,
		data:        savedJson,
		storeErr:    nil,
		retrieveErr: nil,
	}

	// Act
	actual, err := generateFingerprint(log.NewMockLog())

	// Assert
	assert.NoError(t, err, "expected no error from the call")
	assert.NotEqual(t, sampleFingerprint, actual, "expected the instance to generate a fingerprint")
}

func TestGenerateFingerprint_ReturnsError_WhenInvalidCharactersInHardwareHash(t *testing.T) {
	// Arrange
	currentHwHash = func() (map[string]string, error) {
		hwHash := make(map[string]string)
		hwHash[hardwareID] = invalidUTF8String
		return hwHash, nil
	}

	vaultMock := &fpFsVaultMock{}
	vault = vaultMock

	//Act
	fingerprint, err := generateFingerprint(log.NewMockLog())

	//Assert
	assert.Error(t, err)
	assert.Empty(t, fingerprint)
}

func TestGenerateFingerprint_DoesNotSave_WhenHardwareHashesMatch(t *testing.T) {
	// Arrange
	savedHwHash := getHwHash("original")
	currentHwHash = func() (map[string]string, error) {
		return savedHwHash, nil
	}
	savedHwInfo := &hwInfo{
		HardwareHash:        savedHwHash,
		Fingerprint:         sampleFingerprint,
		SimilarityThreshold: defaultMatchPercent,
	}

	savedHwData, _ := json.Marshal(savedHwInfo)

	vaultMock := &fpFsVaultMock{}
	vaultMock.On("Retrieve", vaultKey).Return(savedHwData, nil).Once()
	vault = vaultMock

	// Act
	generateFingerprint(log.NewMockLog())
}

func TestSave_SavesNewFingerprint(t *testing.T) {
	// Arrange
	sampleHwHash := getHwHash("backup")
	sampleHwInfo := hwInfo{
		HardwareHash:        sampleHwHash,
		Fingerprint:         sampleFingerprint,
		SimilarityThreshold: defaultMatchPercent,
	}
	sampleHwInfoData, _ := json.Marshal(sampleHwInfo)
	vaultMock := &fpFsVaultMock{}
	vaultMock.On("Store", vaultKey, sampleHwInfoData).Return(nil)
	vault = vaultMock

	// Act
	err := save(sampleHwInfo)

	// Assert
	assert.NoError(t, err)
}

func TestIsValidHardwareHash_ReturnsHashIsValid(t *testing.T) {
	// Arrange
	sampleHash := make(map[string]string)
	sampleHash[hardwareID] = "sample"

	// Act
	isValid := isValidHardwareHash(sampleHash)

	// Assert
	assert.True(t, isValid)
}

func TestIsValidHardwareHash_ReturnsHashIsInvalid(t *testing.T) {
	// Arrange
	sampleHash := make(map[string]string)
	sampleHash[hardwareID] = invalidUTF8String

	//Act
	isValid := isValidHardwareHash(sampleHash)

	// Assert
	assert.False(t, isValid)
}

func getHwHash(sampleValue string) map[string]string {
	hwHash := make(map[string]string)
	hwHash[hardwareID] = sampleValue
	return hwHash
}

type vaultStub struct {
	rKey        string
	data        []byte
	storeErr    error
	retrieveErr error
}

func (v vaultStub) Store(key string, data []byte) error {
	return v.storeErr
}

func (v vaultStub) Retrieve(key string) ([]byte, error) {
	return v.data, v.retrieveErr
}

// fakeLog is a test double for the seelog logger to verify that particular strings were written into the log
// during a test.
type fakeLog struct {
	traceMessages    []string
	debugMessages    []string
	infoMessages     []string
	warnMessages     []string
	errorMessages    []string
	criticalMessages []string
}

func (f *fakeLog) Trace(v ...interface{}) {
	f.traceMessages = append(f.traceMessages, concat(v...))
}

func (f *fakeLog) Tracef(format string, params ...interface{}) {
	f.traceMessages = append(f.traceMessages, fmt.Sprintf(format, params...))
}

func (f *fakeLog) Debug(v ...interface{}) {
	f.debugMessages = append(f.debugMessages, concat(v...))
}

func (f *fakeLog) Debugf(format string, params ...interface{}) {
	f.debugMessages = append(f.debugMessages, fmt.Sprintf(format, params...))
}

func (f *fakeLog) Info(v ...interface{}) {
	message := concat(v...)
	f.infoMessages = append(f.infoMessages, message)
}

func (f *fakeLog) Infof(format string, params ...interface{}) {
	message := fmt.Sprintf(format, params...)
	f.infoMessages = append(f.infoMessages, message)
}

func (f *fakeLog) Warn(v ...interface{}) error {
	message := concat(v...)
	f.warnMessages = append(f.warnMessages, message)
	return errors.New("Warning: " + message)
}

func (f *fakeLog) Warnf(format string, params ...interface{}) error {
	message := fmt.Sprintf(format, params...)
	f.warnMessages = append(f.warnMessages, message)
	return errors.New("Warning: " + message)
}

func (f *fakeLog) Error(v ...interface{}) error {
	message := concat(v...)
	f.errorMessages = append(f.errorMessages, message)
	return errors.New("Error: " + message)
}

func (f *fakeLog) Errorf(format string, params ...interface{}) error {
	message := fmt.Sprintf(format, params...)
	f.errorMessages = append(f.errorMessages, message)
	return errors.New("Error: " + message)
}

func (f *fakeLog) Critical(v ...interface{}) error {
	message := concat(v...)
	f.criticalMessages = append(f.criticalMessages, message)
	return errors.New("Critical: " + message)
}

func (f *fakeLog) Criticalf(format string, params ...interface{}) error {
	message := fmt.Sprintf(format, params...)
	f.criticalMessages = append(f.criticalMessages, message)
	return errors.New("Critical: " + message)
}

func (f fakeLog) Flush() {}

func (f fakeLog) Close() {}

func (f fakeLog) Closed() bool {
	return false
}

func (f fakeLog) WithContext(_ ...string) log.T {
	return nil
}

func (f fakeLog) WriteEvent(_ string, _ string, _ string) {}

func NewFakeLog(capacity int) *fakeLog {
	return &fakeLog{
		traceMessages:    make([]string, 0, capacity),
		debugMessages:    make([]string, 0, capacity),
		infoMessages:     make([]string, 0, capacity),
		warnMessages:     make([]string, 0, capacity),
		errorMessages:    make([]string, 0, capacity),
		criticalMessages: make([]string, 0, capacity),
	}
}

func concat(v ...interface{}) string {
	message := ""
	for _, item := range v {
		if len(message) != 0 {
			message += ";"
		}

		message += fmt.Sprintf("%v", item)
	}

	return message
}

func arrayContainsSubstring(haystacks []string, needle string) bool {
	result := false

	for _, haystack := range haystacks {
		if strings.Contains(haystack, needle) {
			result = true
			break
		}
	}

	return result
}
