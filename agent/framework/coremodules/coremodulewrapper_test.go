package coremodules

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/contracts/mocks"
	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
)

func createTestModuleWrapper(module contracts.ICoreModule) *CoreModuleWrapper {
	return &CoreModuleWrapper{
		module:      module,
		log:         logger.NewMockLog(),
		mtx:         &sync.Mutex{},
		started:     false,
		stopStarted: false,
		stoppedChan: make(chan struct{}),
		stopErr:     nil,
	}
}

func TestCoreModuleWrapper_Stop_PanicAssertChanNotClosed(t *testing.T) {
	module := &mocks.ICoreModule{}
	module.On("ModuleName").Return("SomeModuleName")

	module.On("ModuleStop").Return(func() error {
		panic(fmt.Errorf("SomePanicError"))
	}).Once()

	wrapper := createTestModuleWrapper(module)
	wrapper.stop()
	assert.NotNil(t, wrapper.stoppedChan)
	module.AssertExpectations(t)
}

func TestCoreModuleWrapper_Stop_AssertChanClosed(t *testing.T) {
	expErr := fmt.Errorf("SomeError")

	module := &mocks.ICoreModule{}
	module.On("ModuleStop").Return(expErr).Once()

	wrapper := createTestModuleWrapper(module)
	wrapper.stop()

	isClosed := false
	select {
	case <-wrapper.stoppedChan:
		isClosed = true
	default:
	}
	assert.True(t, isClosed)
	assert.Equal(t, expErr, wrapper.stopErr)
	module.AssertExpectations(t)
}

func TestCoreModuleWrapper_ModuleStop_NeverStarted(t *testing.T) {
	log := logger.NewMockLog()

	module := &mocks.ICoreModule{}
	module.On("ModuleName").Return("SomeModuleName")

	wrapper := NewCoreModuleWrapper(log, module)

	err := wrapper.ModuleStop(time.Second)
	assert.Contains(t, err.Error(), "module has never been started")
}

func TestCoreModuleWrapper_ModuleStop_AlreadyStopped(t *testing.T) {
	expErr := fmt.Errorf("SomeModuleStopError1")

	module := &mocks.ICoreModule{}
	module.On("ModuleName").Return("SomeModuleName").Once()

	wrapper := createTestModuleWrapper(module)
	wrapper.stopStarted = true
	wrapper.started = true
	close(wrapper.stoppedChan)
	wrapper.stopErr = expErr

	startTime := time.Now()
	err := wrapper.ModuleStop(time.Second)

	assert.Equal(t, expErr, err)
	assert.Less(t, time.Since(startTime), time.Second)
	module.AssertExpectations(t)
}

func TestCoreModuleWrapper_ModuleStop_StopStartedNotFinished(t *testing.T) {
	expErr := fmt.Errorf("SomeModuleStopError1")

	module := &mocks.ICoreModule{}

	wrapper := createTestModuleWrapper(module)
	wrapper.stopStarted = true
	wrapper.started = true
	wrapper.stopErr = expErr

	go func() {
		time.Sleep(500 * time.Millisecond)
		close(wrapper.stoppedChan)
	}()
	startTime := time.Now()
	err := wrapper.ModuleStop(time.Second)

	assert.Equal(t, expErr, err)
	assert.Less(t, time.Since(startTime), time.Second)
	assert.Greater(t, time.Since(startTime), 500*time.Millisecond)
	module.AssertExpectations(t)

}

func TestCoreModuleWrapper_ModuleStop_StopStartedNotFinished_Timeout(t *testing.T) {
	module := &mocks.ICoreModule{}
	module.On("ModuleName").Return("SomeModuleName").Once()

	wrapper := createTestModuleWrapper(module)
	wrapper.stopStarted = true
	wrapper.started = true

	startTime := time.Now()
	err := wrapper.ModuleStop(500 * time.Millisecond)

	assert.Contains(t, err.Error(), "timeout stopping module")
	assert.Less(t, time.Since(startTime), time.Second)
	assert.Greater(t, time.Since(startTime), 500*time.Millisecond)
	module.AssertExpectations(t)
}

func TestCoreModuleWrapper_ModuleStop_StopNotStarted_Timeout(t *testing.T) {
	module := &mocks.ICoreModule{}
	module.On("ModuleName").Return("SomeModuleName").Once()
	module.On("ModuleStop").Return(func() error {
		time.Sleep(time.Second)
		return nil
	}).Once()

	wrapper := createTestModuleWrapper(module)
	wrapper.started = true

	startTime := time.Now()
	err := wrapper.ModuleStop(500 * time.Millisecond)

	assert.Contains(t, err.Error(), "timeout stopping module")
	assert.Less(t, time.Since(startTime), time.Second)
	assert.Greater(t, time.Since(startTime), 500*time.Millisecond)
	module.AssertExpectations(t)
}

func TestCoreModuleWrapper_ModuleStop_StopNotStarted_FinishedWithError(t *testing.T) {
	expErr := fmt.Errorf("SomeStopError")

	module := &mocks.ICoreModule{}
	module.On("ModuleStop").Return(func() error {
		return expErr
	}).Once()

	wrapper := createTestModuleWrapper(module)
	wrapper.started = true

	startTime := time.Now()
	err := wrapper.ModuleStop(time.Second)

	assert.Less(t, time.Since(startTime), 500*time.Millisecond)
	assert.Equal(t, expErr, err)
	module.AssertExpectations(t)
}

func TestCoreModuleWrapper_ModuleStop_StopNotStarted_FinishedWithNoError(t *testing.T) {
	module := &mocks.ICoreModule{}
	module.On("ModuleStop").Return(func() error {
		return nil
	}).Once()

	wrapper := createTestModuleWrapper(module)
	wrapper.started = true

	startTime := time.Now()
	err := wrapper.ModuleStop(time.Second)

	assert.Less(t, time.Since(startTime), 500*time.Millisecond)
	assert.NoError(t, err)
	module.AssertExpectations(t)
}

func TestCoreModuleWrapper_ModuleExectute(t *testing.T) {
	expErr := fmt.Errorf("SomeError")

	module := &mocks.ICoreModule{}
	module.On("ModuleExecute").Return(expErr).Once()

	wrapper := createTestModuleWrapper(module)
	assert.False(t, wrapper.started)

	err := wrapper.ModuleExecute()
	assert.Equal(t, expErr, err)
	assert.True(t, wrapper.started)
	module.AssertExpectations(t)
}

func TestCoreModuleWrapper_ModuleName(t *testing.T) {
	module := &mocks.ICoreModule{}
	module.On("ModuleName").Return("SomeName").Once()

	wrapper := createTestModuleWrapper(module)

	name := wrapper.ModuleName()
	assert.Equal(t, "SomeName", name)
	module.AssertExpectations(t)
}
