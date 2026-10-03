package terminal

import (
	"strings"
	"testing"
)

func TestScreenOperations(t *testing.T) {
	s := New(80, 20)
	for _, p := range []string{"old\r\nline", "\x1b", "[2J\x1b[HProgress 10%\rProgress 90%\x1b[K\x1b[3;5HPOS\x1b[?25l"} {
		s.Feed([]byte(p))
	}
	x := s.Snapshot()
	if x.Text != "Progress 90%\n\n    POS" || x.Row != 3 || x.Column != 8 || x.CursorVisible {
		t.Fatalf("%+v", x)
	}
	s.Feed([]byte("\x1b[?1049hALT"))
	if !s.Snapshot().Alternate || s.Snapshot().Text != "ALT" {
		t.Fatal("alternate screen")
	}
	s.Resize(90, 30)
	s.Feed([]byte("\x1b[?1049l"))
	if s.Snapshot().Text != x.Text {
		t.Fatal("primary screen lost")
	}
}
func TestParserBoundsAndUTF8(t *testing.T) {
	s := New(20, 5)
	b := []byte("مرحبا🙂")
	for _, c := range b {
		s.Feed([]byte{c})
	}
	if s.Snapshot().Text != "مرحبا🙂" {
		t.Fatalf("split UTF-8: %+v", s.Snapshot())
	}
	for _, p := range []string{"\x1b[" + strings.Repeat("9", 10000) + "A", "\x1b]52;c;" + strings.Repeat("x", 10000) + "\a", "\x1b[999999999999999999999999P", "\x1b[999999999999999999999999L"} {
		s.Feed([]byte(p))
	}
	x := s.Snapshot()
	if x.Row < 1 || x.Row > 5 || x.Column < 1 || x.Column > 20 || len(s.sequence) > 128 {
		t.Fatalf("unbounded parser %+v", x)
	}
}
func FuzzScreen(f *testing.F) {
	for _, p := range []string{"text", "\x1b[2J\x1b[H", "\x1b[?1049h\x1b[?1049l", "🙂"} {
		f.Add([]byte(p))
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			t.Skip()
		}
		s := New(40, 10)
		s.Feed(b)
		s.Resize(20, 5)
		x := s.Snapshot()
		if x.Row < 1 || x.Row > 5 || x.Column < 1 || x.Column > 20 {
			t.Fatal(x)
		}
	})
}
