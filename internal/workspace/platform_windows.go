package workspace

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var reopenFile = windows.NewLazySystemDLL("kernel32.dll").NewProc("ReOpenFile")

func readFlags() int                   { return os.O_RDONLY }
func directoryName(name string) string { return name }
func unsafeLink(fi os.FileInfo) bool {
	a, ok := fi.Sys().(*syscall.Win32FileAttributeData)
	return fi.Mode()&os.ModeSymlink != 0 || (ok && a.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0)
}
func singleLink(f *os.File, _ os.FileInfo) error {
	var i windows.ByHandleFileInformation
	if e := windows.GetFileInformationByHandle(windows.Handle(f.Fd()), &i); e != nil {
		return e
	}
	if i.NumberOfLinks != 1 {
		return errors.New("hard-linked files are not accessible")
	}
	return nil
}
func reopen(f *os.File, access uint32) (windows.Handle, error) {
	h, _, e := reopenFile.Call(f.Fd(), uintptr(access), windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, 0)
	if windows.Handle(h) == windows.InvalidHandle {
		return 0, e
	}
	return windows.Handle(h), nil
}
func defaultOwner(token windows.Token) (*windows.SID, error) {
	var size uint32
	if e := windows.GetTokenInformation(token, windows.TokenOwner, nil, 0, &size); e != windows.ERROR_INSUFFICIENT_BUFFER {
		return nil, fmt.Errorf("query default owner size: %v", e)
	}
	if size < uint32(unsafe.Sizeof(uintptr(0))) || size > 1024 {
		return nil, errors.New("unexpected default owner information size")
	}
	buffer := make([]byte, size)
	if e := windows.GetTokenInformation(token, windows.TokenOwner, &buffer[0], size, &size); e != nil {
		return nil, e
	}
	owner := (*struct{ Owner *windows.SID })(unsafe.Pointer(&buffer[0])).Owner
	copy, e := owner.Copy()
	runtime.KeepAlive(buffer)
	return copy, e
}
func cloneMetadata(src, dst *os.File, fi os.FileInfo) error {
	attrs := fi.Sys().(*syscall.Win32FileAttributeData).FileAttributes
	if attrs&(windows.FILE_ATTRIBUTE_READONLY|windows.FILE_ATTRIBUTE_ENCRYPTED|windows.FILE_ATTRIBUTE_COMPRESSED) != 0 {
		return errors.New("read-only, encrypted or compressed files are not editable")
	}
	// Fail closed rather than silently discarding alternate data streams.
	buf := make([]byte, 65536)
	if e := windows.GetFileInformationByHandleEx(windows.Handle(src.Fd()), 7, &buf[0], uint32(len(buf))); e != nil {
		return e
	}
	if *(*uint32)(unsafe.Pointer(&buf[0])) != 0 {
		return errors.New("files with alternate data streams are not editable")
	}
	sh, e := reopen(src, windows.READ_CONTROL)
	if e != nil {
		return fmt.Errorf("open source security: %w", e)
	}
	defer windows.CloseHandle(sh)
	sd, e := windows.GetSecurityInfo(sh, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.OWNER_SECURITY_INFORMATION)
	if e != nil {
		return e
	}
	owner, _, e := sd.Owner()
	if e != nil {
		return e
	}
	token := windows.GetCurrentProcessToken()
	tokenUser, e := token.GetTokenUser()
	if e != nil {
		return e
	}
	tokenOwner, e := defaultOwner(token)
	if e != nil {
		return e
	}
	if !owner.Equals(tokenUser.User.Sid) && !owner.Equals(tokenOwner) {
		return errors.New("only files owned by the process user or its default owner are editable")
	}
	acl, _, e := sd.DACL()
	if e != nil {
		return e
	}
	control, _, e := sd.Control()
	if e != nil {
		return e
	}
	dh, e := reopen(dst, windows.WRITE_DAC|windows.READ_CONTROL)
	if e != nil {
		return fmt.Errorf("open destination security: %w", e)
	}
	defer func() { windows.CloseHandle(dh) }()
	destination, e := windows.GetSecurityInfo(dh, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if e != nil {
		return e
	}
	destinationOwner, _, e := destination.Owner()
	if e != nil {
		return e
	}
	flags := windows.DACL_SECURITY_INFORMATION | windows.UNPROTECTED_DACL_SECURITY_INFORMATION
	if control&windows.SE_DACL_PROTECTED != 0 {
		flags = windows.DACL_SECURITY_INFORMATION | windows.PROTECTED_DACL_SECURITY_INFORMATION
	}
	var preserveOwner *windows.SID
	if !owner.Equals(destinationOwner) {
		withOwner, e := reopen(dst, windows.WRITE_OWNER|windows.WRITE_DAC|windows.READ_CONTROL)
		if e != nil {
			return fmt.Errorf("open destination owner: %w", e)
		}
		windows.CloseHandle(dh)
		dh = withOwner
		flags |= windows.OWNER_SECURITY_INFORMATION
		preserveOwner = owner
	}
	if e := windows.SetSecurityInfo(dh, windows.SE_FILE_OBJECT, windows.SECURITY_INFORMATION(flags), preserveOwner, nil, acl, nil); e != nil {
		return fmt.Errorf("preserve owner and DACL: %w", e)
	}
	return nil
}
