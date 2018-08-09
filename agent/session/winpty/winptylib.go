// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.
//
// +build windows

//winpty package is wrapper package for calling procedures of winpty.dll
package winpty

import (
	"syscall"
)

const (
	WINPTY_SPAWN_FLAG_AUTO_SHUTDOWN                 = 1
	NIL_POINTER_VALUE                               = 0
	DEFAULT_WINPTY_FLAGS                            = 0
	WINPTY_FLAG_IMPERSONATE_THREAD                  = 0x10
	CREATE_FILE_CREATE_MODE                         = syscall.OPEN_EXISTING
	CREATE_FILE_MODE                                = 0
	CREATE_FILE_ATTRS                               = 0
	CREATE_FILE_TEMPLATE                            = 0
	CONIN_FILE_ACCESS                               = syscall.GENERIC_WRITE
	CONOUT_FILE_ACCESS                              = syscall.GENERIC_READ
	STDIN_FILE_NAME                                 = "stdin"
	STDOUT_FILE_NAME                                = "stdout"
	WINPTY_ERROR_SPAWN_CREATE_PROCESS_FAILED uint32 = 2
)

var (
	winpty_error_code,
	winpty_error_msg,
	winpty_error_free,
	winpty_config_new,
	winpty_config_free,
	winpty_config_set_initial_size,
	winpty_open,
	winpty_conin_name,
	winpty_conout_name,
	winpty_spawn_config_new,
	winpty_spawn_config_free,
	winpty_spawn,
	winpty_set_size,
	winpty_free *syscall.LazyProc
)

var winptyModule *syscall.LazyDLL

//loadDll gets lazydll for winpty.dll which gets loaded once it's procedures are called
func loadDll(winptyDllFilePath string) {
	winptyModule = syscall.NewLazyDLL(winptyDllFilePath)
}

//defineProcedures gets lazyproc for winpty.dll procedures
func defineProcedures() {

	// Error handling.
	winpty_error_code = winptyModule.NewProc("winpty_error_code")
	winpty_error_msg = winptyModule.NewProc("winpty_error_msg")
	winpty_error_free = winptyModule.NewProc("winpty_error_free")

	// Configuration of a new agent.
	winpty_config_new = winptyModule.NewProc("winpty_config_new")
	winpty_config_free = winptyModule.NewProc("winpty_config_free")
	winpty_config_set_initial_size = winptyModule.NewProc("winpty_config_set_initial_size")

	// Start the agent.
	winpty_open = winptyModule.NewProc("winpty_open")

	// I/O Pipes
	winpty_conin_name = winptyModule.NewProc("winpty_conin_name")
	winpty_conout_name = winptyModule.NewProc("winpty_conout_name")

	// Agent RPC Calls
	winpty_spawn_config_new = winptyModule.NewProc("winpty_spawn_config_new")
	winpty_spawn_config_free = winptyModule.NewProc("winpty_spawn_config_free")
	winpty_spawn = winptyModule.NewProc("winpty_spawn")
	winpty_set_size = winptyModule.NewProc("winpty_set_size")
	winpty_free = winptyModule.NewProc("winpty_free")
}
