package registrar

import (
	"math"
	"math/rand"
	"runtime/debug"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/identity"
	"github.com/aws/amazon-ssm-agent/core/app/context"
)

func getBackoffRetryJitterSleepDuration(retryCount int) time.Duration {
	expBackoff := math.Pow(2, float64(retryCount))
	return time.Duration(int(expBackoff)+rand.Intn(int(math.Ceil(expBackoff*0.2)))) * time.Second
}

type IRetryableRegistrar interface {
	Start() error
	Stop()
}

type RetryableRegistrar struct {
	log                       log.T
	registrationAttemptedChan chan struct{}
	stopRegistrarChan         chan struct{}
	identityRegistrar         identity.Registrar
	timeAfterFunc             func(time.Duration) <-chan time.Time
	isRegistrarRunning        bool
}

func NewRetryableRegistrar(context context.ICoreAgentContext) *RetryableRegistrar {
	log := context.Log().WithContext("[Registrar]")
	log.Debug("initializing registrar")
	// Cast to innerIdentityGetter interface that defined getInner
	innerGetter, ok := context.Identity().(identity.IInnerIdentityGetter)
	if !ok {
		log.Errorf("malformed identity")
		return nil
	}

	var identityRegistrar identity.Registrar
	if identityRegistrar, ok = innerGetter.GetInner().(identity.Registrar); !ok {
		log.Debug("identity does not leverage auto-registration")
		return nil
	}

	return &RetryableRegistrar{
		log:                       log,
		identityRegistrar:         identityRegistrar,
		registrationAttemptedChan: make(chan struct{}, 1),
		stopRegistrarChan:         make(chan struct{}),
		timeAfterFunc:             time.After,
	}
}

func (r *RetryableRegistrar) Start() error {
	r.log.Info("Starting registrar module")
	go r.RegisterWithRetry()
	r.isRegistrarRunning = true
	// Block until registration attempted at least once
	<-r.registrationAttemptedChan
	return nil
}

func (r *RetryableRegistrar) RegisterWithRetry() {
	defer func() {
		if err := recover(); err != nil {
			r.log.Errorf("registrar panic: %v", err)
			r.log.Errorf("Stacktrace:\n%s", debug.Stack())
			r.log.Flush()
			r.isRegistrarRunning = false
			select {
			case <-r.registrationAttemptedChan:
				//channel open, write to channel to unblock and close
				r.registrationAttemptedChan <- struct{}{}
				close(r.registrationAttemptedChan)
			default:
			}
		}
	}()

	retryCount := 0
	for {
		err := r.identityRegistrar.Register()
		if retryCount == 0 {
			r.registrationAttemptedChan <- struct{}{}
			close(r.registrationAttemptedChan)
		}

		if err == nil {
			r.isRegistrarRunning = false
			return
		}

		r.log.Errorf("failed to register identity: %v", err)
		// Default sleep duration for non-aws errors
		sleepDuration := getBackoffRetryJitterSleepDuration(retryCount)
		// Max retry count is 16, which will sleep for about 18-22 hours
		if retryCount < 16 {
			retryCount++
		}

		r.log.Infof("sleeping for %v minutes before retrying registration", sleepDuration.Minutes())

		select {
		case <-r.stopRegistrarChan:
			r.log.Info("Stopping registrar")
			r.isRegistrarRunning = false
			r.log.Flush()
			return
		case <-r.timeAfterFunc(sleepDuration):
		}
	}
}

func (r *RetryableRegistrar) Stop() {
	if !r.isRegistrarRunning {
		r.log.Info("Registrar is already stopped")
		r.log.Flush()
		return
	}

	r.log.Info("Sending signal to stop registrar")
	r.stopRegistrarChan <- struct{}{}
}
