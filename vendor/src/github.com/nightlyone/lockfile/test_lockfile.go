package lockfile

import (
"os"

"github.com/stretchr/testify/mock"
)

// Note: This code is used in the test files. However, this code is not in a _test.go file
// because then we would have to copy it in every test package that needs the mock.

// MockLockfile stands for a mock lockfile.
type MockLockfile struct {
	mock.Mock
}

// GetOwner mocks the method with the same name.
func (mockPool MockLockfile) GetOwner() (*os.Process, error) {
	return nil, mockPool.Called().Error(0)
}

// ChangeOwner mocks the method with the same name.
func (mockPool MockLockfile) ChangeOwner(pid int) error {
	return mockPool.Called(pid).Error(0)
}

// Unlock mocks the method with the same name.
func (mockPool MockLockfile) Unlock() error {
	return mockPool.Called().Error(0)
}

// TryLock mocks the method with the same name.
func (mockPool MockLockfile) TryLock() error {
	return mockPool.Called().Error(0)
}

// TryLockExpire mocks the method with the same name.
func (mockPool MockLockfile) TryLockExpire(minutes int64) error {
	return mockPool.Called(minutes).Error(0)
}

// TryLockExpireWithRetry mocks the method with the same name.
func (mockPool MockLockfile) TryLockExpireWithRetry(minutes int64) error {
	return mockPool.Called(minutes).Error(0)
}

// ShouldRetry mocks the method with the same name.
func (mockPool MockLockfile) ShouldRetry(err error) bool {
	return mockPool.Called(err).Bool(0)
}
