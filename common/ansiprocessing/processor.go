package ansiprocessing

import (
	"bytes"
	"fmt"
	"unicode/utf8"
)

type Processor struct {
	buf          *bytes.Buffer
	output       *bytes.Buffer
	prevChar     string
	cursorOffset int
}

func NewProcessor() *Processor {
	return &Processor{
		buf:    &bytes.Buffer{},
		output: &bytes.Buffer{},
	}
}

func (p *Processor) WriteString(data string) (int, error) {
	n, err := p.buf.WriteString(data)
	if err != nil {
		return n, err
	}

	for p.buf.Len() > 0 {
		result := p.tryProcessEscape()
		if result == 1 {
			continue // Processed, continue loop
		} else if result == -1 {
			break // Incomplete sequence, stop processing
		}
		p.writeNextChar()
	}
	return n, nil
}

func (p *Processor) tryProcessEscape() int {
	data := p.buf.Bytes()
	if len(data) == 0 {
		return 0
	}

	// Check for escape sequences
	if data[0] == ansiEscape[0] {
		consumed := p.handleEscapeSeq(data)
		if consumed > 0 {
			// Colors don't move the cursor, so a space before them still counts.
			if sgrLen(data) != consumed {
				p.prevChar = ""
			}
			p.buf.Next(consumed)
			return 1 // Processed
		}
		if consumed == -1 {
			return -1 // Incomplete sequence
		}
	}

	// Check for control characters
	ch := string(data[0:1])
	if p.isControlChar(ch) {
		p.handleControlChar(ch, data[1:])
		p.buf.Next(1)
		p.prevChar = ""
		return 1 // Processed
	}

	return 0 // Not an escape/control char
}

func (p *Processor) isControlChar(ch string) bool {
	return ch == ansiBell || ch == ansiBackspace || ch == ansiTab ||
		ch == ansiLineFeed || ch == ansiVertTab || ch == ansiFormFeed ||
		ch == ansiReturn || ch == ansiNull || ch == ansiDelete
}

// handleControlChar applies a control character. next holds the bytes that
// follow it, if already buffered.
func (p *Processor) handleControlChar(ch string, next []byte) {
	switch ch {
	case ansiBackspace:
		// Backspace stops at the start of the line, as on a terminal.
		b := p.output.Bytes()
		pos := len(b) - p.cursorOffset
		if pos > lineStart(b, pos) {
			_, size := utf8.DecodeLastRune(b[:pos])
			p.cursorOffset += size
		}
	case ansiReturn:
		if p.prevChar == ansiSpace && isLineWrap(next) {
			// The space wrapped onto a new row; the next character overwrites it.
			p.cursorOffset = 1
		} else {
			b := p.output.Bytes()
			p.cursorOffset = len(b) - lineStart(b, len(b)-p.cursorOffset)
		}
	case ansiLineFeed, ansiVertTab, ansiFormFeed:
		p.output.WriteByte('\n')
		p.cursorOffset = 0
	}
}

func (p *Processor) handleEscapeSeq(data []byte) int {
	if len(data) < 2 {
		return -1
	}

	switch data[1] {
	case '[':
		result := p.parseCSI(data[2:])
		if result == -1 {
			return -1 // Incomplete CSI
		}
		if result == 0 {
			return 2 // Invalid CSI, discard only ESC[
		}
		return result + 2
	case ']':
		result := p.parseOSC(data[2:])
		if result == -1 {
			return -1 // Incomplete OSC
		}
		return result + 2
	default:
		if len(data) < 3 {
			return -1
		}
		return 3
	}
}

func (p *Processor) parseCSI(data []byte) int {
	params := ""
	for i, b := range data {
		if b >= 0x40 && b <= 0x7E {
			p.executeCSI(string(b), params)
			return i + 1
		}
		if b == ansiCancel[0] || b == ansiSubstitute[0] {
			return i + 1
		}
		// Valid CSI parameter bytes: 0x30-0x3F (digits, semicolon, etc.)
		if b < 0x30 || b > 0x3F {
			// Invalid byte in CSI sequence, return 0 to signal invalid
			return 0
		}
		params += string(b)
		// Limit parameter length to prevent buffering too much
		if len(params) > 16 {
			return 0 // Too long, treat as invalid
		}
	}
	return -1
}

