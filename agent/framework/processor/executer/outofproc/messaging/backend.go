package messaging

import (
	"errors"
	"runtime/debug"

	"sync"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

const (
	//make sure the channel operation in backend is not blocked
	defaultBackendChannelSize = 10
)

type PluginRunner func(
	context context.T,
	docState contracts.DocumentState,
	resChan chan contracts.PluginResult,
	cancelFlag task.CancelFlag,
)

//worker backend receives request messages from master, controls a pluginRunner based off the request and send reponses to Executer
type WorkerBackend struct {
	ctx        context.T
	input      chan string
	once       sync.Once
	cancelFlag task.CancelFlag
	runner     PluginRunner
	stopChan   chan int
}

//Executer backend formulate the run request to the worker, and collect back the responses from worker
type ExecuterBackend struct {
	//the shared state object that Executer hand off to data backend
	docState   *contracts.DocumentState
	input      chan string
	cancelFlag task.CancelFlag
	output     chan contracts.DocumentResult
	stopChan   chan int
}

func NewExecuterBackend(log log.T, output chan contracts.DocumentResult, docState *contracts.DocumentState, cancelFlag task.CancelFlag) *ExecuterBackend {
	stopChan := make(chan int, defaultBackendChannelSize)
	inputChan := make(chan string, defaultBackendChannelSize)
	p := ExecuterBackend{
		output:     output,
		docState:   docState,
		input:      inputChan,
		cancelFlag: cancelFlag,
		stopChan:   stopChan,
	}
	go p.start(log, *docState)
	return &p
}

func (p *ExecuterBackend) start(log log.T, docState contracts.DocumentState) {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Executer backend start panic: \n%v", r)
			log.Errorf("Stacktrace:\n%s", debug.Stack())
		}
	}()
	startDatagram, _ := CreateDatagram(MessageTypePluginConfig, docState)
	p.input <- startDatagram
	p.cancelFlag.Wait()
	if p.cancelFlag.Canceled() {
		cancelDatagram, _ := CreateDatagram(MessageTypeCancel, "cancel")
		p.input <- cancelDatagram
	} else if p.cancelFlag.ShutDown() {
		p.stopChan <- stopTypeShutdown
	}
	//cancel state is complete, safe return
	close(p.input)
}

func (p *ExecuterBackend) Accept() <-chan string {
	return p.input
}

func (p *ExecuterBackend) Stop() <-chan int {
	return p.stopChan
}

func (p *ExecuterBackend) Close() {
	p.input = nil
}

func (p *ExecuterBackend) CloseStop() {
	return
}

//TODO handle error and logging, when err, ask messaging to stop
//TODO version handling?
func (p *ExecuterBackend) Process(datagram string) error {
	t, content := ParseDatagram(datagram)
	switch t {
	case MessageTypeReply, MessageTypeComplete:
		var docResult contracts.DocumentResult
		jsonutil.Unmarshal(content, &docResult)
		p.formatDocResult(&docResult)
		p.output <- docResult
		if t == MessageTypeComplete {
			//get document result, force termniate messaging worker
			p.stopChan <- stopTypeTerminate
		}
	default:
		return errors.New("unsupported message type")
	}
	return nil
}

func (p *ExecuterBackend) formatDocResult(docResult *contracts.DocumentResult) {
	//fill doc level information that the sub-process wouldn't know
	docResult.MessageID = p.docState.DocumentInformation.MessageID
	docResult.AssociationID = p.docState.DocumentInformation.AssociationID
	docResult.DocumentName = p.docState.DocumentInformation.DocumentName
	docResult.NPlugins = len(p.docState.InstancePluginsInformation)
	docResult.DocumentVersion = p.docState.DocumentInformation.DocumentVersion
	//update current document status
	contracts.UpdateDocState(docResult, p.docState)
}

func NewWorkerBackend(ctx context.T, runner PluginRunner) *WorkerBackend {
	stopChan := make(chan int)
	return &WorkerBackend{
		ctx:        ctx.With("[DataBackend]"),
		input:      make(chan string),
		cancelFlag: task.NewChanneledCancelFlag(),
		runner:     runner,
		stopChan:   stopChan,
	}
}

func (p *WorkerBackend) Process(datagram string) error {
	t, content := ParseDatagram(datagram)
	log := p.ctx.Log()
	switch t {
	case MessageTypePluginConfig:
		log.Info("received plugin config message")
		var docState contracts.DocumentState
		log.Info(content)
		if err := jsonutil.Unmarshal(content, &docState); err != nil {
			log.Errorf("failed to unmarshal plugin config: %v", err)
			//TODO request messaging to stop
			return err
		}
		log.Debugf("unmarshal plugin config: %+v", docState)
		p.once.Do(func() {
			statusChan := make(chan contracts.PluginResult)
			go p.runner(p.ctx, docState, statusChan, p.cancelFlag)
			go p.pluginListener(statusChan)
		})

	case MessageTypeCancel:
		log.Info("requested cancel the command, setting cancel flag...")
		p.cancelFlag.Set(task.Canceled)
	default:
		//TODO add extra logic to check whether plugin has started, if not, stop IPC, or add timeout
		return errors.New("unsupported message type")
	}
	return nil
}

func (p *WorkerBackend) pluginListener(statusChan chan contracts.PluginResult) {
	log := p.ctx.Log()
	results := make(map[string]*contracts.PluginResult)
	var finalStatus contracts.ResultStatus
	defer func() {
		//if this routine panics, return failed results
		if msg := recover(); msg != nil {
			log.Errorf("plugin listener panic: %v", msg)
			log.Errorf("Stacktrace:\n%s", debug.Stack())
			finalStatus = contracts.ResultStatusFailed
		}

		docResult := contracts.DocumentResult{
			Status:        finalStatus,
			PluginResults: results,
			LastPlugin:    "",
		}
		log.Info("sending document complete response...")
		completeMessage, _ := CreateDatagram(MessageTypeComplete, docResult)
		p.input <- completeMessage
		close(p.input)
		log.Info("stopping ipc worker...")
		//sending stop signal
		p.stopChan <- stopTypeShutdown
		close(p.stopChan)
	}()

	for res := range statusChan {
		var result = res
		results[res.PluginID] = &result
		//TODO move the aggregator under executer package and protect it, there's global lock in this package
		status, _, _ := contracts.DocumentResultAggregator(log, res.PluginID, results)
		docResult := contracts.DocumentResult{
			Status:        status,
			PluginResults: results,
			LastPlugin:    res.PluginID,
		}
		replyMessage, _ := CreateDatagram(MessageTypeReply, docResult)
		log.Debugf("plugin: %v done, sending reply message...", res.PluginID)
		p.input <- replyMessage
	}
	log.Info("document execution complete")
	finalStatus, _, _ = contracts.DocumentResultAggregator(log, "", results)

}

func (p *WorkerBackend) Accept() <-chan string {
	return p.input
}

func (p *WorkerBackend) Stop() <-chan int {
	return p.stopChan
}

func (p *WorkerBackend) Close() {
	p.input = nil
}

func (p *WorkerBackend) CloseStop() {
	p.stopChan = nil
}
