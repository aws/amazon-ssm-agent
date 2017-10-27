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

package multiwriter

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"testing"

	"sync"

	"github.com/stretchr/testify/assert"
)

// TestInputCases is a list of strings which we test multi-writer on.
var TestInputCases = [...]string{
	"Test input text.",
	"\b5Ὂg̀9! ℃ᾭG",
	"Lorem ipsum dolor sit amet, consectetur adipiscing elit. In fermentum cursus mi, sed placerat tellus condimentum non. " +
		"Pellentesque vel volutpat velit. Sed eget varius nibh. Sed quis nisl enim. Nulla faucibus nisl a massa fermentum porttitor. " +
		"Integer at massa blandit, congue ligula ut, vulputate lacus. Morbi tempor tellus a tempus sodales. Nam at placerat odio, " +
		"ut placerat purus. Donec imperdiet venenatis orci eu mollis. Phasellus rhoncus bibendum lacus sit amet cursus. Aliquam erat" +
		" volutpat. Phasellus auctor ipsum vel efficitur interdum. Duis sed elit tempor, convallis lacus sed, accumsan mi. Integer" +
		" porttitor a nunc in porttitor. Vestibulum felis enim, pretium vel nulla vel, commodo mollis ex. Sed placerat mollis leo, " +
		"at varius eros elementum vitae. Nunc aliquet velit quis dui facilisis elementum. Etiam interdum lobortis nisi, vitae " +
		"convallis libero tincidunt at. Nam eu velit et velit dignissim aliquet facilisis id ipsum. Vestibulum hendrerit, arcu " +
		"id gravida facilisis, felis leo malesuada eros, non dignissim quam turpis a massa. ",
}

// testReadBulk runs to read the stream in bulk and check if the output matches the source string
func testReadBulk(t *testing.T, stream *io.PipeReader, sourceString string, wg *sync.WaitGroup) {
	stdoutString, _ := ioutil.ReadAll(stream)
	assert.Equal(t, string(stdoutString), sourceString)
	wg.Done()
}

// testReadStream runs to read the stream and check if the output matches the source string
func testReadStream(t *testing.T, stream *io.PipeReader, sourceString string, wg *sync.WaitGroup) {
	// Read byte by byte
	scanner := bufio.NewScanner(stream)
	scanner.Split(bufio.ScanBytes)
	var buffer bytes.Buffer
	for scanner.Scan() {
		buffer.WriteString(scanner.Text())
	}
	assert.Equal(t, buffer.String(), sourceString)
	wg.Done()
}

// testBulkWrite runs to see if the input from the stream matches the string written to multi-writer.
func testBulkWrite(t *testing.T, sourceString string, listeners int) {
	mw := NewDocumentIOMultiWriter()
	for i := 0; i < listeners; i++ {
		r, w := io.Pipe()
		mw.AddWriter(w)
		go testReadBulk(t, r, sourceString, mw.wg)
	}

	bytesWritten, err := mw.Write([]byte(sourceString))
	assert.Equal(t, bytesWritten, len(sourceString))
	assert.Nil(t, err)
	mw.Close()
}

// testStreamWrite runs to see if the input from the stream matches the string written to multi-writer.
func testStreamWrite(t *testing.T, sourceString string, listeners int) {
	mw := NewDocumentIOMultiWriter()
	for i := 0; i < listeners; i++ {
		r, w := io.Pipe()
		mw.AddWriter(w)
		go testReadStream(t, r, sourceString, mw.wg)
	}

	totalbytesWritten := 0
	for i := 0; i < len(sourceString); i++ {
		bytesWritten, err := mw.Write([]byte{sourceString[i]})
		totalbytesWritten += bytesWritten
		assert.Nil(t, err)
	}

	assert.Equal(t, totalbytesWritten, len(sourceString))
	mw.Close()
}

// testBulkWriteString runs to see if the input from the stream matches the string written to multi-writer.
func testBulkWriteString(t *testing.T, sourceString string, listeners int) {
	mw := NewDocumentIOMultiWriter()
	for i := 0; i < listeners; i++ {
		r, w := io.Pipe()
		mw.AddWriter(w)
		go testReadBulk(t, r, sourceString, mw.wg)
	}

	bytesWritten, err := mw.WriteString(sourceString)
	assert.Equal(t, bytesWritten, len(sourceString))
	assert.Nil(t, err)
	mw.Close()
}

// TestWrite runs to see if the input from the stream matches the string written to multi-writer.
func TestWrite(t *testing.T) {
	for _, testInput := range TestInputCases {
		testBulkWrite(t, testInput, 4)
		testStreamWrite(t, testInput, 4)
	}
}

// TestWriteString runs to see if the input from the stream matches the string written to multi-writer.
func TestWriteString(t *testing.T) {
	for _, testInput := range TestInputCases {
		testBulkWriteString(t, testInput, 4)
	}
}

// TestAddWriter runs tests to check AddWriter function.
func TestAddWriter(t *testing.T) {
	mw := NewDocumentIOMultiWriter()
	listeners := 4
	for i := 0; i < listeners; i++ {
		_, w := io.Pipe()
		mw.AddWriter(w)
	}

	assert.Equal(t, listeners, len(mw.writers))
}

// TestCloseWriter runs tests to check Close function.
func TestCloseWriter(t *testing.T) {
	mw := NewDocumentIOMultiWriter()
	listeners := 4
	for i := 0; i < listeners; i++ {
		r, w := io.Pipe()
		mw.AddWriter(w)
		go testReadBulk(t, r, "", mw.wg)
	}
	mw.Close()

	bytesWritten, err := mw.WriteString("")
	assert.Equal(t, bytesWritten, 0)
	assert.Nil(t, err)

}
