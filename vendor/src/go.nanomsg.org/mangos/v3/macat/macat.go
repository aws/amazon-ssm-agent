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

// Package macat implements a command-line interface to send and receive
// data via the mangos implementation of the SP (nanomsg) protocols.  It is
// designed to be suitable for use as a drop-in replacement for nanocat(1).`
package macat

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/optopia"

	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/bus"
	"go.nanomsg.org/mangos/v3/protocol/pair"
	"go.nanomsg.org/mangos/v3/protocol/pub"
	"go.nanomsg.org/mangos/v3/protocol/pull"
	"go.nanomsg.org/mangos/v3/protocol/push"
	"go.nanomsg.org/mangos/v3/protocol/rep"
	"go.nanomsg.org/mangos/v3/protocol/req"
	"go.nanomsg.org/mangos/v3/protocol/respondent"
	"go.nanomsg.org/mangos/v3/protocol/star"
	"go.nanomsg.org/mangos/v3/protocol/sub"
	"go.nanomsg.org/mangos/v3/protocol/surveyor"
	"go.nanomsg.org/mangos/v3/transport/all"
)

// Duration is our internal duration, which parses bare numbers as seconds.
// Otherwise it's just a time.Duration.
type Duration time.Duration

// UnmarshalText implements the encoding.TextUnmarshaller.  It parses
// bare integers as strings for legacy reasons.
func (d *Duration) UnmarshalText(b []byte) error {
	if val, err := strconv.Atoi(string(b)); err == nil {
		*d = Duration(val) * Duration(time.Second)
		return nil
	}
	if dur, err := time.ParseDuration(string(b)); err == nil {
		*d = Duration(dur)
		return nil
	}
	return errors.New("value is not a duration")
}

// App is an instance of the macat application.
type App struct {
	verbose       int
	dialAddr      []string
	bindAddr      []string
	subscriptions []string
	recvTimeout   Duration
	sendTimeout   Duration
	sendInterval  Duration
	sendDelay     Duration
	sendData      []byte
	printFormat   string
	sock          mangos.Socket
	tlsCfg        tls.Config
	certFile      string
	keyFile       string
	noVerifyTLS   bool
	options       *optopia.Options
	count         int
	countSet      bool
	stdOut        io.Writer
}

func mustSucceed(e error) {
	if e != nil {
		panic(e.Error())
	}
}

func (a *App) setSocket(f func() (mangos.Socket, error)) error {
	var err error
	if a.sock != nil {
		return errors.New("protocol already selected")
	}
	a.sock, err = f()
	mustSucceed(err)
	all.AddTransports(a.sock)
	return nil
}

func (a *App) addDial(addr string) error {
	if !strings.Contains(addr, "://") {
		return errors.New("invalid address format")
	}
	a.dialAddr = append(a.dialAddr, addr)
	return nil
}

func (a *App) addBind(addr string) error {
	if !strings.Contains(addr, "://") {
		return errors.New("invalid address format")
	}
	a.bindAddr = append(a.bindAddr, addr)
	return nil
}

func (a *App) addBindIPC(path string) error {
	return a.addBind("ipc://" + path)
}

func (a *App) addDialIPC(path string) error {
	return a.addDial("ipc://" + path)
}

func (a *App) addBindLocal(port string) error {
	return a.addBind("tcp://127.0.0.1:" + port)
}

func (a *App) addDialLocal(port string) error {
	return a.addDial("tcp://127.0.0.1:" + port)
}

func (a *App) addSub(sub string) error {
	a.subscriptions = append(a.subscriptions, sub)
	return nil
}

func (a *App) setSendData(data string) error {
	if a.sendData != nil {
		return errors.New("data or file already set")
	}
	a.sendData = []byte(data)
	return nil
}

func (a *App) setSendFile(path string) error {
	if a.sendData != nil {
		return errors.New("data or file already set")
	}
	var err error
	a.sendData, err = ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	return nil
}

func (a *App) setFormat(f string) error {
	if len(a.printFormat) > 0 {
		return errors.New("output format already set")
	}
	switch f {
	case "no":
	case "raw":
	case "ascii":
	case "quoted":
	case "msgpack":
	default:
		return errors.New("invalid format type: " + f)
	}
	a.printFormat = f
	return nil
}

func (a *App) setCert(path string) error {
	if len(a.certFile) != 0 {
		return errors.New("certificate file already set")
	}
	a.certFile = path
	return nil
}

