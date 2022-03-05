package ansi

import "testing"

func TestStrip(t *testing.T) {
	for _, tt := range []struct {
		in, out string
		errors  bool
	}{
		{},

		// Make sure control characters are not stripped.
		{in: "abc", out: "abc"},
		{in: "abc\r\n", out: "abc\r\n"},
		{in: "\t", out: "\t"},
		{in: "abc\t", out: "abc\t"},
		{in: "\tabc", out: "\tabc"},
		{in: "abc\tabc", out: "abc\tabc"},

		// Lone escape
		{in: "\033", errors: true},
		{in: "abc\033", out: "abc", errors: true},

		// invalid escape sequence
		{in: "abc\033\r\n", out: "abc\n", errors: true},
		{in: "ab\033\tc", out: "abc", errors: true},
		{in: "abc\033\033\n", out: "abc\n", errors: true},

		// Strip simple escape sequences
		{in: "\033B"},
		{in: "abc\033B", out: "abc"},
		{in: "\033Babc", out: "abc"},
		{in: "a\033Bbc", out: "abc"},
		{in: "a\033Bb\033Bc", out: "abc"},
		{in: "a\033B\033Bbc", out: "abc"},

		// Strip multi-byte CSI escape sequences with no parameters
		{in: "\033[A"},
		{in: "abc\033[A", out: "abc"},
		{in: "\033[Aabc", out: "abc"},
		{in: "a\033[Abc", out: "abc"},
		{in: "a\033[Ab\033[Ac", out: "abc"},
		{in: "a\033[A\033[Abc", out: "abc"},

		// Strip single byte CSI escape sequences
		{in: "\233A"},
		{in: "abc\233A", out: "abc"},
		{in: "\233Aabc", out: "abc"},
		{in: "a\233Abc", out: "abc"},
		{in: "a\233Ab\233Ac", out: "abc"},
		{in: "a\233A\233Abc", out: "abc"},

		// Strip CSI escape sequences with parameters
		{in: "\033[4A"},
		{in: "abc\033[4A", out: "abc"},
		{in: "\033[4Aabc", out: "abc"},
		{in: "a\033[4Abc", out: "abc"},
		{in: "a\033[4Ab\033[4Ac", out: "abc"},
		{in: "a\033[4A\033[4Abc", out: "abc"},
		{in: "a\033[4A\033[4Abc", out: "abc"},

		// Strip CSI escape sequenes with intermediate bytes
		// Strip CSI escape sequence with multiple parameters
		{in: "\033[1;2 Tabc", out: "abc"},

		// Strip SOS escape sequences (start of string)
		{in: "\033_This is an APC string\033\\abc", out: "abc"},

		// SOS without ST
		{in: "\033_This is an incomplete APC string", out: "", errors: true},

		// too many parameters
		{in: "\033[1;2Aabc", out: "abc", errors: true},

		// two few parameters (" T" requires 2)
		{in: "\033[ Tabc", out: "abc", errors: true},
		{in: "\033[1 Tabc", out: "abc", errors: true},
	} {
		bout, err := Strip([]byte(tt.in))
		switch {
		case tt.errors && err == nil:
			t.Errorf("%q: did not get expected error", tt.in)
		case !tt.errors && err != nil:
			t.Errorf("%q: got unexpected error %v", tt.in, err)
		}
		out := string(bout)
		if out != tt.out {
			t.Errorf("%q: got %q, want %q", tt.in, out, tt.out)
		}
	}
}
