package process

import (
	"fmt"
	"golang.org/x/sys/unix"
	"os"
	"strings"
)

func terminalSize() string {
	s, e := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if e != nil {
		return e.Error()
	}
	return fmt.Sprintf("%dx%d", s.Col, s.Row)
}
func processAlive(pid int) bool {
	b, e := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if e != nil {
		return false
	}
	_, rest, _ := strings.Cut(string(b), ") ")
	return !strings.HasPrefix(rest, "Z ") && unix.Kill(pid, 0) == nil
}
