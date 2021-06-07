// +build linux

// Copyright 2020 The Mangos Authors
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

package ipc

import (
	"net"
	"syscall"

	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/transport"
)

func getPeer(c *net.UnixConn, pipe transport.ConnPipe) {
	if sc, err := c.SyscallConn(); err == nil {
		sc.Control(func(fd uintptr) {
			uc, err := syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
			if err == nil {
				pipe.SetOption(mangos.OptionPeerPID, int(uc.Pid))
				pipe.SetOption(mangos.OptionPeerUID, int(uc.Uid))
				pipe.SetOption(mangos.OptionPeerGID, int(uc.Gid))
			}
		})
	}
}
