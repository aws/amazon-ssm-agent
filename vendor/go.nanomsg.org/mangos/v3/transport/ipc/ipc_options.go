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

const (
	// OptionIpcSocketPermissions is used to set the permissions on the
	// UNIX domain socket via chmod.  The argument is a uint32, and
	// represents the mode passed to chmod().  This is
	// done on the server side.  Be aware that relying on
	// socket permissions for enforcement is not portable.
	OptionIpcSocketPermissions = "UNIX-IPC-CHMOD"

	// OptionIpcSocketOwner is used to set the socket owner by
	// using chown on the server socket.  This will only work if
	// the process has permission.   The argument is an int.
	// If this fails to set at socket creation time,
	// no error is reported.
	OptionIpcSocketOwner = "UNIX-IPC-OWNER"

	// OptionIpcSocketGroup is used to set the socket group by
	// using chown on the server socket.  This will only work if
	// the process has permission.   The argument is an int.
	// If this fails to set at socket creation time,
	// no error is reported.
	OptionIpcSocketGroup = "UNIX-IPC-GROUP"

	// OptionSecurityDescriptor represents a Windows security
	// descriptor in SDDL format (string).  This can only be set on
	// a Listener, and must be set before the Listen routine
	// is called.
	OptionSecurityDescriptor = "WIN-IPC-SECURITY-DESCRIPTOR"

	// OptionInputBufferSize represents the Windows Named Pipe
	// input buffer size in bytes (type int32).  Default is 4096.
	// This is only for Listeners, and must be set before the
	// Listener is started.
	OptionInputBufferSize = "WIN-IPC-INPUT-BUFFER-SIZE"

	// OptionOutputBufferSize represents the Windows Named Pipe
	// output buffer size in bytes (type int32).  Default is 4096.
	// This is only for Listeners, and must be set before the
	// Listener is started.
	OptionOutputBufferSize = "WIN-IPC-OUTPUT-BUFFER-SIZE"
)

