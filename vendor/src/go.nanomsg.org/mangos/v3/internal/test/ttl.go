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
	"bytes"
	"testing"
	"time"

	"go.nanomsg.org/mangos/v3"
)

// SetTTLZero tests that a given socket fails to set a TTL of zero.
func SetTTLZero(t *testing.T, f func() (mangos.Socket, error)) {
	s, err := f()
	if err != nil {
		t.Errorf("Failed to make socket: %v", err)
		return
	}
	defer s.Close()
	err = s.SetOption(mangos.OptionTTL, 0)
	switch err {
	case mangos.ErrBadValue: // expected result
	case nil:
		t.Errorf("Negative test fail, permitted zero TTL")
	default:
		t.Errorf("Negative test fail (0), wrong error %v", err)
	}
}

// SetTTLNegative tests that a given socket fails to set a negative TTL.
func SetTTLNegative(t *testing.T, f func() (mangos.Socket, error)) {
	s, err := f()
	if err != nil {
		t.Errorf("Failed to make socket: %v", err)
		return
	}
	defer s.Close()
	err = s.SetOption(mangos.OptionTTL, -1)
	switch err {
	case mangos.ErrBadValue: // expected result
	case nil:
		t.Errorf("Negative test fail, permitted negative TTL")
	default:
		t.Errorf("Negative test fail (-1), wrong error %v", err)
	}
}

// SetTTLTooBig tests that a given socket fails to set a very large TTL.
func SetTTLTooBig(t *testing.T, f func() (mangos.Socket, error)) {
	s, err := f()
	if err != nil {
		t.Errorf("Failed to make socket: %v", err)
		return
	}
	defer s.Close()
	err = s.SetOption(mangos.OptionTTL, 256)
	switch err {
	case mangos.ErrBadValue: // expected result
	case nil:
		t.Errorf("Negative test fail, permitted too large TTL")
	default:
		t.Errorf("Negative test fail (256), wrong error %v", err)
	}
}

// SetTTLNotInt tests that a given socket fails to set a non-integer TTL.
func SetTTLNotInt(t *testing.T, f func() (mangos.Socket, error)) {
	s, err := f()
	if err != nil {
		t.Errorf("Failed to make socket: %v", err)
		return
	}
	defer s.Close()
	err = s.SetOption(mangos.OptionTTL, "garbage")
	switch err {
	case mangos.ErrBadValue: // expected result
	case nil:
		t.Errorf("Negative test fail, permitted non-int value")
	default:
		t.Errorf("Negative test fail (garbage), wrong error %v", err)
	}
}

// SetTTL tests that we can set a valid TTL, and get the same value back.
func SetTTL(t *testing.T, f func() (mangos.Socket, error)) {
	s, err := f()
	if err != nil {
		t.Errorf("Failed to make socket: %v", err)
		return
	}
	defer s.Close()

	err = s.SetOption(mangos.OptionTTL, 2)
	if err != nil {
		t.Errorf("Failed SetOption: %v", err)
		return
	}

	v, err := s.GetOption(mangos.OptionTTL)
	if err != nil {
		t.Errorf("Failed GetOption: %v", err)
		return
	}
	if val, ok := v.(int); !ok {
		t.Errorf("Returned value not type int")
	} else if val != 2 {
		t.Errorf("Returned value %d not %d", val, 2)
	}
}

// TTLDropTest is a generic test for dropping based on TTL expiration.
// F1 makes the Client socket, f2 makes the Server socket.
func TTLDropTest(t *testing.T,
	cli func() (mangos.Socket, error),
	srv func() (mangos.Socket, error),
	rawcli func() (mangos.Socket, error),
	rawsrv func() (mangos.Socket, error)) {

	nhop := 3
	clis := make([]mangos.Socket, 0, nhop)
	srvs := make([]mangos.Socket, 0, nhop)
	var addrs []string
	for i := 0; i < nhop; i++ {
		addrs = append(addrs, AddrTestInp())
	}

	for i := 0; i < nhop; i++ {
		var fn func() (mangos.Socket, error)
		if i == nhop-1 {
			fn = srv
		} else {
			fn = rawsrv
		}
		s, err := fn()
		MustSucceed(t, err)
		MustNotBeNil(t, s)
		defer s.Close()

		MustSucceed(t, s.Listen(addrs[i]))
		srvs = append(srvs, s)
	}

	for i := 0; i < nhop; i++ {
		var fn func() (mangos.Socket, error)
		if i == 0 {
			fn = cli
		} else {
			fn = rawcli
		}
		s, err := fn()
		MustSucceed(t, err)
		MustNotBeNil(t, s)
		defer s.Close()

		MustSucceed(t, s.Dial(addrs[i]))

		clis = append(clis, s)
	}

	// Now make the device chain
	for i := 0; i < nhop-1; i++ {
		MustSucceed(t, mangos.Device(srvs[i], clis[i+1]))
	}

	// Wait for the various connections to plumb up
	time.Sleep(time.Millisecond * 100)

	// At this point, we can issue requests on clis[0], and read them from
	// srvs[nhop-1].

	rq := clis[0]
	rp := srvs[nhop-1]

	MustSucceed(t, rp.SetOption(mangos.OptionRecvDeadline, time.Millisecond*100))
	MustSucceed(t, rq.Send([]byte("GOOD")))
	v, err := rp.Recv()
	MustSucceed(t, err)
	MustBeTrue(t, bytes.Equal(v, []byte("GOOD")))

	// Now try setting the option.
	MustSucceed(t, rp.SetOption(mangos.OptionTTL, nhop-1))

	MustSucceed(t, rq.Send([]byte("DROP")))

	_, err = rp.Recv()
	MustBeError(t, err, mangos.ErrRecvTimeout)
}
