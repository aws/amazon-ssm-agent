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
	"fmt"
	"os"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

type IWinPTY interface {
	SetSize(ws_col, ws_row uint32) error
	Close() error
}

//WinPTY contains handlers and pointers needed for launching winpty agent process
type WinPTY struct {
	IWinPTY
	StdIn  *os.File
	StdOut *os.File

	agentConfig   uintptr
	agent         uintptr
	processHandle uintptr
	closed        bool
}

//Start launches winpty agent as a separate process
func Start(winptyDllFilePath, cmdLine string, window_size_cols, window_size_rows uint32, winptyFlag int32) (*WinPTY, error) {

	var winpty WinPTY = WinPTY{}

	loadDll(winptyDllFilePath)
	defineProcedures()

	if err := winpty.configureAgent(window_size_cols, window_size_rows, winptyFlag); err != nil {
		return nil, err
	}

	if err := winpty.startAgent(); err != nil {
		return nil, err
	}

	if err := winpty.getIOPipes(); err != nil {
		return nil, err
	}

	if err := winpty.spawnProcess(cmdLine); err != nil {
		return nil, err
	}

	return &winpty, nil
}

//configureAgent configures agent and sets initial window size.
func (winpty *WinPTY) configureAgent(window_size_cols, window_size_rows uint32, winptyFlag int32) (err error) {
	var errorPtr uintptr
	defer winpty_error_free.Call(errorPtr)

	var lastErr error
	winpty.agentConfig, _, lastErr = winpty_config_new.Call(uintptr(winptyFlag), uintptr(unsafe.Pointer(&errorPtr)))
	if winpty.agentConfig == uintptr(NIL_POINTER_VALUE) {
		return winpty.getFormattedErrorMessage(
			"Unable to configure winpty agent.",
			lastErr,
			errorPtr)
	}

	if window_size_cols == 0 || window_size_rows == 0 {
		return fmt.Errorf(
			"Invalid window console size. Cannot set cols %d and rows %d",
			window_size_cols,
			window_size_rows)
	}
	winpty_config_set_initial_size.Call(
		winpty.agentConfig,
		uintptr(window_size_cols),
		uintptr(window_size_rows))

	return nil
}

//startAgent launches winpty agent.
func (winpty *WinPTY) startAgent() (err error) {
	var errorPtr uintptr
	defer winpty_error_free.Call(errorPtr)

	var lastErr error
	winpty.agent, _, lastErr = winpty_open.Call(winpty.agentConfig, uintptr(unsafe.Pointer(&errorPtr)))
	if winpty.agent == uintptr(NIL_POINTER_VALUE) {
		return winpty.getFormattedErrorMessage(
			"Unable to launch winpty agent.",
			lastErr,
			errorPtr)
	}
	winpty_config_free.Call(winpty.agentConfig)

	return nil
}

//getIOPipes gets handle for stdin and stdout.
func (winpty *WinPTY) getIOPipes() (err error) {
	conin_name, _, lastErr := winpty_conin_name.Call(winpty.agent)
	if conin_name == uintptr(NIL_POINTER_VALUE) {
		return fmt.Errorf(
			"Unable to get conin name. %s",
			lastErr)
	}

	conout_name, _, lastErr := winpty_conout_name.Call(winpty.agent)
	if conout_name == uintptr(NIL_POINTER_VALUE) {
		return fmt.Errorf(
			"Unable to get conout name. %s",
			lastErr)
	}

	conin_handle, err := syscall.CreateFile(
		(*uint16)(unsafe.Pointer(conin_name)),
		CONIN_FILE_ACCESS,
		CREATE_FILE_MODE,
		nil,
		CREATE_FILE_CREATE_MODE,
		CREATE_FILE_ATTRS,
		CREATE_FILE_TEMPLATE)
	if err != nil {
		return fmt.Errorf("Unable to get conin handle. %s", err)
	}
	winpty.StdIn = os.NewFile(uintptr(conin_handle), STDIN_FILE_NAME)

	conout_handle, err := syscall.CreateFile(
		(*uint16)(unsafe.Pointer(conout_name)),
		CONOUT_FILE_ACCESS,
		CREATE_FILE_MODE,
		nil,
		CREATE_FILE_CREATE_MODE,
		CREATE_FILE_ATTRS,
		CREATE_FILE_TEMPLATE)
	if err != nil {
		return fmt.Errorf("Unable to get conout handle. %s", err)
	}
	winpty.StdOut = os.NewFile(uintptr(conout_handle), STDOUT_FILE_NAME)

	return nil
}

