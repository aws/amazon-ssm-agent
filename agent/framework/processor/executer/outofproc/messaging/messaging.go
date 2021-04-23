package messaging

import (
	"errors"
	"runtime/debug"

	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/filewatcherbasedipc"
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
	//Sets input channel to nil.
	Close()
	//Sets stop channel to nil.
	CloseStop()
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
func Messaging(log log.T, ipc filewatcherbasedipc.IPCChannel, backend MessagingBackend, stopTimer chan bool) (err error) {

	defer func() {
		if msg := recover(); msg != nil {
			log.Errorf("messaging worker panic: %v", msg)
			log.Errorf("Stacktrace:\n%s", debug.Stack())
		}
	}()

	log.Infof("inter process communication started at %v", ipc.GetPath())
	requestedStop := false
	inboundClosed := false
	//TODO add timer, if IPC is unresponsive to Close(), force return
	for {
		select {
		case <-stopTimer:
			log.Error("received timedout signal!")
			err = errors.New("document worker timed out")
			//messaging already timed out, close ipc and wait for done
			ipc.Close()

		case signal, more := <-backend.Stop():
			//stopChannel is closed, stop transmission
			if !more {
				ipc.Close()
				backend.CloseStop()
				break
			}
			//soft stop, safely close IPC
			if signal == stopTypeShutdown {
				log.Info("requested shutdown, prepare to stop messaging")
				requestedStop = true
				//TODO add timer, and if inbound has not closed within a given period, force return
				if inboundClosed {
					ipc.Close()
				}
				break
			} else if signal == stopTypeTerminate {
				//hard stop, remove the channel and force return
				log.Info("requested terminate messaging worker, destroying the channel")
				ipc.Destroy()
				return
			}
		case datagram, more := <-backend.Accept():
			if !more {
				inboundClosed = true
				if requestedStop {
					ipc.Close()
				}
				// Set channel to nil by calling Close function. Receive on closed channel is non blocking
				// and leads to endless loop causing cpu usage spike
				backend.Close()
				//if inbound channel from backend breaks, still continue messaging to send outbound messages
				break
			}

			log.Debugf("sending datagram to %v: %v", ipc.GetPath(), datagram)
			if err = ipc.Send(datagram); err != nil {
				//this is fatal error, force return
				log.Errorf("failed to send message to ipc channel: %v", err)
				return
			}
		case datagram, more := <-ipc.GetMessage():
			if !more {
				//safe close
				log.Info("ipc channel closed, stop messaging worker")
				return
			}

			log.Debugf("received datagram from %v: %v", ipc.GetPath(), datagram)
			if err = backend.Process(datagram); err != nil {
				//encountered error in databackend, it's up to the backend to decide whether close or not
				log.Errorf("messaging pipeline process datagram encountered error: %v", err)
			}

		}
	}
}
