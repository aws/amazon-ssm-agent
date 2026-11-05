package ansiprocessing

import (
	"bufio"
	"strings"
	"testing"
)

func TestProcessor(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Normal CR", "aa\rbb", "bb"},
		{"Line feed", "line1\nline2", "line1\nline2"},
		{"Remove ANSI", "\x1b[31mRed\x1b[0m", "Red"},
		{"Invalid bytes", "\xff\xfe\xfd\xfc hello", " hello"},
		{"Delete 1 char", "helloWorld\x1b[1P", "helloWorl"},
		{"Delete 2 chars", "helloWorld\x1b[2P", "helloWor"},
		{"Delete then add", "helloWorld\x1b[2Pld", "helloWorld"},
		{"Backspace and erase", "helloWorld\x08\x1b[K", "helloWorl"},
		{"Backspace delete insert", "helloWorld\x08\x08\x08\x08\x1b[1Prld\x08\x08\x08", "helloWrld"},
		{"Erase to end of line", "hello\nWorld\x08\x08\x08\x08\x08\x1b[K", "hello\n"},
		{"CR after space line wrap", "aa \rbb", "aabb"},
		{"CR after space long line", "abcdefghijklmnopqrstuvwxyzabcdefghijklmn \ropqrstu", "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstu"},
		{"Backspace cursor forward overwrite", "helloWorld\x08\x08\x08\x1b[Csld\x08\x08", "helloWorsld"},
		{"Color removed", "text\x1b[31mred\x1b[0m", "textred"},
		{"UTF-8 backspace erase", "hello世界\x08\x08\x1b[K", "hello"},
		{"UTF-8 backspace overwrite", "hello世界\x08\x08ab", "helloab"},
		{"UTF-8 cursor forward overwrite", "hello世界\x08\x08\x1b[Cab", "hello世ab"},
		{"UTF-8 CR overwrite", "hello世界\rtest", "testo世界"},
		{"UTF-8 CR overwrite 2", "世界hello\rtest", "testllo"},
		{"Incomplete ESC", "hello\x1b", "hello"},
		{"Incomplete CSI", "hello\x1b[", "hello"},
		{"Incomplete CSI param", "hello\x1b[3", "hello"},
		{"Incomplete then complete", "hello\x1b[31mred\x1b[0m", "hellored"},
		{"Invalid CSI", "hello\x1b[Xworld", "helloworld"},
		{"Incomplete CSI then text", "test", "test"},
		{"Euro cursor back delete", "€€\x1b[D\x1b[P", "€"},
		{"UTF-8 delete char", "世界\x1b[D\x1b[P", "世"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewProcessor()
			s.WriteString(tt.input)
			output := string(s.GetOutput())
			if output != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, output)
			}
		})
	}
}

func TestProcessor_Reset(t *testing.T) {
	s := NewProcessor()
	s.WriteString("test")
	s.Reset()
	if len(s.GetOutput()) != 0 {
		t.Errorf("Expected empty output after reset, got %q", string(s.GetOutput()))
	}
}

func TestProcessor_IncompleteSequenceCompletion(t *testing.T) {
	s := NewProcessor()
	s.WriteString("hello\x1b[3")
	if string(s.GetOutput()) != "hello" {
		t.Errorf("Expected %q, got %q", "hello", string(s.GetOutput()))
	}
	s.WriteString("1mred\x1b[0m")
	if string(s.GetOutput()) != "hellored" {
		t.Errorf("Expected %q, got %q", "hellored", string(s.GetOutput()))
	}
}

func TestProcessor_IncompleteCSIFollowedByText(t *testing.T) {
	s := NewProcessor()
	s.WriteString("hello\x1b[3")
	if string(s.GetOutput()) != "hello" {
		t.Errorf("After incomplete: expected %q, got %q", "hello", string(s.GetOutput()))
	}
	// When regular text follows incomplete CSI, first char may be consumed as command
	// This is expected behavior - incomplete sequences are ambiguous
	s.WriteString("world")
	output := string(s.GetOutput())
	// 'w' (0x77) is in valid CSI command range, so it terminates the sequence
	// Result: ESC[3w is consumed, leaving "orld"
	if output != "helloorld" {
		t.Errorf("After text: expected %q, got %q", "helloorld", output)
	}
}

