// +build !windows

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

package transport

import (
	"encoding/binary"
	"io"
	"net"

	"go.nanomsg.org/mangos/v3"
)

// NewConnPipeIPC allocates a new Pipe using the IPC exchange protocol.
func NewConnPipeIPC(c net.Conn, proto ProtocolInfo) ConnPipe {
	p := &connipc{
		conn: conn{
			c:       c,
			proto:   proto,
			options: make(map[string]interface{}),
			maxrx:   0,
		},
	}
	p.options[mangos.OptionMaxRecvSize] = 0
	p.options[mangos.OptionLocalAddr] = c.LocalAddr()
	p.options[mangos.OptionRemoteAddr] = c.RemoteAddr()
	return p
}

func (p *connipc) Send(msg *Message) error {

	var buff = net.Buffers{}

	// Serialize the length header
	l := uint64(len(msg.Header) + len(msg.Body))
	lbyte := make([]byte, 9)
	lbyte[0] = 1
	binary.BigEndian.PutUint64(lbyte[1:], l)

	// Attach the length header along with the actual header and body
	buff = append(buff, lbyte, msg.Header, msg.Body)

	if _, err := buff.WriteTo(p.c); err != nil {
		return err
	}
	msg.Free()
	return nil
}

func (p *connipc) Recv() (*Message, error) {

	var sz int64
	var err error
	var msg *Message
	var one [1]byte

	if _, err = p.c.Read(one[:]); err != nil {
		return nil, err
	}
	if err = binary.Read(p.c, binary.BigEndian, &sz); err != nil {
		return nil, err
	}

	// Limit messages to the maximum receive value, if not
	// unlimited.  This avoids a potential denaial of service.
	if sz < 0 || (p.maxrx > 0 && sz > int64(p.maxrx)) {
		return nil, mangos.ErrTooLong
	}
	msg = mangos.NewMessage(int(sz))
	msg.Body = msg.Body[0:sz]
	if _, err = io.ReadFull(p.c, msg.Body); err != nil {
		msg.Free()
		return nil, err
	}
	return msg, nil
}
