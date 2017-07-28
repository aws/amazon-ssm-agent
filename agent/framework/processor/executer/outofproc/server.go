package outofproc

import (
	"time"

	"strings"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/channel"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/proc"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

const (
	defaultServerTimeout = 3600 * time.Second
)
const (
	stopTypeCancel       = 1
	stopTypeShutdown     = 2
	controlTypeCancelled = "cancel"
)

type OutOfProcExecuter struct {
	resChan        chan contracts.DocumentResult
	docState       *model.DocumentState
	docStore       executer.DocumentStore
	cancelFlag     task.CancelFlag
	procController proc.ProcessController
	ctx            context.T
}

type DocumentControl struct {
	Type string `json:"type"`
}

var channelCreator channel.ChannelCreator
var channelDiscoverer = func(documentID string) (string, bool) {
	return documentID + "-" + "1.0", false
}

func createChannelHandle(documentID, version string) string {
	return documentID + "_" + version
}
func parseChannelHandle(handle string) (string, string) {
	res := strings.Split(handle, "_")
	return res[0], res[1]
}

func isClientCancelled(ctrl DocumentControl) bool {
	return ctrl.Type == controlTypeCancelled
}

func createProcessName(documentID string) string {
	return "amazon-ssm-command-" + documentID
}

func NewOutOfProcExecuter(ctx context.T) *OutOfProcExecuter {
	return &OutOfProcExecuter{
		ctx: ctx.With("[OutOfProcExecuter]"),
	}
}

func (e *OutOfProcExecuter) Run(
	cancelFlag task.CancelFlag,
	docStore executer.DocumentStore) chan contracts.DocumentResult {

	//parse docInfo
	e.docStore = docStore
	e.docState = new(model.DocumentState)
	*e.docState = docStore.Load()
	pluginConfigs := e.docState.InstancePluginsInformation

	//create reply channel
	resChan := make(chan contracts.DocumentResult, len(e.docState.InstancePluginsInformation))
	e.cancelFlag = cancelFlag
	e.resChan = resChan

	//update context with the document id
	e.ctx = e.ctx.With("[" + e.docState.DocumentInformation.DocumentID + "]")
	e.docState.DocumentInformation.RunCount++
	//create named handle

	go e.server(pluginConfigs, resChan)
	return resChan
}

func (e *OutOfProcExecuter) prepare() (ipc channel.Channel, serverVersion string, err error) {
	log := e.ctx.Log()
	//first, do channel discovery, if channel not found, create new sub-process
	documentID := e.docState.DocumentInformation.DocumentID
	handle, found := channelDiscoverer(e.docState.DocumentInformation.DocumentID)
	//if channel already exists, try to parse the version string and adapt to client's version
	//TODO Client should adapt to server's version as well for downgrade scenarios
	serverVersion = messageContracts.GetLastestVersion()
	if found {
		_, clientVersion := parseChannelHandle(handle)
		//lexicographical comparison
		//TODO extract this to a function in messageContract
		if clientVersion < messageContracts.GetLastestVersion() {
			serverVersion = clientVersion
		}
	} else {
		handle = createChannelHandle(e.docState.DocumentInformation.DocumentID, serverVersion)
	}
	//server create channel, ready to connect
	ipc, err = channelCreator(handle, channel.ModeServer)
	log.Debugf("creating channel: %v", handle)
	if err != nil {
		log.Errorf("failed to create connection object: %v", handle)
		return
	}
	//if connection object doesn't exist, means client not started yet, launch new client
	var procName string
	var pid = -1
	if !found {
		log.Debug("starting a new process...")
		procName = createProcessName(documentID)
		if pid, err = e.procController.StartProcess(procName, []string{string(handle)}); err != nil {
			log.Errorf("start process: %v error: %v", procName, err)
			return
		} else {
			log.Infof("successfully launched new process %v|%v", procName, pid)
		}
	}
	//At server mode, connect() operation is listening for client connections
	if err = ipc.Connect(); err != nil {
		log.Error("Not able to connect to IPC object")
		return
	}
	return
}

//TODO may need to change all info logs to debug once this feature is released
//server implement the communication protocol between client <--> server
func (e *OutOfProcExecuter) server(pluginConfigs []model.PluginState, outbound chan contracts.DocumentResult) {
	log := e.ctx.Log()
	defer func() {
		if msg := recover(); msg != nil {
			log.Errorf("Executer listener panic: %v", msg)
		}
		//save the document state
		//TODO must refactor Save() to single argument input
		e.docStore.Save(*e.docState)
		//make sure to close the outbound channel when return in order to signal Processor
		close(outbound)
	}()
	ipc, serverVersion, err := e.prepare()
	if err != nil {
		log.Error("failed to prepare channel")
		return
	}
	//use the latest version
	startMessage, err := messageContracts.Marshal(serverVersion, messageContracts.MessageTypeStart, pluginConfigs)
	if err != nil {
		log.Errorf("failed to marshal start message: %v", err)
		//kill the launched sub-process
		e.procController.Kill()
		//close channel and exit
		ipc.Close()
		return
	}
	log.Debug("Sending start message...")
	ipc.Send(startMessage)
	onMessageChannel := ipc.GetMessageChannel()
	//listen for cancel and shutdown
	stopChan := make(chan int)
	go func(cancelFlag task.CancelFlag) {
		cancelFlag.Wait()
		if cancelFlag.Canceled() {
			stopChan <- stopTypeCancel
		} else if cancelFlag.ShutDown() {
			stopChan <- stopTypeShutdown
		}

	}(e.cancelFlag)
	for {
		select {
		case signal := <-stopChan:
			//TODO in future we can extend to have "detach" message sent to client
			if signal == stopTypeShutdown {
				log.Infof("requested shutdown, releasing subprocess resources...")
				e.procController.Release()
				//do not close channel since server.close() will destroy the channel object
				return
			} else if signal == stopTypeCancel {
				log.Info("requested cancel, sending cancel message to sub process...")
				docCtrl := DocumentControl{Type: controlTypeCancelled}
				cancelMessage, err := messageContracts.Marshal(serverVersion, messageContracts.MessageTypeControl, docCtrl)
				if err != nil {
					log.Errorf("failed to create cancel message: %v", err)
					//lose control of the child, kill it
					//TODO if this happens at restart, very bad -- need to bookkeep the pid and kill
					//TODO based off pid, however, if the sub-process already terminate, you'll risk killing other
					//TODO since agent has root prioirty
					e.procController.Kill()
					ipc.Close()
					return
				}
				ipc.Send(cancelMessage)
				//do not return to wait for client acknowledge close
			}
		case message := <-onMessageChannel:
			if message.Type == messageContracts.MessageTypeClose {
				log.Info("client closed connection, server closing...")
				ipc.Close()
				return
			}
			var docResult contracts.DocumentResult
			err := messageContracts.UnMarshal(message, &docResult)
			if err != nil {
				log.Errorf("failed to unmarshal client payload: %v", err)
				//TODO same problem above
				e.procController.Kill()
				//close channel
				ipc.Close()
			}
			e.formatResult(&docResult)
			outbound <- docResult

		}
	}
}

func (e *OutOfProcExecuter) formatResult(docResult *contracts.DocumentResult) {
	//fill doc level information that the sub-process wouldn't know
	docResult.MessageID = e.docState.DocumentInformation.MessageID
	docResult.AssociationID = e.docState.DocumentInformation.AssociationID
	docResult.DocumentName = e.docState.DocumentInformation.DocumentName
	docResult.NPlugins = len(e.docState.InstancePluginsInformation)
	docResult.DocumentVersion = e.docState.DocumentInformation.DocumentVersion
	//update current document status
	e.docState.DocumentInformation.DocumentStatus = docResult.Status
}
