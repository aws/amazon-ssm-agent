package coremodules

import (
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/log"
)

type CoreModuleWrapper struct {
	module      contracts.ICoreModule
	log         log.T
	mtx         *sync.Mutex
	stoppedChan chan struct{}

	started     bool
	stopStarted bool
	stopErr     error
}

func (c *CoreModuleWrapper) stop() {
	defer func() {
		if err := recover(); err != nil {
			c.log.Errorf("stop on %s panic with error: %v", c.module.ModuleName(), err)
			c.log.Errorf("Stacktrace:\n%s", debug.Stack())
		}
	}()
	c.stopErr = c.module.ModuleStop()
	close(c.stoppedChan)
}

// ModuleStop tries to stop module, call is blocking until either module is stopped or until
func (c *CoreModuleWrapper) ModuleStop(waitTime time.Duration) (err error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	defer func() {
		if r := recover(); r != nil {
			c.log.Errorf("moduleStop on %s panic with error: %v", c.module.ModuleName(), r)
			c.log.Errorf("Stacktrace:\n%s", debug.Stack())
			err = fmt.Errorf("%v", r)
		}
	}()

	// No need to stop if module never started
	if !c.started {
		return fmt.Errorf("cant stop module %s, module has never been started", c.module.ModuleName())
	}

	// If we have not started the stop processes, attempt to stop the module
	if !c.stopStarted {
		c.stopStarted = true
		go c.stop()
	} else {
		// Check if module has already been stopped
		select {
		case <-c.stoppedChan:
			c.log.Debugf("Module %s already stopped", c.module.ModuleName())
			return c.stopErr
		default:
		}
	}

	// Module is still in stop process, wait for module to stop or timeout
	select {
	case <-c.stoppedChan:
		// module has been stopped
		return c.stopErr
	case <-time.After(waitTime):
		// module stop timed out
		return fmt.Errorf("timeout stopping module %s", c.module.ModuleName())
	}
}

func (c *CoreModuleWrapper) ModuleName() string {
	return c.module.ModuleName()
}

func (c *CoreModuleWrapper) ModuleExecute() error {
	c.started = true
	return c.module.ModuleExecute()
}

func NewCoreModuleWrapper(log log.T, module contracts.ICoreModule) contracts.ICoreModuleWrapper {
	return &CoreModuleWrapper{
		module:      module,
		log:         log.WithContext("CoreModuleWrapper"),
		mtx:         &sync.Mutex{},
		started:     false,
		stopStarted: false,
		stoppedChan: make(chan struct{}),
		stopErr:     nil,
	}
}
