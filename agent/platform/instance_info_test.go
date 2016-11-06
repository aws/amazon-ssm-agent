package platform

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	sampleInstanceError  = "metadata error occured"
	sampleInstanceRegion = "us-east-1"
	sampleInstanceID     = "i-e6c6f145"

	sampleManagedInstError  = "registration error occured"
	sampleManagedInstRegion = "us-west-1"
	sampleManagedInstID     = "mi-e6c6f145e6c6f145"

	sampleDynamicDataError  = "dynamic data error occured"
	sampleDynamicDataRegion = "us-west-2"
)

// metadata stub
type metadataStub struct {
	instanceID string
	region     string
	err        error
	message    string
}

func (c metadataStub) GetMetadata(p string) (string, error) { return c.instanceID, c.err }

func (c metadataStub) Region() (string, error) { return c.region, c.err }

// registration stub
type registrationStub struct {
	instanceID string
	region     string
	err        error
	message    string
}

func (r registrationStub) InstanceID() string { return r.instanceID }

func (r registrationStub) Region() string { return r.region }

// dynamicData stub
type dynamicDataStub struct {
	region  string
	err     error
	message string
}

func (d dynamicDataStub) Region() (string, error) { return d.region, d.err }

// Examples

func ExampleInstanceID() {
	metadata = &metadataStub{instanceID: sampleInstanceID}
	managedInstance = registrationStub{err: errors.New(sampleManagedInstError)}
	result, err := InstanceID()
	fmt.Println(result)
	fmt.Println(err)
	// Output:
	// i-e6c6f145
	// <nil>
}

func ExampleSetInstanceID() {
	err := SetRegion(sampleInstanceID)
	fmt.Println(Region())
	fmt.Println(err)
	// Output:
	// i-e6c6f145 <nil>
	// <nil>
}

func ExampleRegion() {
	metadata = &metadataStub{region: sampleInstanceRegion}
	managedInstance = registrationStub{err: errors.New(sampleManagedInstError)}
	cachedRegion = ""
	result, err := Region()
	fmt.Println(result)
	fmt.Println(err)
	// Output:
	// us-east-1
	// <nil>
}

func ExampleSetRegion() {
	err := SetRegion(sampleInstanceRegion)
	fmt.Println(Region())
	fmt.Println(err)
	// Output:
	// us-east-1 <nil>
	// <nil>
}

// Tests

type instanceInfoTest struct {
	inputMetadata       *metadataStub
	inputRegistration   registrationStub
	inputDynamicData    *dynamicDataStub
	testMessage         string
	expectedID          string
	expectedIDError     error
	expectedRegion      string
	expectedRegionError error
}

var (
	validMetadata       = &metadataStub{instanceID: sampleInstanceID, region: sampleInstanceRegion, err: nil, message: "valid metadata"}
	inValidMetadata     = &metadataStub{err: errors.New(sampleInstanceError), message: "invalid metadata"}
	validRegistration   = registrationStub{instanceID: sampleManagedInstID, region: sampleManagedInstRegion, err: nil, message: "valid registration"}
	inValidRegistration = registrationStub{message: "invalid registration"}
	validDynamicData    = &dynamicDataStub{region: sampleDynamicDataRegion, err: nil, message: "valid dynamic data"}
	inValidDynamicData  = &dynamicDataStub{err: errors.New(sampleDynamicDataError), message: "invalid dynamic data"}

	instanceIDTests = []instanceInfoTest{
		{
			inputMetadata: validMetadata, inputRegistration: validRegistration,
			expectedID: sampleManagedInstID, expectedIDError: nil,
		},
		{
			inputMetadata: validMetadata, inputRegistration: inValidRegistration,
			expectedID: sampleInstanceID, expectedIDError: nil,
		},
		{
			inputMetadata: inValidMetadata, inputRegistration: validRegistration,
			expectedID: sampleManagedInstID, expectedIDError: nil,
		},
		{
			inputMetadata: inValidMetadata, inputRegistration: inValidRegistration,
			expectedID:      "",
			expectedIDError: fmt.Errorf(errorMessage, "instance ID", sampleInstanceError),
		},
	}

	instanceRegionTests = []instanceInfoTest{
		{
			inputMetadata: validMetadata, inputRegistration: validRegistration, inputDynamicData: validDynamicData,
			expectedRegion: sampleManagedInstRegion, expectedRegionError: nil,
		},
		{
			inputMetadata: validMetadata, inputRegistration: inValidRegistration, inputDynamicData: inValidDynamicData,
			expectedRegion: sampleInstanceRegion, expectedRegionError: nil,
		},
		{
			inputMetadata: inValidMetadata, inputRegistration: validRegistration, inputDynamicData: inValidDynamicData,
			expectedRegion: sampleManagedInstRegion, expectedRegionError: nil,
		},
		{
			inputMetadata: inValidMetadata, inputRegistration: inValidRegistration, inputDynamicData: validDynamicData,
			expectedRegion: sampleDynamicDataRegion, expectedRegionError: nil,
		},
		{
			inputMetadata: inValidMetadata, inputRegistration: inValidRegistration, inputDynamicData: inValidDynamicData,
			expectedRegion:      "",
			expectedRegionError: fmt.Errorf(errorMessage, "region", sampleDynamicDataError),
		},
	}
)

func TestFetchInstanceID(t *testing.T) {
	for _, test := range instanceIDTests {
		metadata = test.inputMetadata
		managedInstance = test.inputRegistration
		actualOutput, actualError := fetchInstanceID()
		assert.Equal(t, test.expectedID, actualOutput, "%s %s", test.inputMetadata.message, test.inputRegistration.message)
		assert.Equal(t, test.expectedIDError, actualError, "%s %s", test.inputMetadata.message, test.inputRegistration.message)
	}
}

func TestFetchRegion(t *testing.T) {
	for _, test := range instanceRegionTests {
		metadata = test.inputMetadata
		managedInstance = test.inputRegistration
		dynamicData = test.inputDynamicData
		actualOutput, actualError := fetchRegion()
		assert.Equal(t, test.expectedRegion, actualOutput, "%s %s, %s", test.inputMetadata.message, test.inputRegistration.message, test.inputDynamicData.message)
		assert.Equal(t, test.expectedRegionError, actualError, "%s %s, %s", test.inputMetadata.message, test.inputRegistration.message, test.inputDynamicData.message)
	}
}
