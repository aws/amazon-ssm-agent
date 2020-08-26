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

package main

import (
	"runtime"
	"strings"
	"sync"
	"testing"

	. "go.nanomsg.org/mangos/v3/internal/test"
)

var wg sync.WaitGroup
var exitCode int

func safeExit(code int) {
	defer wg.Done()
	exitCode = code
	runtime.Goexit()
}

func TestTheMain1(t *testing.T) {
	exitCode = 0
	wg.Add(1)
	capture := &strings.Builder{}
	exitFunc = safeExit
	stdErr = capture
	args = []string{"macat", "--help"}
	go main()
	wg.Wait()
}

func TestTheMain2(t *testing.T) {
	exitCode = 0
	wg.Add(1)
	capture := &strings.Builder{}
	exitFunc = safeExit
	stdErr = capture
	args = []string{"macat", "extra"}
	go main()
	wg.Wait()
	MustBeTrue(t, exitCode == 1)

}

func TestTheMain3(t *testing.T) {
	exitCode = 0
	wg.Add(1)
	capture := &strings.Builder{}
	exitFunc = safeExit
	stdErr = capture
	args = []string{"macat", "--dial"}
	go main()
	wg.Wait()
	MustBeTrue(t, exitCode == 1)

}
