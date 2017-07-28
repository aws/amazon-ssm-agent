package channel

import "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/contracts"

const (
	ModeClient Mode = "client"
	ModeServer Mode = "server"
)

type Mode string
type ChannelCreator func(handle string, mode Mode) (Channel, error)

//TODO this interface is designed to adopt both file and named pipe
type Channel interface {
	Connect() error
	Send(contracts.Message) error
	GetMessageChannel() chan contracts.Message
	Close()
}
