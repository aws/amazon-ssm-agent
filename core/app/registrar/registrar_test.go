package registrar

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/aws/amazon-ssm-agent/agent/mocks/log"
	identitymocks "github.com/aws/amazon-ssm-agent/common/identity/mocks"
)

func TestRetryableRegistrar_RegisterWithRetry_Success(t *testing.T) {
	// Arrange
	identityRegistrar := &identitymocks.Registrar{}
	identityRegistrar.On("Register", mock.Anything).Return(nil)

	timeAfterFunc := func(duration time.Duration) <-chan time.Time {
		assert.Fail(t, "expected no registration retry or sleep")
		c := make(chan time.Time, 1)
		c <- time.Now()
		return c
	}

	registrar := &RetryableRegistrar{
		log:                       log.NewMockLog(),
		identityRegistrar:         identityRegistrar,
		registrationAttemptedChan: make(chan struct{}, 1),
		stopRegistrarChan:         make(chan struct{}),
		timeAfterFunc:             timeAfterFunc,
		isRegistrarRunningLock:    &sync.RWMutex{},
	}

	// Act
	registrar.RegisterWithRetry()

	// Assert
	assert.False(t, registrar.getIsRegistrarRunning())
	select {
	case <-registrar.GetRegistrationAttemptedChan():
		break
	case <-time.After(time.Second):
		assert.Fail(t, "expected registrationAttemptedChan to contain value")
	}
}
