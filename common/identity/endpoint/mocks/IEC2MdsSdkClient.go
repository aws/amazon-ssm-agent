package mocks

import "github.com/stretchr/testify/mock"

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