func (a *App) setKey(path string) error {
	if len(a.keyFile) != 0 {
		return errors.New("key file already set")
	}
	a.keyFile = path
	return nil
}

func (a *App) setCaCert(path string) error {
	if a.tlsCfg.RootCAs != nil {
		return errors.New("cacert already set")
	}

	pem, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	a.tlsCfg.RootCAs = x509.NewCertPool()
	if !a.tlsCfg.RootCAs.AppendCertsFromPEM(pem) {
		return errors.New("unable to load CA certs")
	}
	a.tlsCfg.ClientCAs = a.tlsCfg.RootCAs
	return nil
}

func (a *App) getOptions() []*optopia.Option {
	return []*optopia.Option{
		{
			Long:  "verbose",
			Short: 'v',
			Help:  "Increase verbosity",
			Handle: func(string) error {
				a.verbose++
				return nil
			},
		},
		{
			Long:  "silent",
			Short: 'q',
			Help:  "Decrease verbosity",
			Handle: func(string) error {
				a.verbose--
				return nil
			},
		},
		{
			Long: "push",
			Help: "Use PUSH socket type",
			Handle: func(string) error {
				return a.setSocket(push.NewSocket)
			},
		},
		{
			Long: "pull",
			Help: "Use PULL socket type",
			Handle: func(string) error {
				return a.setSocket(pull.NewSocket)
			},
		},
		{
			Long: "pub",
			Help: "Use PUB socket type",
			Handle: func(string) error {
				return a.setSocket(pub.NewSocket)
			},
		},
		{
			Long: "sub",
			Help: "Use SUB socket type",
			Handle: func(string) error {
				return a.setSocket(sub.NewSocket)
			},
		},
		{
			Long: "req",
			Help: "Use REQ socket type",
			Handle: func(string) error {
				return a.setSocket(req.NewSocket)
			},
		},
		{
			Long: "rep",
			Help: "Use REP socket type",
			Handle: func(string) error {
				return a.setSocket(rep.NewSocket)
			},
		},
		{
			Long: "surveyor",
			Help: "Use SURVEYOR socket type",
			Handle: func(string) error {
				return a.setSocket(surveyor.NewSocket)
			},
		},
		{
			Long: "respondent",
			Help: "Use RESPONDENT socket type",
			Handle: func(string) error {
				return a.setSocket(respondent.NewSocket)
			},
		},
		{
			Long: "bus",
			Help: "Use BUS socket type",
			Handle: func(string) error {
				return a.setSocket(bus.NewSocket)
			},
		},
		{
			Long: "pair",
			Help: "Use PAIR socket type",
			Handle: func(string) error {
				return a.setSocket(pair.NewSocket)
			},
		},
		{
			Long: "star",
			Help: "Use STAR socket type",
			Handle: func(string) error {
				return a.setSocket(star.NewSocket)
			},
		},
		{
			Long:    "bind",
			Help:    "Bind socket to ADDR",
			ArgName: "ADDR",
			HasArg:  true,
			Handle:  a.addBind,
		},
		{
			Long:    "connect",
			Help:    "Connect socket to ADDR",
			ArgName: "ADDR",
			HasArg:  true,
			Handle:  a.addDial,
		},
		{
			Long:    "bind-ipc",
			Short:   'X',
			Help:    "Bind socket to IPC PATH",
			ArgName: "PATH",
			HasArg:  true,
			Handle:  a.addBindIPC,
		},
		{
			Long:    "connect-ipc",
			Short:   'x',
			Help:    "Connect socket to IPC PATH",
			ArgName: "PATH",
			HasArg:  true,
			Handle:  a.addDialIPC,
		},
		{
			Long:    "bind-local",
			Short:   'L',
			Help:    "Bind socket to localhost PORT",
			ArgName: "PORT",
			HasArg:  true,
			Handle:  a.addBindLocal,
		},
		{
			Long:    "connect-local",
			Short:   'l',
			Help:    "Connect socket to localhost PATH",
			ArgName: "PORT",
			HasArg:  true,
			Handle:  a.addDialLocal,
		},
		{
			Long:    "subscribe",
			Help:    "Subscribe to PREFIX (default is wildcard)",
			ArgName: "PREFIX",
			HasArg:  true,
			Handle:  a.addSub,
		},
		{
			Long:    "count",
			ArgName: "COUNT",
			HasArg:  true,
			ArgP:    &a.count,
			Help:    "Repeat COUNT times",
			Handle: func(string) error {
				a.countSet = true
				return nil
			},
		},
		{
			Long:    "recv-timeout",
			Help:    "Set receive timeout",
			ArgName: "DUR",
			HasArg:  true,
			ArgP:    &a.recvTimeout,
		},
		{
			Long:    "send-timeout",
			Help:    "Set send timeout",
			ArgName: "DUR",
			HasArg:  true,
			ArgP:    &a.sendTimeout,
		},
		{
			Long:    "send-delay",
			Short:   'd',
			Help:    "Set send delay",
			ArgName: "DUR",
			HasArg:  true,
			ArgP:    &a.sendDelay,
		},
		{
			Long:    "send-interval",
			Short:   'i',
			Help:    "Set send interval",
			ArgName: "DUR",
			HasArg:  true,
			ArgP:    &a.sendInterval,
			Handle: func(string) error {
				if !a.countSet {
					a.count = -1
				}
				return nil
			},
		},
		{
			Long: "raw",
			Help: "Raw output, no delimiters",
			Handle: func(string) error {
				return a.setFormat("raw")
			},
		},
		{
			Long:  "ascii",
			Short: 'A',
			Help:  "ASCII output, one per line",
			Handle: func(string) error {
				return a.setFormat("ascii")
			},
		},
		{
			Long:  "quoted",
			Short: 'Q',
			Help:  "quoted output, one per line",
			Handle: func(string) error {
				return a.setFormat("quoted")
			},
		},
		{
			Long: "msgpack",
			Help: "MsgPack binary output (see msgpack.org)",
			Handle: func(string) error {
				return a.setFormat("msgpack")
			},
		},
		{
			Long:   "format",
			Help:   "Set output format to FORMAT",
			Handle: a.setFormat,
			HasArg: true,
		},
		{
			Long:    "data",
			Short:   'D',
			Help:    "Data to send",
			ArgName: "DATA",
			HasArg:  true,
			Handle:  a.setSendData,
		},
		{
			Long:    "file",
			Short:   'F',
			Help:    "Send contents of FILE",
			ArgName: "FILE",
			HasArg:  true,
			Handle:  a.setSendFile,
		},
		{
			Long:    "cert",
			Short:   'E',
			Help:    "Use self certificate in FILE for TLS",
			ArgName: "FILE",
			HasArg:  true,
			Handle:  a.setCert,
		},
		{
			Long:    "key",
			Help:    "Use private key in FILE for TLS",
			ArgName: "FILE",
			HasArg:  true,
			Handle:  a.setKey,
		},
		{
			Long:    "cacert",
			Help:    "Use CA certificate(s) in FILE for TLS",
			ArgName: "FILE",
			HasArg:  true,
			Handle:  a.setCaCert,
		},
		{
			Long:  "insecure",
			Short: 'k',
			Help:  "Do not validate TLS peer",
			Handle: func(string) error {
				a.noVerifyTLS = true
				return nil
			},
		},
	}
}

