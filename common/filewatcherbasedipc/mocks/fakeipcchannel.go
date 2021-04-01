package ipcchannelmock

import (
	"sync"

	"errors"

	"time"

	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/filewatcherbasedipc"
)

type queue []string

var mu sync.RWMutex

type ch struct {
	q0 *queue
	q1 *queue
}

var queueMap = make(map[string]ch)

func (q *queue) Enqueue(elem string) {
	mu.RLock()
	defer mu.RUnlock()
	*q = append(*q, elem)
}

func (q *queue) Dequeue() (string, bool) {
	if len(*q) == 0 {
		return "", false
	}
	mu.RLock()
	defer mu.RUnlock()
	res := (*q)[0]
	*q = (*q)[1:]
	return res, true
}

type FakeChannel struct {
	recvChan chan string
	closed   bool
	name     string
	mode     filewatcherbasedipc.Mode
}

func getQueue(c ch, mode filewatcherbasedipc.Mode) (*queue, *queue) {
	if mode == filewatcherbasedipc.ModeMaster || mode == filewatcherbasedipc.ModeSurveyor {
		return c.q0, c.q1
	} else {
		return c.q1, c.q0
	}
}

func NewFakeChannel(log log.T, mode filewatcherbasedipc.Mode, name string) *FakeChannel {
	log.Infof("creating channel: %v|%v", name, mode)
	if _, ok := queueMap[name]; !ok {
		queueMap[name] = ch{
			q0: &queue{},
			q1: &queue{},
		}
	}
	recvChan := make(chan string, 100)
	_, recvQ := getQueue(queueMap[name], mode)
	f := FakeChannel{
		mode:     mode,
		name:     name,
		recvChan: recvChan,
	}
	go f.poll(name, recvQ, recvChan)
	return &f
}

func (f *FakeChannel) poll(name string, q *queue, recvChan chan string) {
	for {
		if f.closed {
			return
		}
		if _, ok := queueMap[name]; !ok {
			fmt.Println("fatal: channel " + name + " already destroyed while receiving...")
			return
		}
		if msg, ok := q.Dequeue(); ok {
			recvChan <- msg
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (f *FakeChannel) Send(message string) error {

	if c, ok := queueMap[f.name]; !ok {
		return errors.New("channel not found")
	} else {
		sendQ, _ := getQueue(c, f.mode)
		sendQ.Enqueue(message)
	}
	return nil
}

func (f *FakeChannel) GetMessage() <-chan string {
	return f.recvChan
}

//close stops the receiving channel
func (f *FakeChannel) Close() {
	if f.closed {
		return
	}
	f.closed = true
	time.Sleep(120 * time.Millisecond)
	close(f.recvChan)
	return
}

func (f *FakeChannel) Destroy() {
	//first, close the channel
	f.Close()
	//only master will remove the channel object
	if f.mode == filewatcherbasedipc.ModeMaster || f.mode == filewatcherbasedipc.ModeSurveyor {
		delete(queueMap, f.name)
	}
}

func (f *FakeChannel) CleanupOwnModeFiles() {
	if c, ok := queueMap[f.name]; ok {
		sendQ, _ := getQueue(c, f.mode)
		isPresent := true
		for isPresent {
			_, isPresent = sendQ.Dequeue()
		}
	}
}

func (f *FakeChannel) GetPath() string {
	return f.name
}

func IsExists(name string) bool {
	_, ok := queueMap[name]
	return ok
}
