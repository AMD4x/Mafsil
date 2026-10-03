// Package workspace implements bounded, traversal-resistant file operations.
package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"
)

type Workspace struct {
	root     *os.Root
	path     string
	maxFile  int
	writable bool
	mu       sync.Mutex
}

func Open(path string, maxFile int, writable bool) (*Workspace, error) {
	if !filepath.IsAbs(path) {
		return nil, errors.New("workspace must be absolute")
	}
	path = filepath.Clean(path)
	if path == filepath.VolumeName(path)+string(filepath.Separator) {
		return nil, errors.New("a filesystem root cannot be a workspace")
	}
	// Configuration is trusted, but accidental links and junctions are refused.
	for p := path; ; p = filepath.Dir(p) {
		fi, e := os.Lstat(p)
		if e != nil {
			return nil, e
		}
		if unsafeLink(fi) {
			return nil, errors.New("workspace ancestors must not be links or reparse points")
		}
		if filepath.Dir(p) == p {
			break
		}
	}
	r, err := os.OpenRoot(directoryName(path))
	if err != nil {
		return nil, err
	}
	return &Workspace{root: r, path: path, maxFile: maxFile, writable: writable}, nil
}
func (w *Workspace) Close() error { return w.root.Close() }
func (w *Workspace) Path() string { return w.path }

// Names are deliberately portable: slash-separated, relative, without aliases.
func Name(name string, allowRoot bool) (string, error) {
	if name == "." && allowRoot {
		return name, nil
	}
	if name == "" || len(name) > 4096 || !utf8.ValidString(name) || strings.ContainsAny(name, "\\:\x00") || strings.HasPrefix(name, "/") {
		return "", errors.New("use a slash-separated path relative to the workspace")
	}
	for _, p := range strings.Split(name, "/") {
		if p == "" || p == "." || p == ".." || strings.HasSuffix(p, " ") || strings.HasSuffix(p, ".") || strings.HasPrefix(strings.ToLower(p), ".mafsil-") {
			return "", errors.New("path contains a forbidden component")
		}
		for _, r := range p {
			if r < 32 || r == 127 || strings.ContainsRune(`<>"|?*`, r) {
				return "", errors.New("path contains a non-portable character")
			}
		}
		base := strings.ToUpper(strings.SplitN(p, ".", 2)[0])
		letters := []rune(base)
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || (len(letters) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && strings.ContainsRune("123456789¹²³", letters[3])) {
			return "", errors.New("reserved device filename")
		}
	}
	return filepath.FromSlash(name), nil
}

func (w *Workspace) check(name string) error {
	if name == "." {
		return nil
	}
	p := ""
	for _, s := range strings.Split(filepath.ToSlash(name), "/") {
		p = filepath.Join(p, s)
		fi, err := w.root.Lstat(p)
		if err != nil {
			return err
		}
		if unsafeLink(fi) {
			return errors.New("links and reparse points are not accessible")
		}
	}
	return nil
}

func (w *Workspace) Directory(name string) (string, error) {
	n, err := Name(name, true)
	if err != nil {
		return "", err
	}
	if err = w.check(n); err != nil {
		return "", err
	}
	f, err := w.root.OpenRoot(directoryName(n))
	if err != nil {
		return "", err
	}
	defer f.Close()
	return filepath.Join(w.path, n), nil
}

type File struct {
	Path   string `json:"path"`
	Data   []byte `json:"-"`
	SHA256 string `json:"sha256"`
	Size   int    `json:"size"`
}

func Hash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

func (w *Workspace) Read(ctx context.Context, name string) (File, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := Name(name, false)
	if err != nil {
		return File{}, err
	}
	if err = w.check(n); err != nil {
		return File{}, err
	}
	f, err := w.root.OpenFile(n, readFlags(), 0)
	if err != nil {
		return File{}, err
	}
	defer f.Close()
	b, _, err := readRegular(ctx, f, w.maxFile)
	if err != nil {
		return File{}, err
	}
	return File{Path: name, Data: b, SHA256: Hash(b), Size: len(b)}, nil
}