// Initialize initializes an instance of the app.
func (a *App) Initialize() {
	a.stdOut = os.Stdout
	a.sock = nil
	a.recvTimeout = Duration(-1)
	a.sendTimeout = Duration(-1)
	a.sendInterval = Duration(-1)
	a.sendDelay = Duration(-1)
	a.count = 1
	a.options = &optopia.Options{}
	opts := a.getOptions()
	mustSucceed(a.options.Add(opts...))
}

/*
The macat command is a command-line interface to send and receive
data via the mangos implementation of the SP (nanomsg) protocols.  It is
designed to be suitable for use as a drop-in replacement for nanocat(1).`

Summary = "command line interface to the mangos messaging library"
*/

// Help returns a help string.
func (a *App) Help() string {
	return a.options.Help()
}

func (a *App) printMsg(msg *mangos.Message) {
	if a.printFormat == "no" {
		return
	}
	bw := bufio.NewWriter(a.stdOut)
	switch a.printFormat {
	case "raw":
		_, _ = bw.Write(msg.Body)
	case "ascii":
		for i := 0; i < len(msg.Body); i++ {
			if strconv.IsPrint(rune(msg.Body[i])) {
				_ = bw.WriteByte(msg.Body[i])
			} else {
				_ = bw.WriteByte('.')
			}
		}
		_, _ = bw.WriteString("\n")
	case "quoted":
		for i := 0; i < len(msg.Body); i++ {
			switch msg.Body[i] {
			case '\n':
				_, _ = bw.WriteString("\\n")
			case '\r':
				_, _ = bw.WriteString("\\r")
			case '\\':
				_, _ = bw.WriteString("\\\\")
			case '"':
				_, _ = bw.WriteString("\\\"")
			default:
				if strconv.IsPrint(rune(msg.Body[i])) {
					_ = bw.WriteByte(msg.Body[i])
				} else {
					_, _ = bw.WriteString(fmt.Sprintf("\\x%02x",
						msg.Body[i]))
				}
			}
		}
		_, _ = bw.WriteString("\n")

	case "msgpack":
		enc := make([]byte, 5)
		switch {
		case len(msg.Body) < 256:
			enc = enc[:2]
			enc[0] = 0xc4
			enc[1] = byte(len(msg.Body))

		case len(msg.Body) < 65536:
			enc = enc[:3]
			enc[0] = 0xc5
			binary.BigEndian.PutUint16(enc[1:], uint16(len(msg.Body)))
		default:
			enc = enc[:5]
			enc[0] = 0xc6
			binary.BigEndian.PutUint32(enc[1:], uint32(len(msg.Body)))
		}
		_, _ = bw.Write(enc)
		_, _ = bw.Write(msg.Body)
	}
	_ = bw.Flush()
}

