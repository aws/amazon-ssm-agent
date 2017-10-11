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

// Package multiwriter implements a multi-writer
package multiwriter

import (
	"io"
)

// DocumentIOMultiWriter is a multi-writer with support for close channel.
// This is responsible for creating a fan-out multi-writer which allows you to duplicate
// the writes to all the provided writers.
type DocumentIOMultiWriter interface {
	AddWriter(*io.PipeWriter)
	GetStreamClosedChannel() chan bool
	Write([]byte) (int, error)
	WriteString(string) (int, error)
	Close() error
}

// DefaultDocumentIOMultiWriter is the default implementation of multi-writer.
type DefaultDocumentIOMultiWriter struct {
	writers      []*io.PipeWriter
	streamClosed chan bool
}

// NewDocumentIOMultiWriter creates a new document multi-writer
func NewDocumentIOMultiWriter() (b *DefaultDocumentIOMultiWriter) {
	var w []*io.PipeWriter
	b = &DefaultDocumentIOMultiWriter{w, make(chan bool)}
	return
}

// AddWriter adds a new writer to an existing multi-writer
func (b *DefaultDocumentIOMultiWriter) AddWriter(writer *io.PipeWriter) {
	b.writers = append(b.writers, writer)
}

// GetStreamClosedChannel adds a new writer to an existing multi-writer
func (b *DefaultDocumentIOMultiWriter) GetStreamClosedChannel() chan bool {
	return b.streamClosed
}

// Write is responsible for writing a byte to all the attached pipes.
func (b *DefaultDocumentIOMultiWriter) Write(p []byte) (n int, err error) {
	for _, w := range b.writers {
		n, err = w.Write(p)
		if err != nil {
			return
		}
		if n != len(p) {
			err = io.ErrShortWrite
			return
		}
	}
	return len(p), nil
}

// WriteString is responsible for writing a string to all the attached pipes.
func (b *DefaultDocumentIOMultiWriter) WriteString(message string) (n int, err error) {
	for _, w := range b.writers {
		p := []byte(message)
		n, err = w.Write(p)
		if err != nil {
			return
		}
		if n != len(message) {
			err = io.ErrShortWrite
			return
		}
	}
	return len(message), nil
}

// Close waits for all the writers to be closed.
func (b *DefaultDocumentIOMultiWriter) Close() (err error) {
	for _, w := range b.writers {
		err = w.Close()
		if err != nil {
			return
		}
	}

	for i := 0; i < len(b.writers); i++ {
		<-b.streamClosed
	}
	return
}
