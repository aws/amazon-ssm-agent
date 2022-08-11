package registrar

import (
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
	identitymocks "github.com/aws/amazon-ssm-agent/common/identity/mocks"
	"github.com/stretchr/testify/assert"
)

func TestRetryableRegistrar_RegisterWithRetry_Success(t *testing.T) {
	// Arrange
	identityRegistrar := &identitymocks.Registrar{}
	identityRegistrar.On("Register").Return(nil)
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
	}

	// Act
	registrar.RegisterWithRetry()

	// Assert
	assert.False(t, registrar.isRegistrarRunning)
	select {
	case <-registrar.registrationAttemptedChan:
		break
	case <-time.After(time.Second):
		assert.Fail(t, "expected registrationAttemptedChan to contain value")
	}
}
