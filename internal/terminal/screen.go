// Package terminal provides a bounded, plain-text VT screen approximation.
// It never executes OSC commands, opens URLs, or writes terminal replies.
package terminal

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Snapshot struct {
	Text          string `json:"text"`
	Row           int    `json:"row"`
	Column        int    `json:"column"`
	Columns       int    `json:"columns"`
	Rows          int    `json:"rows"`
	Alternate     bool   `json:"alternate"`
	CursorVisible bool   `json:"cursorVisible"`
}
type Screen struct {
	cells, primary                                                                [][]rune
	col, row, cols, rows, top, bottom, savedCol, savedRow, primaryCol, primaryRow int
	visible, alt, wrap                                                            bool
	state                                                                         byte
	sequence                                                                      []byte
	pending                                                                       []byte
}

func New(cols, rows int) *Screen { s := &Screen{visible: true}; s.Resize(cols, rows); return s }
func (s *Screen) Columns() int   { return s.cols }
func (s *Screen) Rows() int      { return s.rows }
func blank(cols, rows int) [][]rune {
	out := make([][]rune, rows)
	for i := range out {
		out[i] = []rune(strings.Repeat(" ", cols))
	}
	return out
}
func (s *Screen) Resize(cols, rows int) {
	if cols < 1 || cols > 300 || rows < 1 || rows > 100 {
		return
	}
	resize := func(old [][]rune) [][]rune {
		a := blank(cols, rows)
		for r := 0; r < min(rows, len(old)); r++ {
			copy(a[r], old[r])
		}
		return a
	}
	s.cells = resize(s.cells)
	if s.primary != nil {
		s.primary = resize(s.primary)
	}
	s.cols = cols
	s.rows = rows
	s.col = min(s.col, cols-1)
	s.row = min(s.row, rows-1)
	s.top = 0
	s.bottom = rows - 1
	s.wrap = false
}
func (s *Screen) Feed(data []byte) {
	if len(s.pending) > 0 {
		data = append(append([]byte(nil), s.pending...), data...)
		s.pending = nil
	}
	for len(data) > 0 {
		if !utf8.FullRune(data) {
			s.pending = append(s.pending, data...)
			break
		}
		r, n := utf8.DecodeRune(data)
		data = data[n:]
		s.feed(r)
	}
}
func (s *Screen) feed(r rune) {
	switch s.state {
	case 1:
		s.state = 0
		switch r {
		case '[':
			s.state = 2
			s.sequence = nil
		case ']', 'P', '_', '^':
			s.state = 3
		case '(', ')', '*', '+', '#':
			s.state = 5
		case '7':
			s.savedCol = s.col
			s.savedRow = s.row
		case '8':
			s.col = s.savedCol
			s.row = s.savedRow
			s.bound()
		case 'D':
			s.line()
		case 'E':
			s.col = 0
			s.line()
		case 'M':
			if s.row == s.top {
				s.scroll(-1)
			} else {
				s.row = max(0, s.row-1)
			}
		case 'c':
			s.cells = blank(s.cols, s.rows)
			s.col = 0
			s.row = 0
			s.visible = true
			s.alt = false
			s.primary = nil
		}
		return
	case 2:
		if r >= 0x40 && r <= 0x7e {
			s.execute(byte(r))
			s.state = 0
			s.sequence = nil
		} else if len(s.sequence) < 128 && r < 128 {
			s.sequence = append(s.sequence, byte(r))
		} else {
			s.state = 6
		}
		return
	case 3:
		if r == 7 {
			s.state = 0
		} else if r == 27 {
			s.state = 4
		}
		return
	case 4:
		if r == '\\' {
			s.state = 0
		} else {
			s.state = 3
		}
		return
	case 5:
		s.state = 0
		return
	case 6:
		if r >= 0x40 && r <= 0x7e {
			s.state = 0
		}
		return
	}
	switch r {
	case 27:
		s.state = 1
	case '\r':
		s.col = 0
		s.wrap = false
	case '\n', '\v', '\f':
		s.line()
	case '\b':
		s.col = max(0, s.col-1)
		s.wrap = false
	case '\t':
		s.col = min(s.cols-1, (s.col/8+1)*8)
		s.wrap = false
	default:
		if r < 32 || r == 127 || unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) {
			return
		}
		if s.wrap {
			s.col = 0
			s.line()
		}
		s.cells[s.row][s.col] = r
		if s.col == s.cols-1 {
			s.wrap = true
		} else {
			s.col++
		}
	}
}
func (s *Screen) bound() {
	s.row = max(0, min(s.row, s.rows-1))
	s.col = max(0, min(s.col, s.cols-1))
	s.wrap = false
}
func (s *Screen) line() {
	s.wrap = false
	if s.row == s.bottom {
		s.scroll(1)
	} else {
		s.row = min(s.rows-1, s.row+1)
	}
}
func (s *Screen) scroll(n int) {
	n = max(-(s.bottom - s.top + 1), min(n, s.bottom-s.top+1))
	if n > 0 {
		copy(s.cells[s.top:s.bottom+1-n], s.cells[s.top+n:s.bottom+1])
		for i := s.bottom + 1 - n; i <= s.bottom; i++ {
			s.cells[i] = []rune(strings.Repeat(" ", s.cols))
		}
	} else if n < 0 {
		n = -n
		copy(s.cells[s.top+n:s.bottom+1], s.cells[s.top:s.bottom+1-n])
		for i := s.top; i < s.top+n; i++ {
			s.cells[i] = []rune(strings.Repeat(" ", s.cols))
		}
	}
}
func (s *Screen) erase(row, first, last int) {
	for c := max(0, first); c <= min(s.cols-1, last); c++ {
		s.cells[row][c] = ' '
	}
}
func (s *Screen) execute(cmd byte) {
	raw := string(s.sequence)
	private := strings.HasPrefix(raw, "?")
	raw = strings.TrimPrefix(raw, "?")
	parts := strings.Split(raw, ";")
	params := make([]int, len(parts))
	for i, p := range parts {
		n, e := strconv.Atoi(p)
		if e == nil {
			params[i] = max(0, min(n, 10000))
		}
	}
	param := func(i, def int) int {
		if i >= len(params) || params[i] == 0 {
			return def
		}
		return params[i]
	}
	n := param(0, 1)
	if private {
		if cmd != 'h' && cmd != 'l' {
			return
		}
		enable := cmd == 'h'
		for _, p := range params {
			switch p {
			case 25:
				s.visible = enable
			case 47, 1047, 1049:
				if enable && !s.alt {
					s.primary = s.cells
					s.primaryCol = s.col
					s.primaryRow = s.row
					s.cells = blank(s.cols, s.rows)
					s.col = 0
					s.row = 0
					s.alt = true
				} else if !enable && s.alt {
					s.cells = s.primary
					s.primary = nil
					s.col = s.primaryCol
					s.row = s.primaryRow
					s.alt = false
				}
				s.top = 0
				s.bottom = s.rows - 1
				s.bound()
			}
		}
		return
	}
	switch cmd {
	case 'A':
		s.row -= n
	case 'B', 'e':
		s.row += n
	case 'C', 'a':
		s.col += n
	case 'D':
		s.col -= n
	case 'E':
		s.row += n
		s.col = 0
	case 'F':
		s.row -= n
		s.col = 0
	case 'G', '`':
		s.col = n - 1
	case 'd':
		s.row = n - 1
	case 'H', 'f':
		s.row = n - 1
		s.col = param(1, 1) - 1
	case 'J':
		mode := param(0, 0)
		if mode == 2 || mode == 3 {
			for r := range s.rows {
				s.erase(r, 0, s.cols-1)
			}
		} else if mode == 0 {
			s.erase(s.row, s.col, s.cols-1)
			for r := s.row + 1; r < s.rows; r++ {
				s.erase(r, 0, s.cols-1)
			}
		} else if mode == 1 {
			for r := 0; r < s.row; r++ {
				s.erase(r, 0, s.cols-1)
			}
			s.erase(s.row, 0, s.col)
		}
	case 'K':
		switch param(0, 0) {
		case 0:
			s.erase(s.row, s.col, s.cols-1)
		case 1:
			s.erase(s.row, 0, s.col)
		case 2:
			s.erase(s.row, 0, s.cols-1)
		}
	case 'S':
		s.scroll(n)
	case 'T':
		s.scroll(-n)
	case 'L', 'M':
		if s.row >= s.top && s.row <= s.bottom {
			old := s.top
			s.top = s.row
			if cmd == 'L' {
				s.scroll(-n)
			} else {
				s.scroll(n)
			}
			s.top = old
		}
	case '@', 'P', 'X':
		n = min(n, s.cols-s.col)
		row := s.cells[s.row]
		if cmd == '@' {
			copy(row[s.col+n:], row[s.col:])
			s.erase(s.row, s.col, s.col+n-1)
		} else if cmd == 'P' {
			copy(row[s.col:], row[s.col+n:])
			s.erase(s.row, s.cols-n, s.cols-1)
		} else {
			s.erase(s.row, s.col, s.col+n-1)
		}
	case 's':
		s.savedCol = s.col
		s.savedRow = s.row
	case 'u':
		s.col = s.savedCol
		s.row = s.savedRow
	case 'r':
		top, bottom := param(0, 1)-1, param(1, s.rows)-1
		if top >= 0 && bottom < s.rows && top < bottom {
			s.top = top
			s.bottom = bottom
		}
		s.col = 0
		s.row = 0
	}
	s.bound()
}
func (s *Screen) Snapshot() Snapshot {
	lines := make([]string, s.rows)
	for i, line := range s.cells {
		lines[i] = strings.TrimRight(string(line), " ")
	}
	text := strings.TrimRight(strings.Join(lines, "\n"), "\n")
	return Snapshot{text, s.row + 1, s.col + 1, s.cols, s.rows, s.alt, s.visible}
}
