package process

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

var updateAttribute = windows.NewLazySystemDLL("kernel32.dll").NewProc("UpdateProcThreadAttribute")

// Layout of JOBOBJECT_BASIC_ACCOUNTING_INFORMATION from winnt.h.
type jobAccounting struct {
	TotalUserTime, TotalKernelTime, PeriodUserTime, PeriodKernelTime int64
	PageFaults, TotalProcesses, ActiveProcesses, TerminatedProcesses uint32
}

func awaitEmptyJob(job windows.Handle) error {
	deadline := time.Now().Add(5 * time.Second)
	for {
		var accounting jobAccounting
		if e := windows.QueryInformationJobObject(job, windows.JobObjectBasicAccountingInformation, uintptr(unsafe.Pointer(&accounting)), uint32(unsafe.Sizeof(accounting)), nil); e != nil {
			return e
		}
		if accounting.ActiveProcesses == 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return errors.New("job object termination did not finish within five seconds")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func launch(program string, args []string, dir string, env []string, tty bool, cols, rows int) (*native, error) {
	job, e := windows.CreateJobObject(nil, nil)
	if e != nil {
		return nil, e
	}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, e = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); e != nil {
		windows.CloseHandle(job)
		return nil, e
	}
	p := &native{}
	var pseudo windows.Handle
	var pi windows.ProcessInformation
	var files []*os.File
	success := false
	defer func() {
		if !success {
			if pi.Process != 0 {
				windows.TerminateProcess(pi.Process, 1)
				windows.WaitForSingleObject(pi.Process, 5000)
				windows.CloseHandle(pi.Process)
			}
			if pi.Thread != 0 {
				windows.CloseHandle(pi.Thread)
			}
			for _, f := range files {
				f.Close()
			}
			if pseudo != 0 {
				windows.ClosePseudoConsole(pseudo)
			}
			windows.CloseHandle(job)
		}
	}()
	pipe := func() (*os.File, *os.File, error) {
		r, w, e := os.Pipe()
		if e == nil {
			files = append(files, r, w)
		}
		return r, w, e
	}
	inR, inW, e := pipe()
	if e != nil {
		return nil, e
	}
	outR, outW, e := pipe()
	if e != nil {
		return nil, e
	}
	p.in = inW
	p.out = outR
	attrs, e := windows.NewProcThreadAttributeList(1)
	if e != nil {
		return nil, e
	}
	defer attrs.Delete()
	si := windows.StartupInfoEx{}
	si.Cb = uint32(unsafe.Sizeof(si))
	si.ProcThreadAttributeList = attrs.List()
	si.Flags = windows.STARTF_USESTDHANDLES
	inherit := false
	flags := uint32(windows.CREATE_SUSPENDED | windows.CREATE_UNICODE_ENVIRONMENT | windows.EXTENDED_STARTUPINFO_PRESENT)
	var childEnds []*os.File
	if tty {
		if e = windows.CreatePseudoConsole(windows.Coord{X: int16(cols), Y: int16(rows)}, windows.Handle(inR.Fd()), windows.Handle(outW.Fd()), 0, &pseudo); e != nil {
			return nil, fmt.Errorf("ConPTY unavailable: %w", e)
		}
		// PSEUDOCONSOLE is a handle value, unlike pointer-valued attributes.
		r, _, err := updateAttribute.Call(uintptr(unsafe.Pointer(attrs.List())), 0, windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE, uintptr(pseudo), unsafe.Sizeof(pseudo), 0, 0)
		if r == 0 {
			return nil, err
		}
		childEnds = []*os.File{inR, outW}
	} else {
		errR, errW, e := pipe()
		if e != nil {
			return nil, e
		}
		p.errout = errR
		childEnds = []*os.File{inR, outW, errW}
		handles := []windows.Handle{windows.Handle(inR.Fd()), windows.Handle(outW.Fd()), windows.Handle(errW.Fd())}
		for _, h := range handles {
			if e = windows.SetHandleInformation(h, windows.HANDLE_FLAG_INHERIT, windows.HANDLE_FLAG_INHERIT); e != nil {
				return nil, e
			}
		}
		if e = attrs.Update(windows.PROC_THREAD_ATTRIBUTE_HANDLE_LIST, unsafe.Pointer(&handles[0]), uintptr(len(handles))*unsafe.Sizeof(handles[0])); e != nil {
			return nil, e
		}
		si.Flags = windows.STARTF_USESTDHANDLES
		si.StdInput = handles[0]
		si.StdOutput = handles[1]
		si.StdErr = handles[2]
		inherit = true
		flags |= windows.CREATE_NO_WINDOW
	}
	app, e := windows.UTF16PtrFromString(program)
	if e != nil {
		return nil, e
	}
	cmd, e := windows.UTF16FromString(windows.ComposeCommandLine(append([]string{program}, args...)))
	if e != nil {
		return nil, e
	}
	cwd, e := windows.UTF16PtrFromString(dir)
	if e != nil {
		return nil, e
	}
	environment := utf16.Encode([]rune(strings.Join(env, "\x00") + "\x00\x00"))
	if e = windows.CreateProcess(app, &cmd[0], nil, nil, inherit, flags, &environment[0], cwd, &si.StartupInfo, &pi); e != nil {
		return nil, e
	}
	if e = windows.AssignProcessToJobObject(job, pi.Process); e != nil {
		return nil, fmt.Errorf("cannot contain child process in Job Object: %w", e)
	}
	for _, f := range childEnds {
		f.Close()
	}
	if _, e = windows.ResumeThread(pi.Thread); e != nil {
		return nil, e
	}
	windows.CloseHandle(pi.Thread)
	pi.Thread = 0
	p.pid = int(pi.ProcessId)
	var mu sync.Mutex
	exited := false
	p.kill = func() error {
		mu.Lock()
		defer mu.Unlock()
		if exited {
			return nil
		}
		return windows.TerminateJobObject(job, 1)
	}
	p.resize = func(c, r int) error {
		mu.Lock()
		defer mu.Unlock()
		if pseudo == 0 || exited {
			return errors.New("terminal has exited")
		}
		return windows.ResizePseudoConsole(pseudo, windows.Coord{X: int16(c), Y: int16(r)})
	}
	p.wait = func() (int, error) {
		_, e := windows.WaitForSingleObject(pi.Process, windows.INFINITE)
		var code uint32
		if err := windows.GetExitCodeProcess(pi.Process, &code); e == nil {
			e = err
		}
		mu.Lock()
		terminateErr := windows.TerminateJobObject(job, 1)
		exited = true
		console := pseudo
		pseudo = 0
		mu.Unlock()
		// TerminateJobObject initiates termination; wait for all descendants
		// before claiming completion or removing their scratch directory.
		e = errors.Join(e, terminateErr, awaitEmptyJob(job))
		// Drain concurrently with ClosePseudoConsole; never hold a lock needed by
		// the output reader while Windows waits for its final terminal output.
		if console != 0 {
			windows.ClosePseudoConsole(console)
		}
		inW.Close()
		return int(code), e
	}
	var once sync.Once
	p.close = func() {
		once.Do(func() {
			for _, f := range files {
				f.Close()
			}
			mu.Lock()
			windows.CloseHandle(job)
			windows.CloseHandle(pi.Process)
			mu.Unlock()
		})
	}
	success = true
	return p, nil
}
