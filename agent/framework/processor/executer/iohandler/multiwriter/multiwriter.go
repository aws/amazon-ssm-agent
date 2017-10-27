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
	"fmt"
	"io"
	"sync"
)

// DocumentIOMultiWriter is a multi-writer with support for close channel.
// This is responsible for creating a fan-out multi-writer which allows you to duplicate
// the writes to all the provided writers.
type DocumentIOMultiWriter interface {
	AddWriter(*io.PipeWriter)
	GetWaitGroup() *sync.WaitGroup
	Write([]byte) (int, error)
	WriteString(string) (int, error)
	Close() error
}

// DefaultDocumentIOMultiWriter is the default implementation of multi-writer.
type DefaultDocumentIOMultiWriter struct {
	writers []*io.PipeWriter
	wg      *sync.WaitGroup
}

// NewDocumentIOMultiWriter creates a new document multi-writer
func NewDocumentIOMultiWriter() (b *DefaultDocumentIOMultiWriter) {
	var w []*io.PipeWriter
	b = &DefaultDocumentIOMultiWriter{w, new(sync.WaitGroup)}
	return
}

// AddWriter adds a new writer to an existing multi-writer
func (b *DefaultDocumentIOMultiWriter) AddWriter(writer *io.PipeWriter) {
	b.writers = append(b.writers, writer)
	b.wg.Add(1)
}

// GetStreamClosedChannel adds a new writer to an existing multi-writer
func (b *DefaultDocumentIOMultiWriter) GetWaitGroup() *sync.WaitGroup {
	return b.wg
}

// Write is responsible for writing a byte to all the attached pipes.
func (b *DefaultDocumentIOMultiWriter) Write(p []byte) (n int, err error) {
	if len(b.writers) == 0 {
		return 0, fmt.Errorf("No writers present.")
	}

	for i := 0; i < len(b.writers); i++ {
		n, err = b.writers[i].Write(p)
		// TODO: Handler other error types and close the writers after a fixed number of retries
		if err == io.ErrClosedPipe {
			// remove the writer as the reader is closed
			b.writers = append(b.writers[:i], b.writers[i+1:]...)
			i--
		}
		if n != len(p) {
			err = io.ErrShortWrite
		}
	}
	return len(p), nil
}

// WriteString is responsible for writing a string to all the attached pipes.
func (b *DefaultDocumentIOMultiWriter) WriteString(message string) (n int, err error) {
	if len(b.writers) == 0 {
		return 0, fmt.Errorf("No writers present.")
	}

	for _, w := range b.writers {
		p := []byte(message)
		n, err = w.Write(p)
	}
	return len(message), nil
}

// Close waits for all the writers to be closed.
func (b *DefaultDocumentIOMultiWriter) Close() (err error) {
	for i := 0; i < len(b.writers); i++ {
		err = b.writers[i].Close()
	}

	b.wg.Wait()
	return
}
