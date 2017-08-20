package messaging

import (
	"errors"
	"time"

	"sync"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

const (
	defaultWorkerWaitTime = 100 * time.Second
	//make sure the channel operation in backend is not blocked
	defaultBackendChannelSize = 10
)

//TODO this should be moved to a common package
type PluginRunner func(
	context context.T,
	plugins []model.PluginState,
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
	docState   *model.DocumentState
	input      chan string
	cancelFlag task.CancelFlag
	output     chan contracts.DocumentResult
	stopChan   chan int
}

//TODO handle error
func NewExecuterBackend(output chan contracts.DocumentResult, docState *model.DocumentState, cancelFlag task.CancelFlag) *ExecuterBackend {
	stopChan := make(chan int, defaultBackendChannelSize)
	inputChan := make(chan string, defaultBackendChannelSize)
	go func(pluginConfigs []model.PluginState) {
		startDatagram, _ := CreateDatagram(MessageTypePluginConfig, pluginConfigs)
		inputChan <- startDatagram
		cancelFlag.Wait()
		if cancelFlag.Canceled() {
			cancelDatagram, _ := CreateDatagram(MessageTypeCancel, "cancel")
			inputChan <- cancelDatagram
		} else if cancelFlag.ShutDown() {
			stopChan <- stopTypeShutdown
		}
		//cancel state is complete, safe return

	}(docState.InstancePluginsInformation)
	return &ExecuterBackend{
		output:   output,
		docState: docState,
		input:    inputChan,
		stopChan: stopChan,
	}
}

func (p *ExecuterBackend) Accept() <-chan string {
	return p.input
}

func (p *ExecuterBackend) Stop() <-chan int {
	return p.stopChan
}

//TODO handle error and logging
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
			//signal the caller that messaging should stop
			p.stopChan <- stopTypeTerminate
		}
	default:
		return errors.New("unsupported message type")
	}
	return nil
}

func (p *ExecuterBackend) Close() {
	//TODO once we refactored the cancelFlag structure, send signal to the cancelFlag listener routine
	close(p.stopChan)
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

func NewWorkerBackend(ctx context.T, runner PluginRunner) *WorkerBackend {
	stopChan := make(chan int)
	return &WorkerBackend{
		ctx:        ctx.With("DataBackend"),
		input:      make(chan string),
		cancelFlag: task.NewChanneledCancelFlag(),
		runner:     runner,
		stopChan:   stopChan,
	}
}

//TODO handle error and log
func (p *WorkerBackend) Process(datagram string) error {
	t, content := ParseDatagram(datagram)
	switch t {
	case MessageTypePluginConfig:
		var plugins []model.PluginState
		jsonutil.Unmarshal(content, &plugins)
		p.once.Do(func() {
			statusChan := make(chan contracts.PluginResult)
			go p.runner(p.ctx, plugins, statusChan, p.cancelFlag)
			go p.pluginListener(statusChan)
		})

	case MessageTypeCancel:
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
		results[res.PluginID] = &res
		//TODO move the aggregator under executer package and protect it, there's global lock in this package
		status, _, _ := docmanager.DocumentResultAggregator(log, res.PluginID, results)
		docResult := contracts.DocumentResult{
			Status:        status,
			PluginResults: results,
			LastPlugin:    res.PluginID,
		}
		replyMessage, _ := CreateDatagram(MessageTypeReply, docResult)
		log.Debugf("plugin: %v done, sending reply message...", res.PluginID)
		p.input <- replyMessage
	}

	log.Info("document execution complete, sending complete response...")
	status, _, _ := docmanager.DocumentResultAggregator(log, "", results)
	docResult := contracts.DocumentResult{
		Status:        status,
		PluginResults: results,
		LastPlugin:    "",
	}
	completeMessage, _ := CreateDatagram(MessageTypeComplete, docResult)
	p.input <- completeMessage
	//sending stop signal
	p.stopChan <- stopTypeTerminate
	close(p.stopChan)

}

func (p *WorkerBackend) Accept() <-chan string {
	return p.input
}

func (p *WorkerBackend) Stop() <-chan int {
	return p.stopChan
}

func (p *WorkerBackend) Close() {
	//close the cancelFlag in case there're listeners down the call stack
	p.cancelFlag.Set(task.Completed)
	//TODO signal plugin listner to stop as well
}
