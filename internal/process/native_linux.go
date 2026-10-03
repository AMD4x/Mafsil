package process

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"
)

func launch(program string, args []string, dir string, env []string, tty bool, cols, rows int) (*native, error) {
	cmd := exec.Command(program, args...)
	cmd.Dir = dir
	cmd.Env = env
	p := &native{}
	var mu sync.Mutex
	exited := false
	var closeAfterStart []*os.File
	var all []*os.File
	success := false
	defer func() {
		if !success {
			for _, f := range all {
				f.Close()
			}
		}
	}()
	if tty {
		fd, e := unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
		if e != nil {
			return nil, e
		}
		master := os.NewFile(uintptr(fd), "pty")
		all = append(all, master)
		if e = unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0); e != nil {
			return nil, e
		}
		n, e := unix.IoctlGetInt(fd, unix.TIOCGPTN)
		if e != nil {
			return nil, e
		}
		slave, e := os.OpenFile(fmt.Sprintf("/dev/pts/%d", n), os.O_RDWR|unix.O_NOCTTY, 0)
		if e != nil {
			return nil, e
		}
		all = append(all, slave)
		closeAfterStart = append(closeAfterStart, slave)
		p.in = master
		p.out = master
		p.resize = func(c, r int) error {
			mu.Lock()
			defer mu.Unlock()
			if exited {
				return fmt.Errorf("terminal has exited")
			}
			return unix.IoctlSetWinsize(fd, unix.TIOCSWINSZ, &unix.Winsize{Col: uint16(c), Row: uint16(r)})
		}
		if e = p.resize(cols, rows); e != nil {
			return nil, e
		}
		cmd.Stdin = slave
		cmd.Stdout = slave
		cmd.Stderr = slave
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0, Pdeathsig: syscall.SIGKILL}
	} else {
		inR, inW, e := os.Pipe()
		if e != nil {
			return nil, e
		}
		all = append(all, inR, inW)
		outR, outW, e := os.Pipe()
		if e != nil {
			return nil, e
		}
		all = append(all, outR, outW)
		errR, errW, e := os.Pipe()
		if e != nil {
			return nil, e
		}
		all = append(all, errR, errW)
		cmd.Stdin = inR
		cmd.Stdout = outW
		cmd.Stderr = errW
		p.in = inW
		p.out = outR
		p.errout = errR
		closeAfterStart = append(closeAfterStart, inR, outW, errW)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	}
	if e := cmd.Start(); e != nil {
		return nil, e
	}
	p.pid = cmd.Process.Pid
	for _, f := range closeAfterStart {
		f.Close()
	}
	p.kill = func() error {
		mu.Lock()
		defer mu.Unlock()
		if exited {
			return nil
		}
		e := unix.Kill(-p.pid, unix.SIGKILL)
		if e == unix.ESRCH {
			return nil
		}
		return e
	}
	p.wait = func() (int, error) {
		e := cmd.Wait()
		mu.Lock()
		_ = unix.Kill(-p.pid, unix.SIGKILL)
		exited = true
		mu.Unlock()
		if !tty {
			p.in.Close()
		}
		code := cmd.ProcessState.ExitCode()
		return code, e
	}
	var once sync.Once
	p.close = func() {
		once.Do(func() {
			for _, f := range all {
				f.Close()
			}
		})
	}
	success = true
	return p, nil
}
