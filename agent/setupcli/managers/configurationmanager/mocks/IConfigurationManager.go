// Code generated by mockery v2.9.4. DO NOT EDIT.
package mocks

import mock "github.com/stretchr/testify/mock"

// IConfigurationManager is an autogenerated mock type for the IConfigurationManager type
type IConfigurationManager struct {
	mock.Mock
}

// ConfigureAgent provides a mock function with given fields: folderPath
func (_m *IConfigurationManager) ConfigureAgent(folderPath string) error {
	ret := _m.Called(folderPath)

	var r0 error
	if rf, ok := ret.Get(0).(func(string) error); ok {
		r0 = rf(folderPath)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// CreateUpdateAgentConfigWithOnPremIdentity provides a mock function with given fields:
func (_m *IConfigurationManager) CreateUpdateAgentConfigWithOnPremIdentity() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// IsConfigAvailable provides a mock function with given fields: folderPath
func (_m *IConfigurationManager) IsConfigAvailable(folderPath string) bool {
	ret := _m.Called(folderPath)

	var r0 bool
	if rf, ok := ret.Get(0).(func(string) bool); ok {
		r0 = rf(folderPath)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}