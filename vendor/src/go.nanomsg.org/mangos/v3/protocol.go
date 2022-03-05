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

package mangos

// ProtocolPipe represents the handle that a Protocol implementation has
// to the underlying stream transport.  It can be thought of as one side
// of a TCP, IPC, or other type of connection.
type ProtocolPipe interface {
	// ID returns a unique 31-bit value associated with this.
	// The value is unique for a given socket, at a given time.
	ID() uint32

	// Close does what you think.
	Close() error

	// SendMsg sends a message.  On success it returns nil. This is a
	// blocking call.
	SendMsg(*Message) error

	// RecvMsg receives a message.  It blocks until the message is
	// received.  On error, the pipe is closed and nil is returned.
	RecvMsg() *Message

	// SetPrivate is used to set protocol private data.
	SetPrivate(interface{})

	// GetPrivate returns the previously stored protocol private data.
	GetPrivate() interface{}
}

// ProtocolInfo is a description of the protocol.
type ProtocolInfo struct {
	Self     uint16
	Peer     uint16
	SelfName string
	PeerName string
}

// ProtocolContext is a "context" for a protocol, which contains the
// various stateful operations such as timers, etc. necessary for
// running the protocol.  This is separable from the protocol itself
// as the protocol may permit the creation of multiple contexts.
type ProtocolContext interface {
	// Close closes the context.
	Close() error

	// SendMsg sends the message.  The message may be queued, or
	// may be delivered immediately, depending on the nature of
	// the protocol.  On success, the context assumes ownership
	// of the message.  On error, the caller retains ownership,
	// and may either resend the message or dispose of it otherwise.
	SendMsg(*Message) error

	// RecvMsg receives a complete message, including the message header,
	// which is useful for protocols in raw mode.
	RecvMsg() (*Message, error)

	// GetOption is used to retrieve the current value of an option.
	// If the protocol doesn't recognize the option, EBadOption should
	// be returned.
	GetOption(string) (interface{}, error)

	// SetOption is used to set an option.  EBadOption is returned if
	// the option name is not recognized, EBadValue if the value is
	// invalid.
	SetOption(string, interface{}) error
}

// ProtocolBase provides the protocol-specific handling for sockets.
// This is the new style API for sockets, and is how protocols provide
// their specific handling.
type ProtocolBase interface {
	ProtocolContext

	// Info returns the information describing this protocol.
	Info() ProtocolInfo

	// XXX: Revisit these when we can use Pipe natively.

	// AddPipe is called when a new Pipe is added to the socket.
	// Typically this is as a result of connect or accept completing.
	// The pipe ID will be unique for the socket at this time.
	// The implementation must not call back into the socket, but it
	// may reject the pipe by returning a non-nil result.
	AddPipe(ProtocolPipe) error

	// RemovePipe is called when a Pipe is removed from the socket.
	// Typically this indicates a disconnected or closed connection.
	// This is called exactly once, after the underlying transport pipe
	// is closed.  The Pipe ID will still be valid.
	RemovePipe(ProtocolPipe)

	// OpenContext is a request to create a unique instance of the
	// protocol state machine, allowing concurrent use of states on
	// a given protocol socket.  Protocols that don't support this
	// should return ErrProtoOp.
	OpenContext() (ProtocolContext, error)
}

// Useful constants for protocol numbers.  Note that the major protocol number
// is stored in the upper 12 bits, and the minor (subprotocol) is located in
// the bottom 4 bits.
const (
	ProtoPair       = (1 * 16)
	ProtoPub        = (2 * 16)
	ProtoSub        = (2 * 16) + 1
	ProtoReq        = (3 * 16)
	ProtoRep        = (3 * 16) + 1
	ProtoPush       = (5 * 16)
	ProtoPull       = (5 * 16) + 1
	ProtoSurveyor   = (6 * 16) + 2
	ProtoRespondent = (6 * 16) + 3
	ProtoBus        = (7 * 16)
	ProtoStar       = (100 * 16) // Experimental!
)
