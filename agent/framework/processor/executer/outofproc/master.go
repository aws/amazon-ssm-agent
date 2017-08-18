package outofproc

import (
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/basicexecuter"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/channel"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/proc"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

type Backend messageContracts.MessagingBackend

const (
	defaultDrainChannelTimeout = 10 * time.Second
)
const (
	stopTypeTerminate = 1
	stopTypeShutdown  = 2
)

type OutOfProcExecuter struct {
	basicexecuter.BasicExecuter
	documentID     string
	procController proc.ProcessController
	ctx            context.T
}

var channelCreator channel.ChannelCreator

//find the ipc channel identified by the document ID
var channelDiscoverer = func(documentID string) (string, bool) {
	return channel.FindChannel(documentID)
}

func NewOutOfProcExecuter(ctx context.T) *OutOfProcExecuter {
	return &OutOfProcExecuter{
		BasicExecuter:  basicexecuter.NewBasicExecuter(ctx),
		ctx:            ctx.With("[OutOfProcExecuter]"),
		procController: proc.NewOSProcess(ctx),
	}
}

//TODO may need to change all info logs to debug once this feature is released
//Run() prepare the ipc channel, create a data processing backend and start messaging with docment worker
func (e *OutOfProcExecuter) Run(
	cancelFlag task.CancelFlag,
	docStore executer.DocumentStore) chan contracts.DocumentResult {
	docState := docStore.Load()

	documentID := docState.DocumentInformation.DocumentID
	e.documentID = documentID
	//create reply channel
	resChan := make(chan contracts.DocumentResult, len(docState.InstancePluginsInformation)+1)

	//update context with the document id
	e.ctx = e.ctx.With("[" + documentID + "]")

	log := e.ctx.Log()
	//start prepare messaging
	//if anything fails during the prep stage, use in-proc Runner
	if ipc, err := e.prepare(); err != nil {
		log.Errorf("failed to prepare ipc, document run failed")
		e.procController.Kill()
		return e.BasicExecuter.Run(cancelFlag, docStore)
	} else {
		log.Info("launching messaging worker")
		//handoff reply functionalities to data backend.
		backend, stopChan := newExecuterBackend(resChan, docStore, cancelFlag)
		go func() {
			//handoff the data backend to messaging worker
			if err := Messaging(log, ipc, backend, stopChan); err != nil {
				//TODO if this fails in the middle, command lost in InProgress state
				//the messaging worker encountered error, either ipc run into error or data backend throws error
				log.Errorf("messaging worker encountered error: %v", err)
				//close channel
				ipc.Close()
				//try to kill the child-process
				e.procController.Kill()
			}
		}()
	}

	return resChan
}

//TODO add process discoverer
//prepare the channel for messaging as well as launching the document worker process, if the channel already exists, re-open it.
func (e *OutOfProcExecuter) prepare() (ipc channel.Channel, err error) {
	log := e.ctx.Log()
	//first, do channel discovery, if channel not found, create new sub-process
	documentID := e.documentID
	channelName, found := channelDiscoverer(documentID)
	//if channel not exists, create new channel handle and new sub process
	if !found {
		channelName = documentID
		log.Debug("channel not found, starting a new process...")
		var pid int
		var processName = appconfig.DefaultDocumentWorker
		if pid, err = e.procController.StartProcess(processName, []string{string(channelName)}); err != nil {
			log.Errorf("start process: %v error: %v", processName, err)
			return
		} else {
			log.Infof("successfully launched new process: %v", pid)
		}
		//TODO add pid and process creation time to persistence layer
		//release the attached process resource
		e.procController.Release()
	}
	//server create channel, ready to connect
	ipc = channelCreator(channel.ModeMaster)
	//At server mode, connect() operation is listening for client connections
	if err = ipc.Open(channelName); err != nil {
		log.Error("Not able to connect to IPC channel")
		return
	}
	return
}

//TODO implement channel drain timer and command timeout
// Messaging implements the duplex transmission between master and worker, it send datagram it received to data backend,
// close the ipc channel once messaging is done.
func Messaging(log log.T, ipc channel.Channel, backend Backend, stopChan chan int) (err error) {

	defer func() {
		if msg := recover(); msg != nil {
			log.Errorf("Executer listener panic: %v", msg)
		}
		//make sure to close the outbound channel when return in order to signal Processor
		backend.Close()
	}()

	onMessageChannel := ipc.GetMessageChannel()
	for {
		select {
		case signal := <-stopChan:
			//soft stop, do not close the channel
			if signal == stopTypeShutdown {
				log.Info("requested shutdown, ipc messaging stopped")
				//do not close channel since server.close() will destroy the channel object
				//make sure the process handle is properly release
				return
			} else if signal == stopTypeTerminate {
				//hard stop, close the channel
				log.Info("requested terminate messaging worker, closing the channel")
				ipc.Close()
				return
			} else {
				log.Errorf("unrecognized stop type: %v", signal)
				//return?
			}
		case datagram := <-backend.Accept():
			log.Debugf("sending datagram: %v", datagram)
			if err = ipc.Send(datagram); err != nil {
				log.Errorf("failed to send message to ipc channel: %v", err)
				return
			}
		case datagram, more := <-onMessageChannel:
			log.Debugf("received datagram: %v", datagram)
			if !more {
				log.Info("channel closed, stop messaging worker")
				return
			}
			if err = backend.Process(datagram); err != nil {
				log.Errorf("messaging pipeline process datagram encountered error: %v", err)
				return
			}

		}
	}
}