func (a *App) recvLoop() error {
	sock := a.sock
	for {
		msg, err := sock.RecvMsg()
		switch err {
		case mangos.ErrProtoState:
			return nil // Survey completion
		case mangos.ErrRecvTimeout:
			return nil
		case nil:
		default:
			return fmt.Errorf("recv: %v", err)
		}
		a.printMsg(msg)
		msg.Free()
	}
}

func (a *App) sendLoop() error {
	sock := a.sock
	count := a.count
	if a.sendData == nil {
		return errors.New("no data to send")
	}
	for {
		switch count {
		case -1:
		case 0:
			return nil
		default:
			count--
		}
		msg := mangos.NewMessage(len(a.sendData))
		msg.Body = append(msg.Body, a.sendData...)
		err := sock.SendMsg(msg)

		if err != nil {
			return fmt.Errorf("send: %v", err)
		}

		if a.sendInterval >= 0 && count != 0 {
			time.Sleep(time.Duration(a.sendInterval))
		}
	}
}

func (a *App) sendRecvLoop() error {
	sock := a.sock
	count := a.count
	for {
		switch count {
		case -1:
		case 0:
			return nil
		default:
			count--
		}

		msg := mangos.NewMessage(len(a.sendData))
		msg.Body = append(msg.Body, a.sendData...)
		err := sock.SendMsg(msg)

		if err != nil {
			return fmt.Errorf("send: %v", err)
		}

		if a.sendInterval < 0 {
			a.count++
			return a.recvLoop()
		}

		now := time.Now()

		// maximum wait time is upper bound of recvTimeout and
		// sendInterval

		if a.recvTimeout < 0 || a.recvTimeout > a.sendInterval {
			_ = sock.SetOption(mangos.OptionRecvDeadline,
				time.Duration(a.sendInterval))
		}
		msg, err = sock.RecvMsg()
		switch err {
		case mangos.ErrProtoState:
		case mangos.ErrRecvTimeout:
		case nil:
			a.printMsg(msg)
			msg.Free()
		default:
			return fmt.Errorf("recv: %v", err)
		}
		if count != 0 {
			time.Sleep(time.Duration(a.sendInterval) - time.Since(now))
		}
	}
}

func (a *App) replyLoop() error {
	sock := a.sock
	if a.sendData == nil {
		return a.recvLoop()
	}
	for {
		msg, err := sock.RecvMsg()
		switch err {
		case mangos.ErrRecvTimeout:
			return nil
		case nil:
		default:
			return fmt.Errorf("recv: %v", err)
		}
		a.printMsg(msg)
		msg.Free()

		msg = mangos.NewMessage(len(a.sendData))
		msg.Body = append(msg.Body, a.sendData...)
		err = sock.SendMsg(msg)

		if err != nil {
			return fmt.Errorf("send: %v", err)
		}
	}
}

