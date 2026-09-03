//go:build windows
// +build windows

package main

import (
	"log"
	"runtime/debug"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/log/ssmlog"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const serviceName = "AmazonSSMAgent"
const imageStateComplete = "IMAGE_STATE_COMPLETE"
const imageStateUndeployable = "IMAGE_STATE_UNDEPLOYABLE"
const runningService = 4

// isServiceRunning returns if the service is running
func isServiceRunning(service *mgr.Service) (bool bool, err error) {
	var status svc.Status
	if status, err = service.Query(); err != nil {
		log.Printf("Something went wrong while querying the status - %v", err)
		return false, err
	}
	if status.State == runningService {
		return true, nil
	}
	return false, nil
}

// getServiceStartType returns the current start type of the function
func getServiceStartType(service *mgr.Service) (starttype uint32, err error) {
	var config mgr.Config

	if config, err = service.Config(); err != nil {
		log.Printf("Something went wrong while checking the service start type - %v", err)
		return
	}
	log.Printf("Start type is %v", config.StartType)
	return config.StartType, nil
}

func main() {
	// initialize logger

	// parse input parameters
	parseFlags()
	handleAgentVersionFlag()
	handleToolsFlag()

	log := ssmlog.SSMLogger(true)
	defer log.Close()
	defer log.Flush()

	handleRegistrationAndFingerprintFlags(log)
	log.Info("Starting up windows agent")

	// Check if is running as windows service
	isWinService, err := svc.IsWindowsService()
	if err != nil {
		log.Warnf("Failed to determine if we are running in an interactive session, assuming agent is running as a service: %v", err)
		// Set isWinService to true if there is an error because the agent will most likely be running as a service than not
		isWinService = true
	}

	if isWinService {
		log.Debugf("Is running as windows service, starting svc")
		err = svc.Run(serviceName, &amazonSSMAgentService{log: log})
		if err != nil {
			log.Errorf("SVC Run failed with error: %v", err)
		}
	} else {
		log.Debugf("Not running as windows service")
		run(log)
	}

	log.Info("main function returning")
}

type amazonSSMAgentService struct {
	log logger.T
}

// waitForSysPrep checks if sysPrep is done before starting the agent
func waitForSysPrep(log logger.T) (bool, uint32) {
	// check if sysPrep is done
	ec2ConfigExists := false
	ec2ConfigRunning := false

	var winManager *mgr.Mgr
	var erro error
	var ec2ConfigStartType uint32
	var ec2ConfigService *mgr.Service
	if winManager, erro = mgr.Connect(); erro != nil {
		log.Errorf("Something went wrong while trying to connect to Service Manager - %v", erro)
		return true, appconfig.ErrorExitCode
	}

	ec2ConfigService, erro = winManager.OpenService("EC2Config")
	if erro != nil {
		// If EC2Config does not exist, we do not consider that as an error, but just continue after giving the variables their defaults
		log.Debugf("Opening EC2Config Service failed with error %v", erro)
		ec2ConfigStartType = 0
		ec2ConfigExists = false
	} else {
		ec2ConfigExists = true
		if ec2ConfigRunning, erro = isServiceRunning(ec2ConfigService); erro != nil {
			log.Errorf("Error when trying to find out if service is running")
		}
		if ec2ConfigStartType, erro = getServiceStartType(ec2ConfigService); erro != nil {
			log.Errorf("Error when trying to find the start type")
		}
	}

	if ec2ConfigService != nil {
		ec2ConfigService.Close()
	}
	winManager.Disconnect()

	// setupKey contains the ImageState of windows which will indicate if windows is done with sys prep
	setupKey, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Setup\State`, registry.QUERY_VALUE)
	if err != nil {
		log.Errorf("Error while trying to obtain setupKey: %v", err)
		// In Windows 2003 and below, the path for the setupKey does not exist. Since we are making this corner case fix for domain join,
		// and we know domain join is not supported for Windows 2003 and prior, we send back a success and continue
		return true, appconfig.SuccessExitCode
	}
	defer setupKey.Close()
	log.Debugf("The setup key obtained : %v", setupKey)

	windowsImageState, _, err := setupKey.GetStringValue("ImageState")
	if err != nil {
		log.Errorf("Image state cannot be obtained : %v", err)
		return true, appconfig.ErrorExitCode
	}
	log.Debugf("The state of windows Image is : %v", windowsImageState)

	// disable ssm agent if sysprep is not done and EC2 exists in automatic state or manual state while running
	if windowsImageState != imageStateComplete {
		// IMAGE_STATE_UNDEPLOYABLE is a terminal failure state written by Windows Setup when the
		// image cannot be deployed (e.g. bad unattend.xml, failed specialize, corrupted image).
		// Waiting will not help since Windows will not advance out of this state on its own, and
		// starting the agent against a half-sysprepped host risks registering with an identity
		// that will not survive repair. Fail fast so the service manager surfaces a clear error.
		if windowsImageState == imageStateUndeployable {
			log.Errorf("Windows image state is %s; refusing to start agent", imageStateUndeployable)
			return true, appconfig.ErrorExitCode
		}

		log.Debugf("Does EC2 config exist? %v, EC2 Config Start type: %v, ec2Config is running? %v", ec2ConfigExists, ec2ConfigStartType, ec2ConfigRunning)
		if ec2ConfigExists && ((ec2ConfigStartType == mgr.StartAutomatic) || (ec2ConfigStartType == mgr.StartManual && ec2ConfigRunning)) {
			log.Info("Stopping SSM agent because sysprep hasn't completed")
			return false, appconfig.SuccessExitCode
		}
	}
	// loop around windowsImageState till it reaches IMAGE_STATE_COMPLETE
	for windowsImageState != imageStateComplete {
		log.Infof("Waiting for sysprep to complete. Current Windows ImageState: %v", windowsImageState)
		windowsImageState, _, err = setupKey.GetStringValue("ImageState")
		if err != nil {
			log.Errorf("Image state cannot be obtained : %v", err)
			return true, appconfig.ErrorExitCode
		}
		if windowsImageState == imageStateUndeployable {
			log.Errorf("Windows image state transitioned to %s; refusing to start agent", imageStateUndeployable)
			return true, appconfig.ErrorExitCode
		}
		time.Sleep(5 * time.Second)
	}

	return true, appconfig.SuccessExitCode //return to continue starting the agent
}

// Execute agent as Windows service.  Implement golang.org/x/sys/windows/svc#Handler.
func (a *amazonSSMAgentService) Execute(args []string, r <-chan svc.ChangeRequest, s chan<- svc.Status) (bool, uint32) {
	log := a.log
	defer func() {
		// recover in case the agent panics
		// this should handle some kind of seg fault errors.
		if msg := recover(); msg != nil {
			log.Errorf("core Agent service crashed with message %v!", msg)
			log.Errorf("Stacktrace:\n%s", debug.Stack())
		}
	}()

	log.Info("Waiting for system state")
	isSysPrepEC, erro := waitForSysPrep(log)
	if !(isSysPrepEC && erro == appconfig.SuccessExitCode) { //returnCode true with success exit code means we can continue to start the agent
		log.Warnf("System not ready, not starting agent: isSysPrepEC %v, erro %v", isSysPrepEC, erro)
		// In this case, svcSpecificEC = sysPrepEC and so we will return it
		return isSysPrepEC, erro
	}
	log.Info("System is ready")

	// notify service controller status is now StartPending
	s <- svc.Status{State: svc.StartPending}

	// start service
	log.Info("Starting up agent subsystem")
	agent, contextLog, err := initializeBasicModules(a.log)
	if err != nil {
		contextLog.Errorf("Failed to start agent. %v", err)
		return true, appconfig.ErrorExitCode
	}

	statusChannels := &contracts.StatusComm{
		TerminationChan: make(chan struct{}, 1),
		DoneChan:        make(chan struct{}, 1),
	}

	startCoreAgent(contextLog, agent, statusChannels)
	contextLog.Info("Notifying windows service manager for agent subsystem start")

	// update service status to Running
	//
	// AcceptPreShutdown gets SERVICE_CONTROL_PRESHUTDOWN, which the SCM blocks on instead of racing
	// the short WaitToKillServiceTimeout -- that block is what holds the OS open long enough to make
	// in-flight patch documents reboot-safe. AcceptShutdown covers paths that skip preshutdown.
	//
	// We inherit the OS default for dwPreshutdownTimeout rather than configuring it. That default is
	// version-dependent: Windows Server 2016 and older allow 180s, while 2019 and newer cut it to
	// 10s. Preparation must therefore fit inside 10s, and it does by construction rather than by
	// assumption: the survey deadline is 1s, PrepareShutdown abandons the survey after
	// prepareShutdownTimeout (5s), and the preshutdown path refuses to rebuild a dead channel instead
	// of retrying for 15s. That bound is why we do not call
	// ChangeServiceConfig2(SERVICE_CONFIG_PRESHUTDOWN_INFO) to widen the window, so the agent never
	// rewrites its own service configuration.
	const acceptCmds = svc.AcceptStop | svc.AcceptShutdown | svc.AcceptPreShutdown
	s <- svc.Status{State: svc.Running, Accepts: acceptCmds}
	contextLog.Info("Windows service manager notified that agent service has started")

	var (
		incomingServiceReq         svc.ChangeRequest
		incomingServiceReqChan     = make(chan svc.ChangeRequest, 1)
		terminateIncomingReqThread = make(chan bool, 1)
	)

	go func() {
	incomingReqLoop:
		for {
			select {
			case <-coreAgentStartupErrChan:
				contextLog.Error("Failed to start core agent startup module")
				statusChannels.DoneChan <- struct{}{}
				incomingServiceReqChan <- svc.ChangeRequest{Cmd: svc.Stop}
			case incomingServiceReq = <-r:
				incomingServiceReqChan <- incomingServiceReq
			case <-terminateIncomingReqThread:
				contextLog.Info("Incoming service request go routine ended")
				break incomingReqLoop
			}
		}
	}()

loop:
	// using an infinite loop to wait for ChangeRequests
	for {
		// block and wait for ChangeRequests
		c := <-incomingServiceReqChan

		// handle ChangeRequest, svc.Pause is not supported
		switch c.Cmd {
		case svc.Interrogate:
			contextLog.Debug("Service received interrogate ChangeRequest")
			s <- c.CurrentStatus
			// Testing deadlock from https://code.google.com/p/winsvc/issues/detail?id=4
			time.Sleep(100 * time.Millisecond)
			s <- c.CurrentStatus
		case svc.Stop:
			statusChannels.TerminationChan <- struct{}{}
			contextLog.Info("Service received stop ChangeRequest")
			break loop
		// PreShutdown and Shutdown are ALTERNATIVES: accepting SERVICE_ACCEPT_PRESHUTDOWN means the
		// SCM sends PRESHUTDOWN *instead of* SHUTDOWN on an orderly shutdown. Both prepare and both
		// break the loop, so preparation runs exactly once either way.
		case svc.PreShutdown:
			// The SCM waits here, so every agent process is still alive. Use that window to make
			// in-flight patch documents reboot-safe: a servicing reboot that kills a patch mid-run
			// otherwise surfaces as a terminal Failed while patching is still progressing. This
			// blocks the service-control goroutine, so PrepareShutdown is internally bounded (see
			// prepareShutdownTimeout) -- a worker that never answers cannot stall the shutdown.
			agent.PrepareShutdown()
			statusChannels.TerminationChan <- struct{}{}
			contextLog.Info("Service received preshutdown ChangeRequest")
			break loop
		case svc.Shutdown:
			// Only when preshutdown was not delivered (e.g. an abrupt shutdown). The SCM waits just
			// WaitToKillServiceTimeout here, so preparation is best-effort.
			agent.PrepareShutdown()
			statusChannels.TerminationChan <- struct{}{}
			contextLog.Info("Service received shutdown ChangeRequest")
			break loop
		default:
			continue loop
		}
	}
	s <- svc.Status{State: svc.StopPending}
	terminateIncomingReqThread <- true
	<-statusChannels.DoneChan
	agent.Stop()
	return false, appconfig.SuccessExitCode
}
