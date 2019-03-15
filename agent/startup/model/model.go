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

// Package model provides model definition for startup processor
package model

import (
	"github.com/aws/amazon-ssm-agent/agent/util/interop"
)

// Dcb structure
// http://pinvoke.net/default.aspx/Structures/DCB.html
type Dcb struct {
	DCBlength  uint32
	BaudRate   uint32
	flags      [4]byte
	wReserved  uint16
	XonLim     uint16
	XoffLim    uint16
	ByteSize   byte
	Parity     byte
	StopBits   byte
	XonChar    byte
	XoffChar   byte
	ErrorChar  byte
	EOFChar    byte
	EvtChar    byte
	wReserved1 uint16
}

// DriverInfo represents driver information that is written to console.
type DriverInfo struct {
	Name    string
	Version string
}

// WindowsInfo contains ProductName and BuildLabEx from HKLM:\\SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion.
type WindowsInfo struct {
	ProductName               string `json:"ProductName"`
	BuildLabEx                string `json:"BuildLabEx"`
	CurrentMajorVersionNumber int    `json:"CurrentMajorVersionNumber"`
	CurrentMinorVersionNumber int    `json:"CurrentMinorVersionNumber"`
}

// OperatingSystemInfo contains Version and OperatingSystemSKU from Win32_OperatingSystem.
type OperatingSystemInfo struct {
	Version            string `json:"Version"`
	OperatingSystemSKU int    `json:"OperatingSystemSKU"`
}

// PnpEntity contains DeviceId, Service, Name from Win32_PnpEntity.
type PnpEntity struct {
	DeviceID string `json:"DeviceID"`
	Service  string `json:"Service"`
	Name     string `json:"Name"`
}

// PnpSignedDriver contains Description, DriverVersion from Win32_PnpSignedDriver.
type PnpSignedDriver struct {
	Description   string `json:"Description"`
	DriverVersion string `json:"DriverVersion"`
}

// WindowsDriver contains OriginalFileName and Version from result of Get-WindowsDriver.
type WindowsDriver struct {
	OriginalFileName string `json:"OriginalFileName"`
	Version          string `json:"Version"`
}

// EventLog contains Id, LogName, Level, ProviderName, Message, Properties, and TimeCreated from result of Get-WinEvent
type EventLog struct {
	ID           uint32               `json:"Id"`
	LogName      string               `json:"LogName"`
	Level        uint8                `json:"Level"`
	ProviderName string               `json:"ProviderName"`
	Message      string               `json:"Message"`
	Properties   []EventLogProperties `json:"Properties"`
	TimeCreated  string               `json:"TimeCreated"`
}

// EventLogProperties contains Value used by EventLog struct.
// The value can be any type.
type EventLogProperties struct {
	Value interface{} `json:"Value"`
}

// SPCR table defined.
// See https://msdn.microsoft.com/en-us/library/windows/hardware/dn639132(v=vs.85).aspx
//
func get_struct_SPCR_TABLE() *interop.StructDef {
	sd := interop.NewStructDef()
	sd.AddField("Signature", 4)
	sd.AddField("Length", 4)
	sd.AddField("Revision", 1)
	sd.AddField("Checksum", 1)
	sd.AddField("OEMID", 6)
	sd.AddField("OEMTableID", 8)
	sd.AddField("OEMRevision", 4)
	sd.AddField("CreatorID", 4)
	sd.AddField("CreatorRevision", 4)
	sd.AddField("InterfaceType", 1)
	sd.AddField("Reserved1", 3)

	sd.AddField("AddressSpace", 1)
	sd.AddField("BitWidth", 1)
	sd.AddField("BitOffset", 1)
	sd.AddField("AccessSize", 1)
	sd.AddField("Address", 8)

	sd.AddField("InterruptType", 1)
	sd.AddField("Irq", 1)
	sd.AddField("GSI", 4)
	sd.AddField("BaudRate", 1)
	sd.AddField("Parity", 1)
	sd.AddField("StopBits", 1)
	sd.AddField("FlowControl", 1)
	sd.AddField("TerminalType", 1)
	sd.AddField("Reserved2", 1)
	sd.AddField("PCIDeviceID", 2)
	sd.AddField("PCIVendorID", 2)
	sd.AddField("PCIBusNumber", 1)
	sd.AddField("PCIDeviceNumber", 1)
	sd.AddField("PCIFunctionNumber", 1)
	sd.AddField("PCIFlags", 4)
	sd.AddField("PCISegment", 1)
	sd.AddField("Reserved3", 4)
	return sd
}