func readRegular(ctx context.Context, f *os.File, limit int) ([]byte, os.FileInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	fi, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}
	if !fi.Mode().IsRegular() || unsafeLink(fi) {
		return nil, nil, errors.New("only regular files are accessible")
	}
	if err = singleLink(f, fi); err != nil {
		return nil, nil, err
	}
	if fi.Size() > int64(limit) {
		return nil, nil, fmt.Errorf("file exceeds %d bytes", limit)
	}
	b, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if err != nil {
		return nil, nil, err
	}
	if len(b) > limit {
		return nil, nil, errors.New("file grew beyond the size limit")
	}
	after, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}
	if fi.Size() != after.Size() || !fi.ModTime().Equal(after.ModTime()) {
		return nil, nil, errors.New("file changed while reading; retry")
	}
	if err = ctx.Err(); err != nil {
		return nil, nil, err
	}
	return b, fi, nil
}

type Entry struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	Size int64  `json:"size"`
}
type Listing struct {
	Entries    []Entry `json:"entries"`
	Truncated  bool    `json:"truncated"`
	NextOffset int     `json:"nextOffset"`
}

func (w *Workspace) List(ctx context.Context, name string, limit int) (Listing, error) {
	return w.ListPage(ctx, name, 0, limit)
}
func (w *Workspace) ListPage(ctx context.Context, name string, offset, limit int) (Listing, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := Listing{Entries: []Entry{}}
	if limit < 1 || limit > 2000 {
		return out, errors.New("limit must be 1–2000")
	}
	if offset < 0 || offset > 50000 {
		return out, errors.New("directory offset must be 0–50000")
	}
	n, e := Name(name, true)
	if e != nil {
		return out, e
	}
	if e = w.check(n); e != nil {
		return out, e
	}
	// Open as a directory first: opening a FIFO as an ordinary file can block
	// before ReadDir has a chance to reject it. Retain the pinned directory.
	directory, e := w.root.OpenRoot(directoryName(n))
	if e != nil {
		return out, e
	}
	defer directory.Close()
	f, e := directory.Open(".")
	if e != nil {
		return out, e
	}
	defer f.Close()
	for remaining := offset; remaining > 0; {
		if e = ctx.Err(); e != nil {
			return out, e
		}
		skipped, err := f.ReadDir(min(remaining, 256))
		remaining -= len(skipped)
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return out, err
		}
	}
	entries, e := f.ReadDir(limit + 1)
	if e != nil && e != io.EOF {
		return out, e
	}
	out.Truncated = len(entries) > limit
	out.NextOffset = offset + min(limit, len(entries))
	for _, item := range entries[:min(limit, len(entries))] {
		if e = ctx.Err(); e != nil {
			return out, e
		}
		info, e := item.Info()
		if e != nil {
			continue
		}
		kind := "file"
		if info.IsDir() {
			kind = "directory"
		}
		if unsafeLink(info) {
			kind = "blocked-link"
		} else if !info.IsDir() && !info.Mode().IsRegular() {
			kind = "special"
		}
		out.Entries = append(out.Entries, Entry{item.Name(), kind, info.Size()})
	}
	return out, nil
}

// Mkdir creates one directory. Parents must already exist, preventing hidden
// recursive mutations and making a partial failure unambiguous.
func (w *Workspace) Mkdir(ctx context.Context, name string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.writable {
		return errors.New("file writes are disabled")
	}
	n, e := Name(name, false)
	if e != nil {
		return e
	}
	p := filepath.Dir(n)
	if e = w.check(p); e != nil {
		return e
	}
	r, e := w.root.OpenRoot(directoryName(p))
	if e != nil {
		return e
	}
	defer r.Close()
	if e = ctx.Err(); e != nil {
		return e
	}
	return r.Mkdir(filepath.Base(n), 0700)
}
