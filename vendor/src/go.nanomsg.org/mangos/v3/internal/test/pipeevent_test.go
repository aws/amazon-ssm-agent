// Copyright 2019 The Mangos Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use file except in compliance with the License.
// You may obtain a copy of the license at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package test

import (
	"sync"
	"testing"
	"time"

	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/pair"
	_ "go.nanomsg.org/mangos/v3/transport/inproc"
)

func TestPipeHook(t *testing.T) {
	self := GetSocket(t, pair.NewSocket)
	defer MustClose(t, self)
	peer := GetSocket(t, pair.NewSocket)

	type event struct {
		cnt map[mangos.PipeEvent]int
		p   mangos.Pipe
	}

	var lock sync.Mutex
	events := make(map[mangos.Pipe]event)

	hook := func(ev mangos.PipeEvent, p mangos.Pipe) {
		lock.Lock()
		defer lock.Unlock()
		if item, ok := events[p]; ok {
			item.cnt[ev]++
		} else {
			cnt := make(map[mangos.PipeEvent]int)
			cnt[ev] = 1
			events[p] = event{
				cnt: cnt,
				p:   p,
			}
		}
	}

	pass := 0
	self.SetPipeEventHook(hook)

	addr := AddrTestInp()
	MustSucceed(t, peer.Listen(addr))
	MustSucceed(t, self.Dial(addr))

	lock.Lock()
	for p, item := range events {
		MustBeTrue(t, p == item.p)
		MustBeTrue(t, item.cnt[mangos.PipeEventAttaching] == 1)
		MustBeTrue(t, item.cnt[mangos.PipeEventAttached] == 1)
		MustBeTrue(t, item.cnt[mangos.PipeEventDetached] == 0)
		pass++
	}
	MustBeTrue(t, pass == 1)
	pass = 0
	lock.Unlock()

	MustClose(t, peer)
	time.Sleep(time.Millisecond * 100)

	lock.Lock()
	for p, item := range events {
		MustBeTrue(t, p == item.p)
		MustBeTrue(t, item.cnt[mangos.PipeEventAttaching] == 1)
		MustBeTrue(t, item.cnt[mangos.PipeEventAttached] == 1)
		MustBeTrue(t, item.cnt[mangos.PipeEventDetached] == 1)
		pass++
	}
	MustBeTrue(t, pass == 1)
	lock.Unlock()
}

func TestPipeHookReject(t *testing.T) {
	self := GetSocket(t, pair.NewSocket)
	defer MustClose(t, self)
	peer := GetSocket(t, pair.NewSocket)

	type event struct {
		cnt map[mangos.PipeEvent]int
		p   mangos.Pipe
	}

	var lock sync.Mutex
	events := make(map[mangos.Pipe]*event)

	hook := func(ev mangos.PipeEvent, p mangos.Pipe) {
		lock.Lock()
		defer lock.Unlock()
		if item, ok := events[p]; ok {
			item.cnt[ev]++
		} else {
			cnt := make(map[mangos.PipeEvent]int)
			cnt[ev] = 1
			events[p] = &event{
				cnt: cnt,
				p:   p,
			}
		}
		if ev == mangos.PipeEventAttaching {
			_ = p.Close()
		}
	}

	MustSucceed(t, self.SetOption(mangos.OptionReconnectTime, time.Millisecond*10))
	MustSucceed(t, self.SetOption(mangos.OptionMaxReconnectTime, time.Millisecond*100))
	MustSucceed(t, self.SetOption(mangos.OptionDialAsynch, true))
	self.SetPipeEventHook(hook)

	addr := AddrTestInp()
	MustSucceed(t, peer.Listen(addr))
	MustSucceed(t, self.Dial(addr))

	time.Sleep(time.Millisecond * 100)
	lock.Lock()
	pass := 0
	for p, item := range events {
		MustBeTrue(t, p == item.p)
		MustBeTrue(t, item.cnt[mangos.PipeEventAttaching] == 1)
		MustBeTrue(t, item.cnt[mangos.PipeEventAttached] == 0)
		MustBeTrue(t, item.cnt[mangos.PipeEventDetached] == 0)
		pass++
	}
	MustBeTrue(t, pass > 2)
	lock.Unlock()

	MustClose(t, peer)
	time.Sleep(time.Millisecond * 100)

	lock.Lock()
	pass = 0
	for p, item := range events {
		MustBeTrue(t, p == item.p)
		MustBeTrue(t, item.cnt[mangos.PipeEventAttaching] == 1)
		MustBeTrue(t, item.cnt[mangos.PipeEventAttached] == 0)
		MustBeTrue(t, item.cnt[mangos.PipeEventDetached] == 0)
		pass++
	}
	MustBeTrue(t, pass > 2)
	lock.Unlock()
}
