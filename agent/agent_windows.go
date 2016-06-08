// +build windows

package main

import (
	"log"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"golang.org/x/sys/windows/svc"
)

const serviceName = "AmazonSSMAgent"

func main() {
	// initialize logger
	log := logger.Logger()
	defer log.Close()
	defer log.Flush()

	// parse input parameters
	parseFlags(log)

	// check whether this is an interactive session
	isIntSess, err := svc.IsAnInteractiveSession()
	if err != nil {
		log.Warnf("Failed to determine if we are running in an interactive session: %v", err)
	}

	// isIntSess is false by default (after declaration), this fits the use
	// case that agent is running as Windows service most of times
	switch isIntSess {
	case true:
		run(log)
	case false:
		svc.Run(serviceName, &amazonSSMAgentService{log: log})
	}
}

type amazonSSMAgentService struct {
	log logger.T
}

// Execute agent as Windows service.  Implement golang.org/x/sys/windows/svc#Handler.
func (a *amazonSSMAgentService) Execute(args []string, r <-chan svc.ChangeRequest, s chan<- svc.Status) (bool, uint32) {

	// notify service controller status is now StartPending
	s <- svc.Status{State: svc.StartPending}

	// start service, without specifying instance id or region
	var emptyString string
	cpm, err := start(a.log, &emptyString, &emptyString)
	if err != nil {
		log.Printf("Failed to start agent. %v", err)
		return true, appconfig.ErrorExitCode
	}

	// update service status to Running
	const acceptCmds = svc.AcceptStop | svc.AcceptShutdown
	s <- svc.Status{State: svc.Running, Accepts: acceptCmds}
loop:
	// using an infinite loop to wait for ChangeRequests
	for {
		// block and wait for ChangeRequests
		c := <-r

		// handle ChangeRequest, svc.Pause is not supported
		switch c.Cmd {
		case svc.Interrogate:
			s <- c.CurrentStatus
			// Testing deadlock from https://code.google.com/p/winsvc/issues/detail?id=4
			time.Sleep(100 * time.Millisecond)
			s <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			break loop
		default:
			continue loop
		}
	}
	s <- svc.Status{State: svc.StopPending}
	stop(a.log, cpm)
	return false, appconfig.SuccessExitCode
}
