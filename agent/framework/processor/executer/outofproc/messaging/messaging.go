package messaging

import (
	"errors"

	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/channel"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
)

type MessageType string

const (
	stopTypeTerminate = 1
	stopTypeShutdown  = 2
)

//Message types
const (
	MessageTypePluginConfig = "pluginconfig"
	MessageTypeComplete     = "complete"
	MessageTypeReply        = "reply"
	MessageTypeCancel       = "cancel"
)

var versions = []string{"1.0"}

type Message struct {
	Version string      `json:"version"`
	Type    MessageType `json:"type"`
	Content string      `json:"content"`
}

//MessagingBackend defines an asycn message in/out processing pipeline
type MessagingBackend interface {
	Accept() <-chan string
	Stop() <-chan int
	//Process a given datagram, should not be blocked
	Process(string) error
	Close()
}

//GetLatestVersion retrieves the current latest message version of the agent build
func GetLatestVersion() string {
	return versions[len(versions)-1]
}

//CreateDatagram marshals a given arbitrary object to raw json string
//Message schema is determined by the current version, content struct is indicated by type field
//TODO add version handling
func CreateDatagram(t MessageType, content interface{}) (string, error) {
	contentStr, err := jsonutil.Marshal(content)
	if err != nil {
		return "", err
	}
	message := Message{
		Version: GetLatestVersion(),
		Type:    t,
		Content: contentStr,
	}
	datagram, err := jsonutil.Marshal(message)
	if err != nil {
		return "", err
	}
	return datagram, nil
}

//TODO add version and error handling
func ParseDatagram(datagram string) (MessageType, string) {
	message := Message{}
	jsonutil.Unmarshal(datagram, &message)
	return message.Type, message.Content
}

// Messaging implements the duplex transmission between master and worker, it send datagram it received to data backend,
// TODO ipc should not be destroyed within this worker, destroying ipc object should be done in its caller: Executer
func Messaging(log log.T, ipc channel.Channel, backend MessagingBackend, stopTimer chan bool) (err error) {

	defer func() {
		if msg := recover(); msg != nil {
			log.Errorf("messaging worker panic: %v", msg)
		}
		//make sure to close the outbound channel when return in order to signal Processor
		backend.Close()
	}()

	onMessageChannel := ipc.GetMessageChannel()
	for {
		select {
		case <-stopTimer:
			log.Error("received timedout signal!")
			return errors.New("messaging worker timed out")
		case signal, more := <-backend.Stop():
			//stopChannel is closed, stop transmission
			if !more {
				log.Info("backend already stopped, stop messaging")
				return
			}
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
		case datagram, more := <-backend.Accept():
			if !more {
				//if inbound channel from backend breaks, still continue messaging to send outbound messages
				break
			}
			log.Debugf("sending datagram: %v", datagram)
			if err = ipc.Send(datagram); err != nil {
				log.Errorf("failed to send message to ipc channel: %v", err)
				return
			}
		case datagram, more := <-onMessageChannel:
			log.Debugf("received datagram: %v", datagram)
			if !more {
				log.Info("ipc channel closed, stop messaging worker")
				return
			}
			if err = backend.Process(datagram); err != nil {
				log.Errorf("messaging pipeline process datagram encountered error: %v", err)
				return
			}

		}
	}
}

func ParseArgv(argv []string) (string, string, error) {
	if len(argv) < 2 {
		return "", "", errors.New("not enough argument input to the executable")
	}
	return argv[0], argv[1], nil
}

func FormArgv(channelName string) []string {
	return []string{channelName}
}
