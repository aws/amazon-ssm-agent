package platform

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	sampleInstanceError  = "metadata error occurred"
	sampleInstanceRegion = "us-east-1"
	sampleInstanceID     = "i-e6c6f145"

	sampleManagedInstError  = "registration error occurred"
	sampleManagedInstRegion = "us-west-1"
	sampleManagedInstID     = "mi-e6c6f145e6c6f145"

	sampleDynamicDataError  = "dynamic data error occurred"
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
	instanceID       string
	region           string
	instanceType     string
	availabilityZone string
	err              error
	message          string
}

func (r registrationStub) InstanceID() string { return r.instanceID }

func (r registrationStub) Region() string { return r.region }

func (r registrationStub) InstanceType() string { return r.instanceType }

func (r registrationStub) AvailabilityZone() string { return r.availabilityZone }

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
	invalidMetadata     = &metadataStub{err: errors.New(sampleInstanceError), message: "invalid metadata"}
	validRegistration   = registrationStub{instanceID: sampleManagedInstID, region: sampleManagedInstRegion, instanceType: "on-premises", availabilityZone: "on-premises", err: nil, message: "valid registration"}
	invalidRegistration = registrationStub{message: "invalid registration"}
	validDynamicData    = &dynamicDataStub{region: sampleDynamicDataRegion, err: nil, message: "valid dynamic data"}
	invalidDynamicData  = &dynamicDataStub{err: errors.New(sampleDynamicDataError), message: "invalid dynamic data"}

	instanceIDTests = []instanceInfoTest{
		{
			inputMetadata: validMetadata, inputRegistration: validRegistration,
			expectedID: sampleManagedInstID, expectedIDError: nil,
		},
		{
			inputMetadata: validMetadata, inputRegistration: invalidRegistration,
			expectedID: sampleInstanceID, expectedIDError: nil,
		},
		{
			inputMetadata: invalidMetadata, inputRegistration: validRegistration,
			expectedID: sampleManagedInstID, expectedIDError: nil,
		},
		{
			inputMetadata: invalidMetadata, inputRegistration: invalidRegistration,
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
			inputMetadata: validMetadata, inputRegistration: invalidRegistration, inputDynamicData: invalidDynamicData,
			expectedRegion: sampleInstanceRegion, expectedRegionError: nil,
		},
		{
			inputMetadata: invalidMetadata, inputRegistration: validRegistration, inputDynamicData: invalidDynamicData,
			expectedRegion: sampleManagedInstRegion, expectedRegionError: nil,
		},
		{
			inputMetadata: invalidMetadata, inputRegistration: invalidRegistration, inputDynamicData: validDynamicData,
			expectedRegion: sampleDynamicDataRegion, expectedRegionError: nil,
		},
		{
			inputMetadata: invalidMetadata, inputRegistration: invalidRegistration, inputDynamicData: invalidDynamicData,
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

func TestFetchInstanceTypeForOnPremisesInstance(t *testing.T) {
	// this tests that validRegistration is used to retrieve the value and
	// no attempt is made to get it from invalidMetadata or invalidDynamicData
	metadata = invalidMetadata
	managedInstance = validRegistration
	dynamicData = invalidDynamicData
	actualOutput, actualError := fetchInstanceType()
	assert.Equal(t, validRegistration.InstanceType(), actualOutput)
	assert.Equal(t, nil, actualError)
}

func TestFetchAvailabilityZoneForOnPremisesInstance(t *testing.T) {
	// this tests that validRegistration is used to retrieve the value and
	// no attempt is made to get it from invalidMetadata or invalidDynamicData
	metadata = invalidMetadata
	managedInstance = validRegistration
	dynamicData = invalidDynamicData
	actualOutput, actualError := fetchAvailabilityZone()
	assert.Equal(t, validRegistration.AvailabilityZone(), actualOutput)
	assert.Equal(t, nil, actualError)
}

func TestFetchInstanceTypeForEc2Instance(t *testing.T) {
	// this tests that the metadata instanceType is returned, because there
	// is no valid managedInstance data
	metadata = validMetadata
	managedInstance = invalidRegistration
	dynamicData = invalidDynamicData
	actualOutput, actualError := fetchInstanceType()
	// GetMetadata() actually is stubbed out trivially, always returning the
	// same string, but for this test we only care that that is the string
	// returned by fetchInstanceType()
	var value, _ = metadata.GetMetadata("instance-type")
	assert.Equal(t, value, actualOutput)
	assert.Equal(t, nil, actualError)
}

func TestFetchAvailabilityZoneForEc2Instance(t *testing.T) {
	// this tests that the metadata AZ is returned, because there
	// is no valid managedInstance data
	metadata = validMetadata
	managedInstance = invalidRegistration
	dynamicData = invalidDynamicData
	actualOutput, actualError := fetchAvailabilityZone()
	// GetMetadata() actually is stubbed out trivially, always returning the
	// same string, but for this test we only care that that is the string
	// returned by fetchAvailabilityZone()
	var value, _ = metadata.GetMetadata("placement/availability-zone")
	assert.Equal(t, value, actualOutput)
	assert.Equal(t, nil, actualError)
}
