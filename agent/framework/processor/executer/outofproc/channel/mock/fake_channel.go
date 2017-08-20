package channelmock

import (
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/channel"
	"github.com/aws/amazon-ssm-agent/agent/log"
)

var channelMap = make(map[string]chan string)
var mu sync.RWMutex

type FakeChannel struct {
	name string
	mode channel.Mode
}

func NewFakeChannel(log log.T, mode channel.Mode, name string) *FakeChannel {
	log.Infof("creating channel: %v|%v", name, mode)
	f := FakeChannel{
		mode: mode,
		name: name,
	}
	//if channel already exist, use the old one
	_, ok := channelMap[name+"-"+string(mode)]
	if ok {
		return &f
	}
	mu.RLock()
	defer mu.RUnlock()
	//The channel size need to be big enough so that the sender is not blocked
	//either one can open up a channel any time when open is called
	channelMap[name+"-"+string(channel.ModeMaster)] = make(chan string, 100)
	channelMap[name+"-"+string(channel.ModeWorker)] = make(chan string, 100)
	return &f
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

func IsExists(name string) bool {
	_, ok := channelMap[name+"-"+string(channel.ModeMaster)]
	return ok
}
