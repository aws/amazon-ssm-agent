package outofproc

import (
	"errors"
	"os"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/channel"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/contracts"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

const (
	defaultClientTimeout = 100 * time.Second
)

type PluginRunner func(
	context context.T,
	plugins []model.PluginState,
	resChan chan contracts.PluginResult,
	cancelFlag task.CancelFlag,
)

func Client(ctx context.T, handle string, runner PluginRunner) (err error) {
	var log = ctx.Log()
	var isRunning = false

	//TODO implement reconnect
	//client connect to the connection object which is already set up by server
	ipc, err := channelCreator(handle, channel.ModeClient)
	if err := ipc.Connect(); err != nil {
		log.Errorf("failed to connect to ")
		os.Exit(1)
	}
	defer ipc.Close()
	//Listen to the pluginConfigs
	onMessageChannel := ipc.GetMessageChannel()
	statusChan := make(chan contracts.PluginResult)
	//TODO client should adapt to server's version during downgrade
	curVersion := messageContracts.GetLastestVersion()
	log.Infof("current accepted message version: %v", curVersion)
	results := make(map[string]*contracts.PluginResult)
	timer := time.After(defaultClientTimeout)
	cancelFlag := task.NewChanneledCancelFlag()
	for {
		select {
		case <-timer:
			if !isRunning {
				err = errors.New("plugin still not started, timeout and exit")
				return
			}
		case message := <-onMessageChannel:
			if message.Type == messageContracts.MessageTypeStart && !isRunning {
				var plugins []model.PluginState
				if err = messageContracts.UnMarshal(message, &plugins); err != nil {
					log.Errorf("Unmarshalling start message failed: %v", err)
					//timeout channel will terminate connection
					break
				}
				//set running flag
				isRunning = true
				go runner(ctx, plugins, statusChan, cancelFlag)

			} else if message.Type == messageContracts.MessageTypeControl {
				var ctrl DocumentControl
				if err = messageContracts.UnMarshal(message, &ctrl); err != nil {
					//client cannot parse server's control message, should immediately return
					log.Errorf("Unmarshalling control message failed: %v", err)
					return
				}
				if isClientCancelled(ctrl) {
					log.Info("executer requested cancel, setting cancelFlag...")
					cancelFlag.Set(task.Canceled)
					//TODO may enforce a timeout to force exit if runplugins is not responsive
					break
				}
			}
		case res, more := <-statusChan:
			//runplugins complete
			if !more {
				log.Info("plugins execution complete, send document complete response...")
				status, _, _ := docmanager.DocumentResultAggregator(ctx.Log(), "", results)
				docResult := contracts.DocumentResult{
					Status:        status,
					PluginResults: results,
					LastPlugin:    "",
				}
				var completeMessage messageContracts.Message
				if completeMessage, err = messageContracts.Marshal(
					curVersion, messageContracts.MessageTypePayload, docResult); err != nil {
					log.Errorf("failed to create plugin result message: %v", err)
					return
				}
				ipc.Send(completeMessage)
				closeMessage := messageContracts.Message{
					Version: curVersion,
					Type:    messageContracts.MessageTypeClose,
				}
				log.Info("sending close message...")
				ipc.Send(closeMessage)
				return
			}
			results[res.PluginName] = &res
			//TODO move the aggregator under executer package and protect it
			status, _, _ := docmanager.DocumentResultAggregator(ctx.Log(), res.PluginName, results)
			docResult := contracts.DocumentResult{
				Status:        status,
				PluginResults: results,
				LastPlugin:    res.PluginName,
			}
			var payloadMessage messageContracts.Message
			if payloadMessage, err = messageContracts.Marshal(
				curVersion, messageContracts.MessageTypePayload, docResult); err != nil {
				log.Errorf("failed to create plugin result message: %v", err)
				return
			}
			ipc.Send(payloadMessage)
		}
	}
}
