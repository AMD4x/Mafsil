package workspace

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"
)

// Change is one final-state operation. Each path may occur once in a batch.
// ExpectedSHA256 is "missing" for creates, or the exact revision from read_file.
type Change struct {
	Operation      string  `json:"operation"`
	Path           string  `json:"path"`
	ExpectedSHA256 string  `json:"expectedSha256"`
	Content        *string `json:"content,omitempty"`
	Edits          []Edit  `json:"edits,omitempty"`
	Destination    string  `json:"destination,omitempty"`
}
type Edit struct {
	Old   string `json:"old"`
	New   string `json:"new"`
	Count int    `json:"count"`
}
type EditResult struct {
	Path    string `json:"path"`
	SHA256  string `json:"sha256"`
	Deleted bool   `json:"deleted,omitempty"`
}
type target struct {
	parent        *os.Root
	name, path    string
	before        []byte
	info          os.FileInfo
	after         []byte
	remove        bool
	stage, backup string
	applied       bool
	metadataFrom  *target
	publishedInfo os.FileInfo
}

func (w *Workspace) Edit(ctx context.Context, changes []Change) ([]EditResult, error) {
	return w.edit(ctx, changes, nil)
}

// hook injects IO failures at the commit boundary in regression tests.
func (w *Workspace) edit(ctx context.Context, changes []Change, hook func(int) error) ([]EditResult, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.writable {
		return nil, errors.New("file writes are disabled")
	}
	if len(changes) < 1 || len(changes) > 32 {
		return nil, errors.New("changes must contain 1–32 operations")
	}
	var targets []*target
	defer func() {
		for _, t := range targets {
			if t.stage != "" {
				_ = t.parent.Remove(t.stage)
			}
			t.parent.Close()
		}
	}()
	seen := map[string]bool{}
	budget := 0
	add := func(name string) (*target, error) {
		n, e := Name(name, false)
		if e != nil {
			return nil, e
		}
		key := n
		if runtime.GOOS == "windows" {
			key = strings.ToLower(key)
		}
		if seen[key] {
			return nil, errors.New("a path may occur only once per batch")
		}
		seen[key] = true
		parent := filepath.Dir(n)
		if e = w.check(parent); e != nil {
			return nil, e
		}
		r, e := w.root.OpenRoot(parent)
		if e != nil {
			return nil, e
		}
		t := &target{parent: r, name: filepath.Base(n), path: name}
		targets = append(targets, t)
		fi, e := r.Lstat(t.name)
		if errors.Is(e, os.ErrNotExist) {
			return t, nil
		}
		if e != nil {
			return nil, e
		}
		if unsafeLink(fi) || !fi.Mode().IsRegular() {
			return nil, errors.New("edit target must be a regular non-link file")
		}
		f, e := r.OpenFile(t.name, readFlags(), 0)
		if e != nil {
			return nil, e
		}
		defer f.Close()
		t.before, t.info, e = readRegular(ctx, f, w.maxFile)
		if e != nil {
			return nil, e
		}
		budget += len(t.before)
		if budget > 16<<20 {
			return nil, errors.New("batch exceeds 16 MiB snapshot budget")
		}
		return t, nil
	}
	for _, c := range changes {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		t, e := add(c.Path)
		if e != nil {
			return nil, fmt.Errorf("%s: %w", c.Path, e)
		}
		if t.info == nil {
			if c.ExpectedSHA256 != "missing" {
				return nil, errors.New("create requires expectedSha256=missing")
			}
		} else {
			if len(c.ExpectedSHA256) != 64 || c.ExpectedSHA256 != Hash(t.before) {
				return nil, fmt.Errorf("revision mismatch: %s; read the file again", c.Path)
			}
		}
		switch c.Operation {
		case "write":
			if c.Content == nil || len(c.Edits) != 0 || c.Destination != "" {
				return nil, errors.New("write requires content only")
			}
			t.after = []byte(*c.Content)
		case "edit", "move":
			if t.info == nil || c.Content != nil {
				return nil, errors.New("edit/move requires an existing file and no content field")
			}
			if len(c.Edits) > 0 && !utf8.Valid(t.before) {
				return nil, errors.New("text edits require UTF-8")
			}
			text := string(t.before)
			if len(c.Edits) > 128 {
				return nil, errors.New("at most 128 exact edits per file")
			}
			for _, edit := range c.Edits {
				if edit.Old == "" || edit.Count < 1 || edit.Count > 10000 {
					return nil, errors.New("each edit requires nonempty old text and an exact positive count")
				}
				if strings.Count(text, edit.Old) != edit.Count {
					return nil, errors.New("exact edit match count differs; add context or reread")
				}
				if len(text)+edit.Count*(len(edit.New)-len(edit.Old)) > w.maxFile {
					return nil, errors.New("edited file exceeds size limit")
				}
				text = strings.ReplaceAll(text, edit.Old, edit.New)
			}
			t.after = []byte(text)
			if c.Operation == "move" {
				if c.Destination == "" {
					return nil, errors.New("move requires destination")
				}
				dst, e := add(c.Destination)
				if e != nil {
					return nil, e
				}
				if dst.info != nil {
					return nil, errors.New("move destination already exists")
				}
				dst.after = t.after
				dst.metadataFrom = t
				t.remove = true
			} else if len(c.Edits) == 0 || c.Destination != "" {
				return nil, errors.New("edit requires edits and no destination")
			}
		case "delete":
			if t.info == nil || c.Content != nil || len(c.Edits) > 0 || c.Destination != "" {
				return nil, errors.New("delete requires an existing file and no content/edits/destination")
			}
			t.remove = true
		default:
			return nil, errors.New("operation must be write, edit, move or delete")
		}
		if len(t.after) > w.maxFile {
			return nil, errors.New("file exceeds size limit")
		}
	}
	// Prepare every new inode before any target is changed.
	for _, t := range targets {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		budget += len(t.after)
		if budget > 32<<20 {
			return nil, errors.New("batch exceeds 32 MiB memory budget")
		}
		if t.remove {
			continue
		}
		t.stage = ".mafsil-stage-" + randomName()
		f, e := t.parent.OpenFile(t.stage, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
		if e != nil {
			return nil, e
		}
		_, e = f.Write(t.after)
		donor := t
		if t.metadataFrom != nil {
			donor = t.metadataFrom
		}
		if e == nil && donor.info != nil {
			src, err := donor.parent.OpenFile(donor.name, readFlags(), 0)
			if err != nil {
				e = err
			} else {
				e = cloneMetadata(src, f, donor.info)
				src.Close()
			}
		}
		if e == nil {
			e = f.Sync()
		}
		if e == nil {
			t.publishedInfo, e = f.Stat()
		}
		closeErr := f.Close()
		if e == nil {
			e = closeErr
		}
		if e != nil {
			return nil, e
		}
	}
	for _, t := range targets {
		if e := t.unchanged(ctx, w.maxFile); e != nil {
			return nil, e
		}
	}
	// Cancellation accepted before this point prevents publication. After this
	// point complete the batch or roll it back; a cancelled caller must reread.
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	fail := func(cause error) ([]EditResult, error) {
		var recovery []error
		for i := len(targets) - 1; i >= 0; i-- {
			t := targets[i]
			if !t.applied {
				continue
			}
			if e := t.checkApplied(w.maxFile); e != nil {
				recovery = append(recovery, fmt.Errorf("%s: %w; backup %s", t.path, e, t.backup))
				continue
			}
			var e error
			if t.backup != "" {
				e = t.parent.Rename(t.backup, t.name)
				if e == nil {
					t.backup = ""
				}
			} else if !t.remove {
				e = t.parent.Remove(t.name)
			}
			if e != nil {
				recovery = append(recovery, fmt.Errorf("restore %s: %w", t.path, e))
			}
		}
		for _, t := range targets {
			if !t.applied && t.backup != "" {
				if e := t.parent.Remove(t.backup); e != nil {
					recovery = append(recovery, e)
				} else {
					t.backup = ""
				}
			}
		}
		if len(recovery) > 0 {
			return nil, fmt.Errorf("%w; rollback incomplete, retain .mafsil-backup files: %v", cause, errors.Join(recovery...))
		}
		return nil, fmt.Errorf("%w; changes rolled back", cause)
	}
	for i, t := range targets {
		if e := t.unchanged(context.Background(), w.maxFile); e != nil {
			return fail(e)
		}
		if t.info != nil {
			t.backup = ".mafsil-backup-" + randomName()
			if e := t.parent.Link(t.name, t.backup); e != nil {
				t.backup = ""
				return fail(e)
			}
		}
		var e error
		if t.remove {
			e = t.parent.Remove(t.name)
		} else if t.info == nil {
			e = t.parent.Link(t.stage, t.name)
		} else {
			e = t.parent.Rename(t.stage, t.name)
		}
		if e != nil {
			return fail(e)
		}
		t.applied = true
		if t.info != nil && !t.remove {
			t.stage = ""
		}
		if hook != nil {
			if e = hook(i + 1); e != nil {
				return fail(e)
			}
		}
	}
	results := make([]EditResult, 0, len(targets))
	var cleanup []error
	for _, t := range targets {
		if t.backup != "" {
			if e := t.parent.Remove(t.backup); e != nil {
				cleanup = append(cleanup, e)
			} else {
				t.backup = ""
			}
		}
		if t.stage != "" {
			if e := t.parent.Remove(t.stage); e != nil {
				cleanup = append(cleanup, e)
			} else {
				t.stage = ""
			}
		}
		r := EditResult{Path: t.path, Deleted: t.remove}
		if !t.remove {
			r.SHA256 = Hash(t.after)
		}
		results = append(results, r)
	}
	if len(cleanup) > 0 {
		return results, fmt.Errorf("changes committed; temporary file cleanup failed: %w", errors.Join(cleanup...))
	}
	return results, nil
}

func (t *target) unchanged(ctx context.Context, limit int) error {
	fi, e := t.parent.Lstat(t.name)
	if t.info == nil {
		if errors.Is(e, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("destination appeared: %s", t.path)
	}
	if e != nil {
		return e
	}
	if unsafeLink(fi) || !os.SameFile(t.info, fi) {
		return fmt.Errorf("file identity changed: %s", t.path)
	}
	f, e := t.parent.OpenFile(t.name, readFlags(), 0)
	if e != nil {
		return e
	}
	defer f.Close()
	b, _, e := readRegular(ctx, f, limit)
	if e != nil {
		return e
	}
	if !bytes.Equal(b, t.before) {
		return fmt.Errorf("file changed: %s", t.path)
	}
	return nil
}
func (t *target) checkApplied(limit int) error {
	if t.remove {
		_, e := t.parent.Lstat(t.name)
		if errors.Is(e, os.ErrNotExist) {
			return nil
		}
		return errors.New("deleted path was recreated externally")
	}
	f, e := t.parent.OpenFile(t.name, readFlags(), 0)
	if e != nil {
		return e
	}
	defer f.Close()
	// A just-created target still has its stage hard link until cleanup.
	fi, e := f.Stat()
	if e != nil || !fi.Mode().IsRegular() {
		return errors.New("published target changed type")
	}
	if t.publishedInfo == nil || !os.SameFile(t.publishedInfo, fi) || fi.Mode() != t.publishedInfo.Mode() || !fi.ModTime().Equal(t.publishedInfo.ModTime()) {
		return errors.New("published target identity or metadata changed")
	}
	b := make([]byte, len(t.after)+1)
	n, e := f.ReadAt(b, 0)
	_ = e
	if n != len(t.after) || !bytes.Equal(b[:n], t.after) {
		return errors.New("published file was modified externally")
	}
	return nil
}
func randomName() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
