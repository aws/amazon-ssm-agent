// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package interop provides structures and functions for syscall's data structure marshalling.
package interop

import (
	"fmt"
	"unsafe"
)

type Field struct {
	name   string
	offset int
	size   int
}

type StructDef struct {
	fields    []*Field
	fieldsMap map[string]*Field
	size      int
}

func NewStructDef() *StructDef {
	return &StructDef{
		fields:    []*Field{},
		fieldsMap: make(map[string]*Field),
		size:      0,
	}
}

func (sd *StructDef) AddField(name string, size int) {
	field := &Field{name, sd.size, size}
	sd.fields = append(sd.fields, field)
	sd.fieldsMap[name] = field
	sd.size += size
}

func (sd *StructDef) GetSize() int {
	return sd.size
}

func (sd *StructDef) GetUint(data []byte, name string) uint32 {
	field := sd.fieldsMap[name]

	if field.size == 1 {
		value := *(*uint8)(unsafe.Pointer(&data[field.offset]))
		return (uint32)(value)
	}

	if field.size == 2 {
		value := *(*uint16)(unsafe.Pointer(&data[field.offset]))
		return (uint32)(value)
	}

	if field.size == 4 {
		return *(*uint32)(unsafe.Pointer(&data[field.offset]))
	}

	panic(fmt.Sprintf("Struct field size is not supported for int type"))
}

func UnsafeMemoryToString(pointer unsafe.Pointer, size int) string {
	st := []byte{}
	for i := 0; i < size; i++ {
		pointer1 := unsafe.Pointer(uintptr(pointer) + uintptr(i))
		st = append(st, *(*byte)(pointer1))
	}
	return string(st[:])
}

func Uint32AsString(value uint32) string {
	return UnsafeMemoryToString(unsafe.Pointer(&value), 4)
}
