package mocks

import (
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/stretchr/testify/mock"
)

// IEC2MdsSdkClient is a mock type of iEC2MdsSdkClient
type IEC2MdsSdkClient struct {
	mock.Mock
}

// GetMetadata mocks the GetMetadata function
func (_m *IEC2MdsSdkClient) GetMetadata(metadataResource string) (string, error) {
	ret := _m.Called(metadataResource)
	var dynamicServiceDomain string
	if rf, ok := ret.Get(0).(func(string) string); ok {
		dynamicServiceDomain = rf(metadataResource)
	} else {
		dynamicServiceDomain = ret.Get(0).(string)
	}

	var err error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		err = rf(metadataResource)
	} else {
		err = ret.Error(1)
	}

	return dynamicServiceDomain, err
}

// GetInstanceIdentityDocument mocks the GetInstanceIdentityDocument function
func (_m *IEC2MdsSdkClient) GetInstanceIdentityDocument() (ec2metadata.EC2InstanceIdentityDocument, error) {
	ret := _m.Called()
	var r0 ec2metadata.EC2InstanceIdentityDocument

	if rf, ok := ret.Get(0).(func() ec2metadata.EC2InstanceIdentityDocument); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(ec2metadata.EC2InstanceIdentityDocument)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Region mocks the Region function
func (_m *IEC2MdsSdkClient) Region() (string, error) {
	ret := _m.Called()

	var r0 string

	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
