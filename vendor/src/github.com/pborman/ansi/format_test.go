// Copyright 2014 Google Inc.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ansi

import (
	"bytes"
	"testing"
)

func TestItoa(t *testing.T) {
	for _, tt := range []struct {
		in  int
		out []byte
	}{
		{-1, []byte{}},
		{0, []byte{'0'}},
		{1, []byte{'1'}},
		{12, []byte{'1', '2'}},
		{123, []byte{'1', '2', '3'}},
		{1234, []byte{'1', '2', '3', '4'}},
		{12345, []byte{'1', '2', '3', '4', '5'}},
	} {
		out := itoa(tt.in)
		if !bytes.Equal(out, tt.out) {
			t.Errorf("%d: got %v, want %v", tt.in, out, tt.out)
		}
	}
}

func TestFormat(t *testing.T) {
	for _, tt := range []struct {
		s   *Sequence
		in  []int
		out []byte
	}{
		{s: &CUU_, out: []byte{033, '[', 'A'}},
		{s: &CUU_, in: []int{-1}, out: []byte{033, '[', 'A'}},
		{s: &CUU_, in: []int{42}, out: []byte{033, '[', '4', '2', 'A'}},
		{s: &CUU_, in: []int{3, 2}, out: []byte{033, '[', '3', 'A'}},
		{s: &CUP_, in: []int{1, 2}, out: []byte{033, '[', '1', ';', '2', 'H'}},
	} {
		out := tt.s.Format(tt.in...)
		if !bytes.Equal(out, tt.out) {
			t.Errorf("%s(%v): got %v, want %v", tt.s.Name, tt.in, out, tt.out)
		}
	}
}
