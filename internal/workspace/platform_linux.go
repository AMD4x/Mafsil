package workspace

import (
	"errors"
	"os"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

func readFlags() int                 { return os.O_RDONLY | unix.O_NOFOLLOW | unix.O_NONBLOCK }
func unsafeLink(fi os.FileInfo) bool { return fi.Mode()&os.ModeSymlink != 0 }
func singleLink(f *os.File, fi os.FileInfo) error {
	if fi.Sys().(*syscall.Stat_t).Nlink != 1 {
		return errors.New("hard-linked files are not accessible")
	}
	return nil
}
func cloneMetadata(src, dst *os.File, fi os.FileInfo) error {
	if fi.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 {
		return errors.New("privileged file modes are not editable")
	}
	s := fi.Sys().(*syscall.Stat_t)
	if err := dst.Chown(int(s.Uid), int(s.Gid)); err != nil {
		return err
	}
	n, e := unix.Flistxattr(int(src.Fd()), nil)
	if e != nil && e != unix.ENOTSUP {
		return e
	}
	if n > 65536 {
		return errors.New("extended attributes exceed metadata budget")
	}
	if n > 0 {
		names := make([]byte, n)
		n, e = unix.Flistxattr(int(src.Fd()), names)
		if e != nil {
			return e
		}
		total := 0
		for _, name := range strings.Split(strings.TrimRight(string(names[:n]), "\x00"), "\x00") {
			if name == "security.capability" {
				return errors.New("files with Linux capabilities are not editable")
			}
			size, e := unix.Fgetxattr(int(src.Fd()), name, nil)
			if e != nil {
				return e
			}
			total += size
			if total > 65536 {
				return errors.New("extended attributes exceed metadata budget")
			}
			b := make([]byte, size)
			n, e := unix.Fgetxattr(int(src.Fd()), name, b)
			if e != nil {
				return e
			}
			if e = unix.Fsetxattr(int(dst.Fd()), name, b[:n], 0); e != nil {
				return e
			}
		}
	}
	return dst.Chmod(fi.Mode().Perm())
}
