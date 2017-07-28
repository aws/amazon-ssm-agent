package channelmock

import "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/contracts"

type FakeChannel struct {
	ch chan contracts.Message
}

func (f *FakeChannel) Connect() error {
	f.ch = make(chan contracts.Message, 10)
	return nil
}

func (f *FakeChannel) Send(message contracts.Message) error {
	f.ch <- message
	return nil
}

func (f *FakeChannel) GetMessageChannel() chan contracts.Message {
	return f.ch
}

func (f *FakeChannel) Close() {
	close(f.ch)
	return
}
