// +build solaris,cgo

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

	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/transport"
)

// #include <ucred.h>
// #include <stdio.h>
// #include <zone.h>
// typedef struct mycred {
//		pid_t pid;
//		uid_t uid;
//		gid_t gid;
//		zoneid_t zid;
// } mycred_t;
// int getucred(int fd, mycred_t *mc)
// {
//		ucred_t *uc = NULL;
//		if ((getpeerucred(fd, &uc)) != 0) {
//			return (-1);
//		}
//		mc->pid = ucred_getpid(uc);
//		mc->uid = ucred_geteuid(uc);
//		mc->gid = ucred_getegid(uc);
//		mc->zid = ucred_getzoneid(uc);
//		ucred_free(uc);
//
//		return (0);
// }
import "C"

func getPeer(c *net.UnixConn, pipe transport.ConnPipe) {
	if f, err := c.File(); err == nil {
		mc := &C.mycred_t{}
		if C.getucred(C.int(f.Fd()), mc) == 0 {
			pipe.SetOption(mangos.OptionPeerPID, int(mc.pid))
			pipe.SetOption(mangos.OptionPeerUID, int(mc.uid))
			pipe.SetOption(mangos.OptionPeerGID, int(mc.gid))
			pipe.SetOption(mangos.OptionPeerZone, int(mc.zid))
		}
	}
}

// getZone exists to support testing.
func getZone() int {
	return int(C.getzoneid())
}