//spawnProcess creates a new winpty agent process.
func (winpty *WinPTY) spawnProcess(cmdLine string) (err error) {
	var errorPtr uintptr
	defer winpty_error_free.Call(errorPtr)

	cmdLineUTF16Ptr, err := syscall.UTF16PtrFromString(cmdLine)
	if err != nil {
		return fmt.Errorf("Failed to convert cmd to pointer. %s", err)
	}

	spawnConfig, _, lastErr := winpty_spawn_config_new.Call(
		uintptr(uint64(WINPTY_SPAWN_FLAG_AUTO_SHUTDOWN)),
		uintptr(0),
		uintptr(unsafe.Pointer(cmdLineUTF16Ptr)),
		uintptr(0),
		uintptr(0),
		uintptr(unsafe.Pointer(&errorPtr)))
	if spawnConfig == uintptr(NIL_POINTER_VALUE) {
		return winpty.getFormattedErrorMessage(
			"Unable to create process config.",
			lastErr,
			errorPtr)
	}

	var createProcessErr uint32
	spawnProcess, _, lastErr := winpty_spawn.Call(
		winpty.agent, spawnConfig,
		uintptr(0),
		uintptr(0),
		uintptr(unsafe.Pointer(&createProcessErr)),
		uintptr(unsafe.Pointer(&errorPtr)))
	winpty_spawn_config_free.Call(spawnConfig)

	if spawnProcess == uintptr(NIL_POINTER_VALUE) {
		spawnProcessErrorCode, err := winpty.getWinptyErrorCode(errorPtr)
		if err != nil {
			return fmt.Errorf("Unable to get spawn process error code. %s", err)
		}

		if spawnProcessErrorCode == WINPTY_ERROR_SPAWN_CREATE_PROCESS_FAILED {
			return fmt.Errorf("Unable to create process. %s", getWindowsErrorMessage(createProcessErr))
		} else {
			return winpty.getFormattedErrorMessage(
				"Unable to spawn process.",
				lastErr,
				errorPtr)
		}
	}

	return nil
}

//SetSize sets given console window size.
func (winpty *WinPTY) SetSize(ws_col, ws_row uint32) (err error) {
	var errorPtr uintptr
	defer winpty_error_free.Call(errorPtr)

	if ws_col == 0 || ws_row == 0 {
		return
	}

	sizeSetRet, _, lastErr := winpty_set_size.Call(
		winpty.agent,
		uintptr(ws_col),
		uintptr(ws_row),
		uintptr(unsafe.Pointer(&errorPtr)))
	if sizeSetRet == uintptr(NIL_POINTER_VALUE) {
		return winpty.getFormattedErrorMessage(
			"Unable to set size.",
			lastErr,
			errorPtr)
	}

	return nil
}

//Close closes stdin, stdout and winpty process handle.
func (winpty *WinPTY) Close() (err error) {
	if winpty == nil || winpty.closed {
		return
	}

	winpty_free.Call(winpty.agent)

	if winpty.StdIn != nil {
		if err := winpty.StdIn.Close(); err != nil {
			return fmt.Errorf("Unable to close stdin. %s", err)
		}
	}

	if winpty.StdOut != nil {
		if err := winpty.StdOut.Close(); err != nil {
			return fmt.Errorf("Unable to close stdout. %s", err)
		}
	}

	winpty.closed = true
	return nil
}

//getWinptyErrorMessage returns string error message for given error pointer.
func (winpty *WinPTY) getWinptyErrorMessage(winptyErr uintptr) string {
	winptyErrorMsgPtr, _, lastErr := winpty_error_msg.Call(winptyErr)
	if winptyErrorMsgPtr == uintptr(NIL_POINTER_VALUE) {
		return fmt.Sprintf("Unable to get error message. %s", lastErr)
	}
	return convertUTF16PtrToString((*uint16)(unsafe.Pointer(winptyErrorMsgPtr)))
}

//getWinptyErrorCode gets winpty error code for give error pointer.
func (winpty *WinPTY) getWinptyErrorCode(winptyErr uintptr) (errCode uint32, err error) {
	winptyErrorCodePtr, _, lastErr := winpty_error_code.Call(uintptr(unsafe.Pointer(&winptyErr)))
	if winptyErrorCodePtr == uintptr(NIL_POINTER_VALUE) {
		return 0, winpty.getFormattedErrorMessage(
			"Unable to get error code.",
			lastErr,
			winptyErr)
	}
	return *(*uint32)(unsafe.Pointer(winptyErrorCodePtr)), nil
}

//convertUTF16PtrToString converts utf16 pointer to string
func convertUTF16PtrToString(UTF16Ptr *uint16) string {

	var inputStrChar uint16
	outputStr := make([]uint16, 0)
	inputStrPtr := unsafe.Pointer(UTF16Ptr)

	for {
		inputStrChar = *(*uint16)(inputStrPtr)
		if inputStrChar == 0 {
			return string(utf16.Decode(outputStr))
		}

		outputStr = append(outputStr, inputStrChar)
		inputStrPtr = unsafe.Pointer(uintptr(inputStrPtr) + unsafe.Sizeof(uint16(inputStrChar)))
	}
}

//getWindowsErrorMessage fetches windows error message for given error code
func getWindowsErrorMessage(errorCode uint32) string {
	flags := uint32(windows.FORMAT_MESSAGE_FROM_SYSTEM | windows.FORMAT_MESSAGE_IGNORE_INSERTS)
	langId := uint32(windows.SUBLANG_ENGLISH_US)<<10 | uint32(windows.LANG_ENGLISH)
	buf := make([]uint16, 512)

	_, err := windows.FormatMessage(flags, uintptr(0), errorCode, langId, buf, nil)
	if err != nil {
		return fmt.Sprintf("Unable to fetch windows error message for code %d, err: %s", errorCode, err)
	}
	return strings.TrimSpace(syscall.UTF16ToString(buf))
}

//getFormattedErrorMessage returns formatted error containing custom error string, syscall error and winpty error
func (winpty *WinPTY) getFormattedErrorMessage(errString string, syscallErr error, winptyErr uintptr) (err error) {
	return fmt.Errorf(
		"%s Error returned by syscall: %s Error returned by winpty: %s",
		errString, syscallErr, winpty.getWinptyErrorMessage(winptyErr))
}