func TestProcessor_LineHandling(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Prompt redraw after space", "sh-5.2$ \r\x1b[K\rsh-5.2$ ls", "sh-5.2$ ls"},
		{"Trailing space command redraw", "sh-5.2$ \x1b[7mecho hi \x1b[27m\r\x1b[C\x1b[C\x1b[C\x1b[C\x1b[C\x1b[C\x1b[C\x1b[Cecho hi \n", "sh-5.2$ echo hi \n"},
		{"Line wrap twice", "abcd \refgh \rij", "abcdefghij"},
		{"Line wrap after a color", "abcd \x1b[0m\refgh", "abcdefgh"},
		{"Line wrap into a color", "PS C:\\> Get-Child \r\x1b[33mItem", "PS C:\\> Get-ChildItem"},
		{"Line wrap with wide chars", "世界 \rab", "世界ab"},
		{"Line wrap at end of chunk", "abcd \r", "abcd "},
		{"Backspace stops at line start", "cmd\n\b\b\b\b\x1b[Knext", "cmd\nnext"},
		{"Cursor back stops at line start", "cmd\n\x1b[10D\x1b[Knext", "cmd\nnext"},
		{"Backspace within line still works", "cmd\nabc\b\b\x1b[K", "cmd\na"},
		{"Delete char at end stops at line start", "cmd\n\x1b[10P", "cmd\n"},
		{"Delete char at end within line", "cmd\nabc\x1b[2P", "cmd\na"},
		{"Delete char at end counts runes", "世界\x1b[1P", "世"},
		{"Delete char cannot remove earlier command", "sh-5.2$ echo SECRET-CMD\nSECRET\nsh-5.2$ printf '\\033[45P'\n\x1b[45Psh-5.2$ echo after\n", "sh-5.2$ echo SECRET-CMD\nSECRET\nsh-5.2$ printf '\\033[45P'\nsh-5.2$ echo after\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewProcessor()
			s.WriteString(tt.input)
			if got := string(s.GetOutput()); got != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, got)
			}
		})
	}
}

// customerSessionLog is a shell session recorded by bash with bracketed paste,
// where printf writes backspaces and an erase-in-line after a command.
func customerSessionLog() string {
	fwd := strings.Repeat("\x1b[C", 8)
	cmdLine := func(cmd string) string {
		return "\x1b[?2004hsh-5.2$ \x1b[7m" + cmd + "\x1b[27m\r" + fwd + cmd + "\r\n"
	}
	printfCmd := "printf '" + strings.Repeat(`\b`, 40) + `\033[K'`
	return "\x1b[?2004hsh-5.2$ \r\x1b[K\rsh-5.2$ \x1b[7mecho AUDIT-MARK-1\x1b[27m\r" + fwd + "echo AUDIT-MARK-1\r\n" +
		"\x1b[?2004l\rAUDIT-MARK-1\r\n" +
		cmdLine(printfCmd) +
		"\x1b[?2004l\r" + strings.Repeat("\b", 40) + "\x1b[K" + cmdLine("echo AUDIT-MARK-2") +
		"\x1b[?2004l\rAUDIT-MARK-2\r\n" +
		cmdLine("echo hello") +
		"\x1b[?2004l\rhello\r\n" +
		cmdLine("echo hello ") +
		"\x1b[?2004l\rhello\r\n" +
		"\x1b[?2004hsh-5.2$ "
}

func TestProcessor_SessionLogKeepsEveryCommand(t *testing.T) {
	expectedLines := []string{
		"sh-5.2$ echo AUDIT-MARK-1",
		"AUDIT-MARK-1",
		"sh-5.2$ printf '" + strings.Repeat(`\b`, 40) + `\033[K'`,
		"sh-5.2$ echo AUDIT-MARK-2",
		"AUDIT-MARK-2",
		"sh-5.2$ echo hello",
		"hello",
		"sh-5.2$ echo hello ",
		"hello",
		"sh-5.2$ ",
	}

	// One processor over the whole file, as for the S3 upload.
	s := NewProcessor()
	scanner := bufio.NewScanner(strings.NewReader(customerSessionLog()))
	for scanner.Scan() {
		s.WriteString(scanner.Text())
		s.WriteString("\n")
	}
	whole := strings.Split(strings.TrimSuffix(string(s.GetOutput()), "\n"), "\n")
	if strings.Join(whole, "\n") != strings.Join(expectedLines, "\n") {
		t.Errorf("whole file: expected\n%q\ngot\n%q", expectedLines, whole)
	}

	// A new processor per line, as for CloudWatch streaming.
	var got []string
	scanner = bufio.NewScanner(strings.NewReader(customerSessionLog()))
	for scanner.Scan() {
		s := NewProcessor()
		s.WriteString(scanner.Text())
		if out := string(s.GetOutput()); out != "" {
			got = append(got, out)
		}
	}
	if strings.Join(got, "\n") != strings.Join(expectedLines, "\n") {
		t.Errorf("per line: expected\n%q\ngot\n%q", expectedLines, got)
	}
}
