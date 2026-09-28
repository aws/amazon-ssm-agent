package registrar

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/common/identity"
	identitymocks "github.com/aws/amazon-ssm-agent/common/identity/mocks"
	"github.com/aws/amazon-ssm-agent/core/app/context"
	contextmocks "github.com/aws/amazon-ssm-agent/core/app/context/mocks"
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

func commonTimeAfterFuncError(duration time.Duration) <-chan time.Time {
	c := make(chan time.Time, 1)
	c <- time.Now()
	return c
}

func createRetryableRegistrar(identityRegistrar identity.Registrar) *RetryableRegistrar {
	return &RetryableRegistrar{
		log:                       log.NewMockLog(),
		identityRegistrar:         identityRegistrar,
		registrationAttemptedChan: make(chan struct{}, 1),
		stopRegistrarChan:         make(chan struct{}),
		timeAfterFunc:             commonTimeAfterFuncError,
		isRegistrarRunningLock:    &sync.RWMutex{},
	}

}

func TestNewRetryableRegistrar_For_Failure_And_Registrar_Start_Stop(t *testing.T) {
	// Arrange
	identityRegistrar := &identitymocks.Registrar{}
	identityRegistrar.On("Register", mock.Anything).Return(nil)

	// Covering malformed identity
	agentCtx := &contextmocks.ICoreAgentContext{}
	agentIdentity := &identitymocks.IAgentIdentity{}
	agentCtx.On("Identity").Return(agentIdentity)
	agentCtx.On("Log").Return(log.NewMockLog())
	assert.Nil(t, NewRetryableRegistrar(agentCtx))

	//covering auto-registration failure
	agentCtx2 := &contextmocks.ICoreAgentContext{}
	agentIdentity2 := &identitymocks.IAgentIdentity{}
	agentIdentity2.On("InstanceID").Return("SomeInstanceId", nil)
	agentCtx2.On("Identity").Return(agentIdentity2)
	agentIdentity2.On("IInnerIdentityGetter").Return(nil)
	agentCtx2.On("Log").Return(log.NewMockLog())
	innerIdentityGetter := &identitymocks.IInnerIdentityGetter{}
	innerIdentityGetter.On("GetInner").Return(nil)
	castToIdentityInner = func(agentCtx context.ICoreAgentContext) (identity.IInnerIdentityGetter, bool) {
		return innerIdentityGetter, true
	}
	assert.Nil(t, NewRetryableRegistrar(agentCtx2))

	//covering registrar start stop
	registrarTestClose := createRetryableRegistrar(identityRegistrar)
	assert.Nil(t, registrarTestClose.Start())
	registrarTestClose.Stop()
	registrarTestClose.setIsRegistrarRunning(false)
	assert.False(t, registrarTestClose.getIsRegistrarRunning())
	registrarTestClose.Stop()

	registrarTestPanic := createRetryableRegistrar(nil)
	registrarTestPanic.RegisterWithRetry()
	_, ok := <-registrarTestPanic.GetRegistrationAttemptedChan()
	assert.True(t, ok)
	_, ok2 := <-registrarTestPanic.GetRegistrationAttemptedChan()
	assert.False(t, ok2)
}

func TestRetryableRegistrar_RegisterWithRetry_Failure_StopRegistrarChannel(t *testing.T) {
	identityRegistrarReturnError := &identitymocks.Registrar{}
	identityRegistrarReturnError.On("Register", mock.Anything).Return(errors.New("mocked error"))
	registrarError := createRetryableRegistrar(identityRegistrarReturnError)
	registrarError.setIsRegistrarRunning(true)

	go func() {
		registrarError.Stop()
	}()

	registrarError.RegisterWithRetry()
	// Giving some time so that execution reaches at the retry sleeping point
	time.Sleep(15 * time.Second)
}

func TestRetryableRegistrar_RegisterWithRetry_Failure_StopWhenRetrying(t *testing.T) {
	identityRegistrarReturnError := &identitymocks.Registrar{}
	identityRegistrarReturnError.On("Register", mock.Anything).Return(errors.New("mocked error"))
	registrarError := createRetryableRegistrar(identityRegistrarReturnError)
	registrarError.setIsRegistrarRunning(true)

	go registrarError.RegisterWithRetry()
	//Making sure retry happens
	time.Sleep(1 * time.Second)
	registrarError.Stop()
}

//func TestRetryableRegistrar_RegisterWithRetry_Failure_RegistrationAttemptedChannel(t *testing.T) {
//	contextWithCancel = func(parent contextPackage.Context) (contextPackage.Context, contextPackage.CancelFunc) {
//		panic("mocked panic for testing")
//	}
//	identityRegistrarReturnError := &identitymocks.Registrar{}
//	identityRegistrarReturnError.On("Register", mock.Anything).Return(errors.New("mocked error"))
//	registrarErrorContext := createRetryableRegistrar(identityRegistrarReturnError)
//	registrarErrorContext.setIsRegistrarRunning(true)
//
//	go func() {
//		registrarErrorContext.registrationAttemptedChan <- struct{}{}
//	}()
//
//	registrarErrorContext.RegisterWithRetry()
//	_, ok := <-registrarErrorContext.GetRegistrationAttemptedChan()
//	assert.True(t, ok)
//	_, ok2 := <-registrarErrorContext.GetRegistrationAttemptedChan()
//	assert.False(t, ok2)
//}
