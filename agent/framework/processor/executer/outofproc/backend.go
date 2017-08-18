package outofproc

import (
	"errors"
	"time"

	"sync"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/contracts"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

const (
	defaultWorkerWaitTime = 100 * time.Second
	//make sure the channel operation in backend is not blocked
	defaultBackendChannelSize = 10
)

type PluginRunner func(
	context context.T,
	plugins []model.PluginState,
	resChan chan contracts.PluginResult,
	cancelFlag task.CancelFlag,
)

type WorkerBackend struct {
	ctx        context.T
	input      chan string
	once       sync.Once
	cancelFlag task.CancelFlag
	runner     PluginRunner
	stopChan   chan int
}

type ExecuterBackend struct {
	docState *model.DocumentState
	input    chan string
	docStore executer.DocumentStore
	output   chan contracts.DocumentResult
	stopChan chan int
}

//TODO handle error
func newExecuterBackend(output chan contracts.DocumentResult, docStore executer.DocumentStore, cancelFlag task.CancelFlag) (*ExecuterBackend, chan int) {
	stopChan := make(chan int, defaultBackendChannelSize)
	inputChan := make(chan string, defaultBackendChannelSize)
	docState := docStore.Load()
	go func(pluginConfigs []model.PluginState) {
		startDatagram, _ := messageContracts.CreateDatagram(messageContracts.MessageTypePluginConfig, pluginConfigs)
		inputChan <- startDatagram
		cancelFlag.Wait()
		if cancelFlag.Canceled() {
			cancelDatagram, _ := messageContracts.CreateDatagram(messageContracts.MessageTypeCancel, "cancel")
			inputChan <- cancelDatagram
		} else if cancelFlag.ShutDown() {
			stopChan <- stopTypeShutdown
		}
		//cancel state is complete, safe return

	}(docState.InstancePluginsInformation)
	return &ExecuterBackend{
		output:   output,
		docState: &docState,
		docStore: docStore,
		input:    inputChan,
		stopChan: stopChan,
	}, stopChan
}

func (p *ExecuterBackend) Accept() <-chan string {
	return p.input
}

//TODO handle error and logging
//TODO version handling?
func (p *ExecuterBackend) Process(datagram string) error {
	t, content := messageContracts.ParseDatagram(datagram)
	switch t {
	case messageContracts.MessageTypeReply, messageContracts.MessageTypeComplete:
		var docResult contracts.DocumentResult
		jsonutil.Unmarshal(content, &docResult)
		p.formatDocResult(&docResult)
		p.output <- docResult
		if t == messageContracts.MessageTypeComplete {
			//signal the caller that messaging should stop
			p.stopChan <- stopTypeTerminate
		}
	default:
		return errors.New("unsupported message type")
	}
	return nil
}

func (p *ExecuterBackend) Close() {
	//save the docStore object
	p.docStore.Save(*p.docState)
	//make sure the executer output channel is closed
	close(p.output)
}

func (p *ExecuterBackend) formatDocResult(docResult *contracts.DocumentResult) {
	//fill doc level information that the sub-process wouldn't know
	docResult.MessageID = p.docState.DocumentInformation.MessageID
	docResult.AssociationID = p.docState.DocumentInformation.AssociationID
	docResult.DocumentName = p.docState.DocumentInformation.DocumentName
	docResult.NPlugins = len(p.docState.InstancePluginsInformation)
	docResult.DocumentVersion = p.docState.DocumentInformation.DocumentVersion
	//update current document status
	p.docState.DocumentInformation.DocumentStatus = docResult.Status
}

func NewWorkerBackend(ctx context.T, runner PluginRunner) (*WorkerBackend, chan int) {
	stopChan := make(chan int)
	return &WorkerBackend{
		ctx:        ctx,
		input:      make(chan string),
		cancelFlag: task.NewChanneledCancelFlag(),
		runner:     runner,
		stopChan:   stopChan,
	}, stopChan
}

//TODO handle error and log
func (p *WorkerBackend) Process(datagram string) error {
	t, content := messageContracts.ParseDatagram(datagram)
	switch t {
	case messageContracts.MessageTypePluginConfig:
		var plugins []model.PluginState
		jsonutil.Unmarshal(content, &plugins)
		p.once.Do(func() {
			statusChan := make(chan contracts.PluginResult)
			go p.runner(p.ctx, plugins, statusChan, p.cancelFlag)
			go p.pluginListener(statusChan)
		})

	case messageContracts.MessageTypeCancel:
		p.cancelFlag.Set(task.Canceled)
	default:
		return errors.New("unsupported message type")
	}
	return nil
}

func (p *WorkerBackend) pluginListener(statusChan chan contracts.PluginResult) {
	log := p.ctx.Log()
	results := make(map[string]*contracts.PluginResult)
	for res := range statusChan {
		results[res.PluginName] = &res
		//TODO move the aggregator under executer package and protect it
		status, _, _ := docmanager.DocumentResultAggregator(log, res.PluginName, results)
		docResult := contracts.DocumentResult{
			Status:        status,
			PluginResults: results,
			LastPlugin:    res.PluginName,
		}
		replyMessage, _ := messageContracts.CreateDatagram(messageContracts.MessageTypeReply, docResult)
		log.Info("sending reply message...")
		p.input <- replyMessage
	}

	log.Info("plugins execution complete, send document complete response...")
	status, _, _ := docmanager.DocumentResultAggregator(log, "", results)
	docResult := contracts.DocumentResult{
		Status:        status,
		PluginResults: results,
		LastPlugin:    "",
	}
	completeMessage, _ := messageContracts.CreateDatagram(messageContracts.MessageTypeComplete, docResult)
	log.Info("sending complete message...")
	p.input <- completeMessage
	//sending stop signal
	p.stopChan <- stopTypeTerminate

}

func (p *WorkerBackend) Accept() <-chan string {
	return p.input
}

func (p *WorkerBackend) Close() {
	//close the cancelFlag in case there're listeners down the call stack
	p.cancelFlag.Set(task.Completed)
}
