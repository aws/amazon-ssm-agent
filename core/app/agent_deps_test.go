package app

import (
	"github.com/aws/amazon-ssm-agent/common/identity"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/stretchr/testify/mock"
)

type MockIdentity struct {
	mock.Mock
}

// AvailabilityZone provides a mock function with given fields:
func (_m *MockIdentity) AvailabilityZone() (string, error) {
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

// Credentials provides a mock function with given fields:
func (_m *MockIdentity) Credentials() *credentials.Credentials {
	ret := _m.Called()

	var r0 *credentials.Credentials
	if rf, ok := ret.Get(0).(func() *credentials.Credentials); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*credentials.Credentials)
		}
	}

	return r0
}

// GetServiceEndpoint provides a mock function with given fields: _a0
func (_m *MockIdentity) GetServiceEndpoint(_a0 string) string {
	ret := _m.Called(_a0)

	var r0 string
	if rf, ok := ret.Get(0).(func(string) string); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// IdentityType provides a mock function with given fields:
func (_m *MockIdentity) IdentityType() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// InstanceID provides a mock function with given fields:
func (_m *MockIdentity) InstanceID() (string, error) {
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

// InstanceType provides a mock function with given fields:
func (_m *MockIdentity) InstanceType() (string, error) {
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

// Region provides a mock function with given fields:
func (_m *MockIdentity) Region() (string, error) {
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

// ShortInstanceID provides a mock function with given fields:
func (_m *MockIdentity) ShortInstanceID() (string, error) {
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

// GetInner provides a mock function with given fields:
func (_m *MockIdentity) GetInner() identity.IAgentIdentityInner {
	ret := _m.Called()

	var r0 identity.IAgentIdentityInner
	if rf, ok := ret.Get(0).(func() identity.IAgentIdentityInner); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(identity.IAgentIdentityInner)
		}
	}

	return r0
}

type MockInnerIdentityRegistrar struct {
	mock.Mock
}

// AvailabilityZone provides a mock function with given fields:
func (_m *MockInnerIdentityRegistrar) AvailabilityZone() (string, error) {
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

// Credentials provides a mock function with given fields:
func (_m *MockInnerIdentityRegistrar) Credentials() *credentials.Credentials {
	ret := _m.Called()

	var r0 *credentials.Credentials
	if rf, ok := ret.Get(0).(func() *credentials.Credentials); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*credentials.Credentials)
		}
	}

	return r0
}

// IdentityType provides a mock function with given fields:
func (_m *MockInnerIdentityRegistrar) IdentityType() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// InstanceID provides a mock function with given fields:
func (_m *MockInnerIdentityRegistrar) InstanceID() (string, error) {
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

// InstanceType provides a mock function with given fields:
func (_m *MockInnerIdentityRegistrar) InstanceType() (string, error) {
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

// IsIdentityEnvironment provides a mock function with given fields:
func (_m *MockInnerIdentityRegistrar) IsIdentityEnvironment() bool {
	ret := _m.Called()

	var r0 bool
	if rf, ok := ret.Get(0).(func() bool); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// Region provides a mock function with given fields:
func (_m *MockInnerIdentityRegistrar) Region() (string, error) {
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

// ServiceDomain provides a mock function with given fields:
func (_m *MockInnerIdentityRegistrar) ServiceDomain() (string, error) {
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

// VpcPrimaryCIDRBlock provides a mock function with given fields:
func (_m *MockInnerIdentityRegistrar) VpcPrimaryCIDRBlock() (map[string][]string, error) {
	ret := _m.Called()

	var r0 map[string][]string
	if rf, ok := ret.Get(0).(func() map[string][]string); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(map[string][]string)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Register provides a mock function with given fields:
func (_m *MockInnerIdentityRegistrar) Register() {
	_m.Called()
}