func (a *App) cleanup() {
	if a.sock != nil {
		time.Sleep(time.Millisecond * 20) // for draining
		_ = a.sock.Close()
	}
}

// Run runs the instance of the application.
func (a *App) Run(args ...string) error {

	defer a.cleanup()

	args, e := a.options.Parse(args)
	if e != nil {
		return e
	}
	if len(args) > 0 {
		return fmt.Errorf("usage: extra arguments")
	}

	if a.certFile != "" {
		if a.keyFile == "" {
			a.keyFile = a.certFile
		}
		c, e := tls.LoadX509KeyPair(a.certFile, a.keyFile)
		if e != nil {
			return fmt.Errorf("failed loading cert/key: %v", e)
		}
		a.tlsCfg.Certificates = make([]tls.Certificate, 0, 1)
		a.tlsCfg.Certificates = append(a.tlsCfg.Certificates, c)
	}
	if a.tlsCfg.RootCAs != nil {
		a.tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
		a.tlsCfg.InsecureSkipVerify = false
	} else {
		a.tlsCfg.ClientAuth = tls.NoClientCert
		a.tlsCfg.InsecureSkipVerify = a.noVerifyTLS
	}

	if a.sock == nil {
		return errors.New("protocol not specified")
	}

	if len(a.bindAddr) == 0 && len(a.dialAddr) == 0 {
		return errors.New("no address specified")
	}

	if a.sock.Info().Self != mangos.ProtoSub {
		if len(a.subscriptions) > 0 {
			return errors.New("subscription only valid with SUB protocol")
		}
	} else {
		if len(a.subscriptions) > 0 {
			for i := range a.subscriptions {
				err := a.sock.SetOption(mangos.OptionSubscribe,
					[]byte(a.subscriptions[i]))
				mustSucceed(err)
			}
		} else {
			err := a.sock.SetOption(mangos.OptionSubscribe, []byte{})
			mustSucceed(err)
		}
	}

	for _, addr := range a.bindAddr {
		var opts = make(map[string]interface{})

		// TLS addresses require a certificate to be supplied.
		if strings.HasPrefix(addr, "tls") ||
			strings.HasPrefix(addr, "wss") {
			if len(a.tlsCfg.Certificates) == 0 {
				return errors.New("no server cert specified")
			}
			opts[mangos.OptionTLSConfig] = &a.tlsCfg
		}
		err := a.sock.ListenOptions(addr, opts)
		if err != nil {
			return fmt.Errorf("bind(%s): %v", addr, err)
		}
	}

	for _, addr := range a.dialAddr {
		var opts = make(map[string]interface{})

		if strings.HasPrefix(addr, "tls") ||
			strings.HasPrefix(addr, "wss") {
			if a.tlsCfg.RootCAs == nil && !a.noVerifyTLS {
				return errors.New("no CA cert specified")
			}
			opts[mangos.OptionTLSConfig] = &a.tlsCfg
		}
		err := a.sock.DialOptions(addr, opts)
		if err != nil {
			return fmt.Errorf("dial(%s): %v", addr, err)
		}
	}

	// XXX: ugly hack - work around TCP slow start
	time.Sleep(time.Millisecond * 20)
	time.Sleep(time.Duration(a.sendDelay))

	if dur := time.Duration(a.recvTimeout); dur >= 0 {
		_ = a.sock.SetOption(mangos.OptionRecvDeadline, dur)
	}
	if dur := time.Duration(a.sendTimeout); dur >= 0 {
		_ = a.sock.SetOption(mangos.OptionSendDeadline, dur)
	}

	// Start main processing
	switch a.sock.Info().Self {

	case mangos.ProtoPull:
		fallthrough
	case mangos.ProtoSub:
		return a.recvLoop()

	case mangos.ProtoPush:
		fallthrough
	case mangos.ProtoPub:
		return a.sendLoop()

	case mangos.ProtoPair:
		fallthrough
	case mangos.ProtoStar:
		fallthrough
	case mangos.ProtoBus:
		if a.sendData != nil {
			return a.sendRecvLoop()
		}
		return a.recvLoop()

	case mangos.ProtoSurveyor:
		fallthrough
	case mangos.ProtoReq:
		return a.sendRecvLoop()

	case mangos.ProtoRep:
		fallthrough
	case mangos.ProtoRespondent:
		return a.replyLoop()

	default:
		return errors.New("unknown protocol")
	}
}
