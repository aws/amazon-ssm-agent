package channelmock

import (
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/channel"
)

var channelMap = make(map[string]chan string)
var mu sync.RWMutex

//TODO add channel destroy flag
type FakeChannel struct {
	name string
	mode channel.Mode
}

func NewFakeChannel(mode channel.Mode) *FakeChannel {
	return &FakeChannel{
		mode: mode,
	}
}

//this operation need to be synced since Open() is called by 2 separete go-routines, in file channel, sync is guaranteed by renaming action.
func (f *FakeChannel) Open(name string) error {
	f.name = name
	//if channel already exist, use the old one
	_, ok := channelMap[name+"-"+string(f.mode)]
	if ok {
		return nil
	}
	mu.RLock()
	defer mu.RUnlock()
	//The channel size need to be big enough so that the sender is not blocked
	//either one can open up a channel any time when open is called
	channelMap[name+"-"+string(channel.ModeMaster)] = make(chan string, 100)
	channelMap[name+"-"+string(channel.ModeWorker)] = make(chan string, 100)
	return nil
}

func (f *FakeChannel) Send(message string) error {
	channelMap[f.name+"-"+string(f.mode)] <- message
	return nil
}

func (f *FakeChannel) GetMessageChannel() chan string {
	if f.mode == channel.ModeMaster {
		return channelMap[f.name+"-"+string(channel.ModeWorker)]
	} else {
		return channelMap[f.name+"-"+string(channel.ModeMaster)]
	}
}

//close operation also needs to be synced
func (f *FakeChannel) Close() {
	//only master will remove the channel object
	mu.RLock()
	defer mu.RUnlock()
	if f.mode == channel.ModeMaster {
		delete(channelMap, f.name+"-"+string(channel.ModeWorker))
		delete(channelMap, f.name+"-"+string(channel.ModeMaster))
	}

	return
}

func IsClose(name string) bool {
	_, ok := channelMap[name+"-"+string(channel.ModeMaster)]
	return !ok
}