func (p *Processor) executeCSI(cmd string, params string) {
	count := 1
	if params != "" {
		fmt.Sscanf(params, "%d", &count)
		if count < 1 {
			count = 1
		}
	}

	b := p.output.Bytes()
	pos := len(b) - p.cursorOffset

	switch cmd {
	case cmdCursorForward:
		// Move cursor forward by count runes, not bytes
		b := p.output.Bytes()
		pos := len(b) - p.cursorOffset
		moved := 0
		for i := 0; i < count && pos < len(b); i++ {
			_, size := utf8.DecodeRune(b[pos:])
			pos += size
			moved += size
		}
		p.cursorOffset -= moved
		if p.cursorOffset < 0 {
			p.cursorOffset = 0
		}
	case cmdCursorBack:
		// Move cursor back by count runes, not bytes, stopping at the line start
		b := p.output.Bytes()
		pos := len(b) - p.cursorOffset
		start := lineStart(b, pos)
		moved := 0
		for i := 0; i < count && pos > start; i++ {
			_, size := utf8.DecodeLastRune(b[:pos])
			pos -= size
			moved += size
		}
		p.cursorOffset += moved
	case cmdDeleteChar:
		// Delete count characters at cursor position
		if pos < len(b) {
			// Calculate end position by counting runes
			endPos := pos
			deleted := 0
			for i := 0; i < count && endPos < len(b); i++ {
				_, size := utf8.DecodeRune(b[endPos:])
				endPos += size
				deleted += size
			}
			p.output.Reset()
			p.output.Write(b[:pos])
			p.output.Write(b[endPos:])
			p.cursorOffset -= deleted
			if p.cursorOffset < 0 {
				p.cursorOffset = 0
			}
		} else {
			// Cursor at the end: drop up to count runes, but only on this line.
			start := lineStart(b, pos)
			end := len(b)
			for i := 0; i < count && end > start; i++ {
				_, size := utf8.DecodeLastRune(b[start:end])
				end -= size
			}
			p.output.Reset()
			p.output.Write(b[:end])
		}
	case cmdEraseInLine:
		if pos >= 0 && pos <= len(b) {
			p.output.Reset()
			p.output.Write(b[:pos])
			p.cursorOffset = 0
		}
	}
}

func (p *Processor) parseOSC(data []byte) int {
	for i, b := range data {
		if b == ansiBell[0] {
			return i + 1
		}
		if i+1 < len(data) && b == ansiEscape[0] && data[i+1] == '\\' {
			return i + 2
		}
	}
	return -1
}

func (p *Processor) writeNextChar() {
	r, size, err := p.buf.ReadRune()
	if err != nil {
		return
	}

	if r == utf8.RuneError && size == 1 {
		return
	}

	p.prevChar = string(r)

	if p.cursorOffset > 0 {
		b := p.output.Bytes()
		pos := len(b) - p.cursorOffset

		// Find the size of the character at cursor position to skip it
		_, skipSize := utf8.DecodeRune(b[pos:])

		p.output.Reset()
		p.output.Write(b[:pos])
		p.output.WriteString(string(r))
		if pos+skipSize < len(b) {
			p.output.Write(b[pos+skipSize:])
		}
		p.cursorOffset -= skipSize
		if p.cursorOffset < 0 {
			p.cursorOffset = 0
		}
	} else {
		p.output.WriteString(string(r))
	}
}

// isLineWrap reports whether a carriage return right after a space is a line
// wrap, given the bytes after the carriage return. A wrap continues with text,
// possibly after colors; a redraw moves the cursor or erases first.
func isLineWrap(next []byte) bool {
	for {
		n := sgrLen(next)
		if n == 0 {
			break
		}
		next = next[n:]
	}
	if len(next) == 0 {
		return true
	}
	return next[0] >= 0x20 && next[0] != 0x7f
}

// sgrLen returns the length of a complete color sequence (ESC [ params m) at
// the start of b, or 0.
func sgrLen(b []byte) int {
	if len(b) < 3 || b[0] != ansiEscape[0] || b[1] != '[' {
		return 0
	}
	for i := 2; i < len(b); i++ {
		switch c := b[i]; {
		case c == 'm':
			return i + 1
		case (c >= '0' && c <= '9') || c == ';':
		default:
			return 0
		}
	}
	return 0
}

// lineStart returns the offset just after the last newline before pos.
func lineStart(b []byte, pos int) int {
	if i := bytes.LastIndexByte(b[:pos], '\n'); i >= 0 {
		return i + 1
	}
	return 0
}

func (p *Processor) GetOutput() []byte {
	return p.output.Bytes()
}

func (p *Processor) Reset() {
	p.output.Reset()
	p.buf.Reset()
	p.prevChar = ""
	p.cursorOffset = 0
}
