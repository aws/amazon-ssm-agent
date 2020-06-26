// Copyright 2014 Google Inc.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ansi

import (
	"bytes"
	"testing"
)

func TestWriter(t *testing.T) {
	for x, tt := range []struct {
		f   func(*Writer)
		out string
	}{
		{
			f:   func(w *Writer) { w.WriteString("test") },
			out: "test",
		},
		{
			f:   func(w *Writer) { w.Bold().WriteString("test") },
			out: "\033[1mtest\033[0m",
		},
		{
			f:   func(w *Writer) { w.Faint().WriteString("test") },
			out: "\033[2mtest\033[0m",
		},

		{
			f:   func(w *Writer) { w.Black().WriteString("test") },
			out: "\033[30mtest\033[0m",
		},
		{
			f:   func(w *Writer) { w.Red().WriteString("test") },
			out: "\033[31mtest\033[0m",
		},
		{
			f:   func(w *Writer) { w.Green().WriteString("test") },
			out: "\033[32mtest\033[0m",
		},
		{
			f:   func(w *Writer) { w.Yellow().WriteString("test") },
			out: "\033[33mtest\033[0m",
		},
		{
			f:   func(w *Writer) { w.Blue().WriteString("test") },
			out: "\033[34mtest\033[0m",
		},
		{
			f:   func(w *Writer) { w.Magenta().WriteString("test") },
			out: "\033[35mtest\033[0m",
		},
		{
			f:   func(w *Writer) { w.Cyan().WriteString("test") },
			out: "\033[36mtest\033[0m",
		},
		{
			f:   func(w *Writer) { w.White().WriteString("test") },
			out: "\033[37mtest\033[0m",
		},
		{
			f:   func(w *Writer) { w.White().Bold().WriteString("test") },
			out: "\033[1;37mtest\033[0m",
		},

		{
			f:   func(w *Writer) { w.SetBackground(Black).WriteString("test") },
			out: "\033[40mtest\033[0m",
		},
		{
			f:   func(w *Writer) { w.SetBackground(Red).WriteString("test") },
			out: "\033[41mtest\033[0m",
		},
		{
			f:   func(w *Writer) { w.SetBackground(Green).WriteString("test") },
			out: "\033[42mtest\033[0m",
		},
		{
			f:   func(w *Writer) { w.SetBackground(Yellow).WriteString("test") },
			out: "\033[43mtest\033[0m",
		},
		{
			f:   func(w *Writer) { w.SetBackground(Blue).WriteString("test") },
			out: "\033[44mtest\033[0m",
		},
		{
			f:   func(w *Writer) { w.SetBackground(Magenta).WriteString("test") },
			out: "\033[45mtest\033[0m",
		},
		{
			f:   func(w *Writer) { w.SetBackground(Cyan).WriteString("test") },
			out: "\033[46mtest\033[0m",
		},
		{
			f:   func(w *Writer) { w.SetBackground(White).WriteString("test") },
			out: "\033[47mtest\033[0m",
		},

		{
			f:   func(w *Writer) { w.Red().Bold().SetBackground(Blue).WriteString("test") },
			out: "\033[1;31;44mtest\033[0m",
		},
		{
			f: func(w *Writer) {
				w = w.Red()
				w.Write([]byte{'t'})
				w.WriteString("es")
				w.Write([]byte{'t'})
			},
			out: "\033[31mtest",
		},
		{
			f: func(w *Writer) {
				w = w.Red()
				w.Write([]byte{'t'})
				w.Black().WriteString("es")
				w.Write([]byte{'t'})
			},
			out: "\033[31mt\033[30mes\033[31mt",
		},
		{
			f: func(w *Writer) {
				w.Red().Bold().Write([]byte{'A'})
				w.Red().Faint().Write([]byte{'B'})
				w.Red().Bold().Write([]byte{'C'})
			},
			out: "\033[1;31mA\033[2mB\033[22;1mC",
		},
	} {
		var buf bytes.Buffer
		w := NewWriter(&buf)
		tt.f(w)
		out := buf.String()
		if out != tt.out {
			t.Errorf("#%d: got %q, want %q", x, out, tt.out)
		}
	}
}

func TestPrint(t *testing.T) {
	// We trust Go to do the write thing with these one line functions, so
	// we just need a small sanity test that they work.

	want := "\x1b[31mhello world\n"

	var buf bytes.Buffer
	w := NewWriter(&buf)
	w.Red().Printf("hello %s\n", "world")
	got := buf.String()
	if want != got {
		t.Errorf("Printf got %q, want %q", got, want)
	}

	buf.Reset()
	w = NewWriter(&buf)
	w.Red().Print("hello", " world\n")
	got = buf.String()
	if want != got {
		t.Errorf("Print got %q, want %q", got, want)
	}

	buf.Reset()
	w = NewWriter(&buf)
	w.Red().Println("hello", "world")
	got = buf.String()
	if want != got {
		t.Errorf("Println got %q, want %q", got, want)
	}
}
