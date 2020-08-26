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

package core

import (
	"testing"
)

// This is an application-wide global ID allocator.  Unfortunately we need
// to have unique pipe IDs globally to permit certain things to work
// correctly.

func TestPipeAllocatorWrap(t *testing.T) {
	p := &pipeIDAllocator{}
	_ = p.Get() // as it will reset otherwise
	p.next = 0
	p.next--
	if v := p.Get(); v != 0x7fffffff {
		t.Errorf("Got wrong value for wrap: %x", v)
	}
	// skip zero
	if v := p.Get(); v != 1 {
		t.Errorf("Got wrong value for wrap: %x", v)
	}
}

func TestPipeAllocatorReuse(t *testing.T) {
	p := &pipeIDAllocator{}
	_ = p.Get() // as it will reset otherwise
	p.next = 0
	p.next--
	if v := p.Get(); v != 0x7fffffff {
		t.Errorf("Got wrong value for wrap: %x", v)
	}
	// skip zero
	if v := p.Get(); v != 1 {
		t.Errorf("Got wrong value for wrap: %x", v)
	}
	p.next = 0
	if v := p.Get(); v != 2 {
		t.Errorf("Maybe reused value?: %x", v)
	}
}

func TestPipeAllocatorRandom(t *testing.T) {
	p1 := &pipeIDAllocator{}
	p2 := &pipeIDAllocator{}

	v1 := p1.Get()
	v2 := p2.Get()

	if v1 == v2 {
		t.Errorf("values not random: %v %v", v1, v2)
	}
}

func TestPipeAllocatorUnusedFree(t *testing.T) {
	defer func() {
		pass := false
		if r := recover(); r != nil {
			if r != "free of unused pipe ID" {
				t.Errorf("Wrong value for r: %v", r)
			}
			pass = true
		}
		if !pass {
			t.Error("Unused free did not panic")
		}
	}()

	p := &pipeIDAllocator{}
	p.Free(2)
}

func TestPipeNoAddress(t *testing.T) {
	p := &pipe{}
	if addr := p.Address(); addr != "" {
		t.Errorf("Got unexpected address: %v", addr)
	}
}
