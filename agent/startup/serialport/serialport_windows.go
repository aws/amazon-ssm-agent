// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package serialport implements serial port capabilities
package serialport

import (
	"fmt"
	"os"
	"syscall"
	"time"
	"unsafe"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/startup/model"
)

const (
	kernel32 = "kernel32.dll"
)

type SerialPort struct {
	log        log.T
	kernel32   *syscall.DLL
	dcb        model.Dcb
	handle     syscall.Handle
	fileHandle *os.File
	port       string
}

// NewSerialPort creates a serial port object with predefined parameters.
func NewSerialPort(log log.T, port string) (sp *SerialPort) {
	var dcb model.Dcb
	dcb.DCBlength = uint32(unsafe.Sizeof(dcb))
	dcb.BaudRate = uint32(115200)
	dcb.ByteSize = 8
	dcb.Parity = 0
	dcb.StopBits = 0

	kernel32Loaded := syscall.MustLoadDLL(kernel32)

	return &SerialPort{
		log:        log,
		kernel32:   kernel32Loaded,
		dcb:        dcb,
		handle:     0,
		fileHandle: nil,
		port:       port,
	}
}

// OpenPort opens the serial port which MUST be done before WritePort is called.
func (sp *SerialPort) OpenPort() (err error) {
	var comPortName *uint16
	comPortName, err = syscall.UTF16PtrFromString(sp.port)
	if err != nil {
		sp.log.Errorf("Error occurred while opening serial port: %v", err.Error())
		return err
	}

	// open COM1 port and create a handle.
	sp.handle, err = syscall.CreateFile(
		comPortName,
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		0,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0)
	if err != nil {
		sp.log.Errorf("Error occurred while opening serial port: %v", err.Error())
		return
	}

	sp.fileHandle = os.NewFile(uintptr(sp.handle), sp.port)

	// set communication state with default values.
	var r uintptr
	r, _, err = sp.kernel32.MustFindProc("SetCommState").Call(
		uintptr(sp.handle),
		uintptr(unsafe.Pointer(&sp.dcb)),
	)
	if r == 0 {
		sp.log.Errorf("Error occurred while opening serial port: %v", err.Error())
		return
	}

	return nil
}

// ClosePort closes the serial port, which MUST be done at the end.
func (sp *SerialPort) ClosePort() {
	if sp.fileHandle == nil {
		sp.log.Error("Error occurred while closing serial port: Port must be opened")
		return
	}
	if err := sp.fileHandle.Close(); err != nil {
		sp.log.Errorf("Error occurred while closing serial port: %v", err.Error())
	}
	return
}

// WritePort writes messages to serial port, which is then picked up by ICD in EC2 droplet
// and sent to system log in console.
func (sp *SerialPort) WritePort(message string) {
	sp.log.Infof("Write to serial port: %v", message)
	var done uint32
	formattedMessage := fmt.Sprintf("%v: %v\n", time.Now().UTC().Format("2006/01/02 15:04:05Z"), message)
	if err := syscall.WriteFile(sp.handle, []byte(formattedMessage), &done, nil); err != nil {
		sp.log.Errorf("Error occurred while writing to serial port: %v", err.Error())
	}

	return
}
