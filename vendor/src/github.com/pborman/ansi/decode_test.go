// Copyright 2014 Google Inc.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ansi

import (
	"reflect"
	"strings"
	"testing"
)

func TestDecode(t *testing.T) {
	for _, tt := range []struct {
		in  string
		rem string
		lu  *Sequence
		out *S
		s   string // what out.String should return, if not in.
		err error
	}{
		{
			in: "abc",
			out: &S{
				Code: "abc",
			},
		},
		{
			in: "\033[1;23;456h",
			out: &S{
				Code: "\033[h",
				Type: "CSI",
				Params: []string{
					"1",
					"23",
					"456",
				},
			},
			lu: &SM_,
		},
		{
			in: "\033B",
			out: &S{
				Code: "\033B",
				Type: "C1",
			},
			lu: &BPH_,
		},
		{
			in: "\202", // One byte version of "\033B"
			s:  "\033B",
			out: &S{
				Code: "\033B",
				Type: "C1",
			},
			lu: &BPH_,
		},
		{
			in: "\033[1;23;456",
			out: &S{
				Code: "\033[",
				Type: "CSI",
				Params: []string{
					"1",
					"23",
					"456",
				},
			},
			lu:  &CSI_,
			err: IncompleteCSI,
		},
		{
			in: "\033[1;23;456 ",
			out: &S{
				Code: "\033[ ",
				Type: "CSI",
				Params: []string{
					"1",
					"23",
					"456",
				},
			},
			err: IncompleteCSI,
		},
		{
			in: "\033[A",
			s:  "\033[1A",
			out: &S{
				Code:   "\033[A",
				Type:   "CSI",
				Params: []string{"1"},
			},
			lu: &CUU_,
		},
		{
			in: "\033[42A",
			out: &S{
				Code:   "\033[A",
				Type:   "CSI",
				Params: []string{"42"},
			},
			lu: &CUU_,
		},
		{
			in: "\033[4;2A",
			out: &S{
				Code:   "\033[A",
				Type:   "CSI",
				Params: []string{"4", "2"},
			},
			lu:  &CUU_,
			err: ExtraParameters,
		},
		{
			in: "\033[ U",
			out: &S{
				Code:   "\033[ U",
				Type:   "CSI",
				Params: []string{""},
			},
			lu:  &SLH_,
			err: MissingParameters,
		},
		{
			in: "\033[ c",
			out: &S{
				Code:   "\033[ c",
				Type:   "CSI",
				Params: []string{"", "32"},
			},
			lu:  &TCC_,
			err: MissingParameters,
		},
		{
			in: "\033[42 c",
			s:  "\033[42;32 c",
			out: &S{
				Code:   "\033[ c",
				Type:   "CSI",
				Params: []string{"42", "32"},
			},
			lu: &TCC_,
		},
		{
			in: "\033[42;52 c",
			out: &S{
				Code:   "\033[ c",
				Type:   "CSI",
				Params: []string{"42", "52"},
			},
			lu: &TCC_,
		},
		{
			in: "\033[42;52;62 c",
			out: &S{
				Code:   "\033[ c",
				Type:   "CSI",
				Params: []string{"42", "52", "62"},
			},
			lu:  &TCC_,
			err: ExtraParameters,
		},
		{
			in: "\033]string\033\\",
			out: &S{
				Code:   "\033]",
				Type:   "CS",
				Params: []string{"string"},
			},
			lu: &OSC_,
		},
		{
			in:  "\033]string\033\\extra",
			rem: "extra",
			out: &S{
				Code:   "\033]",
				Type:   "CS",
				Params: []string{"string"},
			},
			lu: &OSC_,
		},
		{
			in: "\033]string\033X",
			out: &S{
				Code:   "\033]",
				Type:   "CS",
				Params: []string{"string"},
			},
			err: FoundSOS,
			lu:  &OSC_,
		},
		{
			in:  "\033]string\033Xextra",
			rem: "extra",
			out: &S{
				Code:   "\033]",
				Type:   "CS",
				Params: []string{"string"},
			},
			err: FoundSOS,
			lu:  &OSC_,
		},
		{
			in: "\033]string",
			out: &S{
				Code:   "\033]",
				Type:   "CS",
				Params: []string{"string"},
			},
			err: NoST,
			lu:  &OSC_,
		},
		{
			in: "\033",
			out: &S{
				Code: "\033",
				Type: "C0",
			},
			err: LoneEscape,
			lu:  &ESC_,
		},
		{
			in: "\033\020",
			out: &S{
				Code: "\033\020",
				Type: "ESC",
			},
			err: UnknownEscape,
		},
		{
			in: "\033[?25l",
			out: &S{
				Code:   "\033[l",
				Type:   "CSI",
				Params: []string{"?25"},
			},
			lu: &RM_,
		},
		{
			in:  "\033Nabc",
			rem: "bc",
			out: &S{
				Code:   "\033N",
				Type:   "C1",
				Params: []string{"a"},
			},
			lu: &SS2_,
		},
		{
			in:  "\033Oabc",
			rem: "bc",
			out: &S{
				Code:   "\033O",
				Type:   "C1",
				Params: []string{"a"},
			},
			lu: &SS3_,
		},
		{
			in: "\033O",
			out: &S{
				Code: "\033O",
				Type: "C1",
			},
			lu: &SS3_,
		},
	} {
		remb, out, err := Decode([]byte(tt.in))
		rem := string(remb)
		if rem != tt.rem {
			t.Errorf("%q: got rem %q, want %q", tt.in, rem, tt.rem)
		}
		if err != tt.err {
			t.Errorf("%q: got error %v, want %v", tt.in, err, tt.err)
		}
		if !reflect.DeepEqual(out, tt.out) {
			t.Errorf("%q: got/want\n%#v\n%#v", tt.in, out, tt.out)
		}
		lu := Table[out.Code]
		if lu != tt.lu {
			t.Errorf("%q: got lu %#v, want %#v", tt.in, lu, tt.lu)
		}
		if tt.s == "" {
			tt.s = strings.TrimSuffix(tt.in, tt.rem)
		}
		if err == nil {
			if s := out.String(); s != tt.s {
				t.Errorf("%q: String got %q, want %q", tt.in, s, tt.s)
			}
		}
	}
}

/*
var (
	LoneEscape    = errors.New("escape at end of input")
	UnknownEscape = errors.New("unknown escape sequence")
	NoST          = errors.New("control string missing string terminator")
	FoundSOS      = errors.New("start of string encountered in control string")
	IncompleteCSI = errors.New("incomplete control sequence")
)
*/
