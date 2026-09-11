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

// queueMapMu guards queueMap. Separate from mu, which guards queue contents: poll() reads the map
// and then dequeues, so one lock covering both would have to be re-entrant.
var queueMapMu sync.RWMutex
var queueMap = make(map[string]ch)

func (q *queue) Enqueue(elem string) {
	mu.Lock()
	defer mu.Unlock()
	*q = append(*q, elem)
}

func (q *queue) Dequeue() (string, bool) {
	mu.Lock()
	defer mu.Unlock()
	if len(*q) == 0 {
		return "", false
	}
	res := (*q)[0]
	*q = (*q)[1:]
	return res, true
}

// lookupChannel returns the queue pair registered for name.
func lookupChannel(name string) (ch, bool) {
	queueMapMu.RLock()
	defer queueMapMu.RUnlock()
	c, ok := queueMap[name]
	return c, ok
}

type FakeChannel struct {
	recvChan    chan string
	closedMutex sync.RWMutex // mutex on the closed variable below
	closed      bool
	name        string
	mode        filewatcherbasedipc.Mode
	// Close closes stopPoll to stop poll, then waits on pollDone before closing recvChan, so the
	// close cannot race poll's send -- which is both a data race and a panic.
	stopPoll chan struct{}
	pollDone chan struct{}
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
	queueMapMu.Lock()
	if _, ok := queueMap[name]; !ok {
		queueMap[name] = ch{
			q0: &queue{},
			q1: &queue{},
		}
	}
	c := queueMap[name]
	queueMapMu.Unlock()

	recvChan := make(chan string, 100)
	_, recvQ := getQueue(c, mode)
	f := FakeChannel{
		closedMutex: sync.RWMutex{},
		mode:        mode,
		name:        name,
		recvChan:    recvChan,
		stopPoll:    make(chan struct{}),
		pollDone:    make(chan struct{}),
	}
	go f.poll(name, recvQ, recvChan)
	return &f
}

func (f *FakeChannel) poll(name string, q *queue, recvChan chan string) {
	// Signals Close that no further send on recvChan can happen.
	defer close(f.pollDone)
	for {
		select {
		case <-f.stopPoll:
			return
		default:
		}

		if _, ok := lookupChannel(name); !ok {
			fmt.Println("fatal: channel " + name + " already destroyed while receiving...")
			return
		}
		if msg, ok := q.Dequeue(); ok {
			// stopPoll here too, so a full recvChan with no reader cannot wedge Close.
			select {
			case recvChan <- msg:
			case <-f.stopPoll:
				return
			}
		}
		select {
		case <-f.stopPoll:
			return
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (f *FakeChannel) Send(message string) error {
	if c, ok := lookupChannel(f.name); !ok {
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

func (f *FakeChannel) isClosed() bool {
	f.closedMutex.RLock()
	defer f.closedMutex.RUnlock()
	return f.closed
}

// close stops the receiving channel
func (f *FakeChannel) Close() {
	// Test and set under one acquisition: checking isClosed() then locking lets two concurrent
	// Close calls both reach close(recvChan), which panics.
	f.closedMutex.Lock()
	if f.closed {
		f.closedMutex.Unlock()
		return
	}
	f.closed = true
	close(f.stopPoll)
	f.closedMutex.Unlock()

	// Wait for poll rather than sleeping and hoping: a sleep only makes the race unlikely.
	<-f.pollDone
	close(f.recvChan)
}

func (f *FakeChannel) Destroy() {
	//first, close the channel
	f.Close()
	//only master will remove the channel object
	if f.mode == filewatcherbasedipc.ModeMaster || f.mode == filewatcherbasedipc.ModeSurveyor {
		queueMapMu.Lock()
		delete(queueMap, f.name)
		queueMapMu.Unlock()
	}
}

func (f *FakeChannel) CleanupOwnModeFiles() {
	if c, ok := lookupChannel(f.name); ok {
		sendQ, _ := getQueue(c, f.mode)
		isPresent := true
		for isPresent {
			_, isPresent = sendQ.Dequeue()
		}
	}
}

func (f *FakeChannel) GetMode() filewatcherbasedipc.Mode {
	return f.mode
}

func (f *FakeChannel) GetPath() string {
	return f.name
}

// RemoveChannel drops the channel's queues from the registry, standing in for the removal of the
// on-disk channel directory that RemoveFileWatcherChannel performs.
func RemoveChannel(name string) {
	queueMapMu.Lock()
	defer queueMapMu.Unlock()
	delete(queueMap, name)
}

func IsExists(name string) bool {
	queueMapMu.RLock()
	defer queueMapMu.RUnlock()
	_, ok := queueMap[name]
	return ok
}
