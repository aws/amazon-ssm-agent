package channel

const (
	ModeMaster Mode = "master"
	ModeSlave  Mode = "slave"
)
const (
	defaultReceiveChannelBufferSize = 100
)

type Mode string
type ChannelCreator func(handle string, mode Mode) (Channel, error)

//Channel is an interface for raw json datagram transmission, it is designed to adopt both file ad named pipe
type Channel interface {
	//open a named channel, system call is issued at this call
	Open(string) error
	//send a raw json datagram to the channel
	Send(string) error
	GetMessageChannel() chan string
	Close()
}
