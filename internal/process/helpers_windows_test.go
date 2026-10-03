package process

import (
	"fmt"
	"golang.org/x/sys/windows"
	"os"
)

func terminalSize() string {
	var info windows.ConsoleScreenBufferInfo
	if e := windows.GetConsoleScreenBufferInfo(windows.Handle(os.Stdout.Fd()), &info); e != nil {
		return e.Error()
	}
	return fmt.Sprintf("%dx%d", info.Window.Right-info.Window.Left+1, info.Window.Bottom-info.Window.Top+1)
}
func processAlive(pid int) bool {
	h, e := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if e != nil {
		return false
	}
	defer windows.CloseHandle(h)
	v, _ := windows.WaitForSingleObject(h, 0)
	return v == uint32(windows.WAIT_TIMEOUT)
}
