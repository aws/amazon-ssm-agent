package channel

const (
	ModeMaster Mode = "master"
	ModeWorker Mode = "worker"
)
const (
	defaultChannelBufferSize = 100
)

type Mode string
type ChannelCreator func(mode Mode) Channel

//Channel is an interface for raw json datagram transmission, it is designed to adopt both file ad named pipe
type Channel interface {
	//open a named channel, system call is issued at this call
	Open(string) error
	//send a raw json datagram to the channel, should be non-blocked
	Send(string) error
	GetMessageChannel() chan string
	Close()
}
