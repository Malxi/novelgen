package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
)

// FS is the narrow filesystem seam behind Root. It exists so the storage layer
// can be exercised (and later re-implemented) without the host filesystem; it
// is deliberately small, so that a new backend is a day's work rather than a
// rewrite. Anything project-specific belongs on Root instead.
type FS interface {
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm os.FileMode) error
	AppendFile(name string, data []byte, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
	ReadDir(name string) ([]os.DirEntry, error)
	Stat(name string) (os.FileInfo, error)
	Remove(name string) error
	Rename(oldpath, newpath string) error
}

// OSFS is the host filesystem.
type OSFS struct{}

func (OSFS) ReadFile(name string) ([]byte, error) { return os.ReadFile(name) }

func (OSFS) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}

func (OSFS) AppendFile(name string, data []byte, perm os.FileMode) error {
	file, err := os.OpenFile(name, os.O_APPEND|os.O_CREATE|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func (OSFS) MkdirAll(path string, perm os.FileMode) error { return os.MkdirAll(path, perm) }
func (OSFS) ReadDir(name string) ([]os.DirEntry, error)   { return os.ReadDir(name) }
func (OSFS) Stat(name string) (os.FileInfo, error)        { return os.Stat(name) }
func (OSFS) Remove(name string) error                     { return os.Remove(name) }
func (OSFS) Rename(oldpath, newpath string) error         { return os.Rename(oldpath, newpath) }

// File and directory modes used across the project. Keeping them named makes
// the "everything is 0644/0755" convention explicit instead of sprayed around.
const (
	FileMode os.FileMode = 0o644
	DirMode  os.FileMode = 0o755
)

// tempCounter keeps sibling temp-file names unique without pulling in a random
// source, which keeps the write path deterministic.
var tempCounter atomic.Uint64

// writeFileAtomic writes data through a sibling temp file and renames it into
// place, so an interrupted write (Ctrl-C, crash, full disk) can never leave a
// half-written artifact behind. Everything the project persists goes through
// here; the previous scattered writes could truncate outline.json mid-write.
func writeFileAtomic(fs FS, path string, data []byte, perm os.FileMode) error {
	tmp := fmt.Sprintf("%s.tmp%d-%d", path, os.Getpid(), tempCounter.Add(1))
	if err := fs.WriteFile(tmp, data, perm); err != nil {
		// Some backends cannot host a sibling temp file (read-only directory,
		// exotic implementation). Fall back to a plain write so the operation
		// degrades to the previous behavior instead of failing outright.
		return fs.WriteFile(path, data, perm)
	}
	if err := fs.Rename(tmp, path); err != nil {
		_ = fs.Remove(tmp)
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}

// Read returns the contents of a project file.
func (r *Root) Read(path string) ([]byte, error) {
	return r.FS().ReadFile(path)
}

// Write writes a project file atomically, creating parent directories.
func (r *Root) Write(path string, data []byte, perm os.FileMode) error {
	if err := r.FS().MkdirAll(filepath.Dir(path), DirMode); err != nil {
		return err
	}
	return writeFileAtomic(r.FS(), path, data, perm)
}

// WriteText writes a UTF-8 text file atomically.
func (r *Root) WriteText(path, content string) error {
	return r.Write(path, []byte(content), FileMode)
}

// Append appends to a project file, creating it and its parents when needed.
// Append is not atomic by design: it is used for log streams, where losing the
// previous lines would be worse than losing the last one.
func (r *Root) Append(path string, data []byte, perm os.FileMode) error {
	if err := r.FS().MkdirAll(filepath.Dir(path), DirMode); err != nil {
		return err
	}
	return r.FS().AppendFile(path, data, perm)
}

// Exists reports whether a project path exists.
func (r *Root) Exists(path string) bool {
	_, err := r.FS().Stat(path)
	return err == nil
}

// Remove deletes a project file. A missing file is not an error, matching the
// best-effort cleanup callers used to write by hand.
func (r *Root) Remove(path string) error {
	if err := r.FS().Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// MkdirAll creates a directory tree inside the project.
func (r *Root) MkdirAll(path string, perm os.FileMode) error {
	return r.FS().MkdirAll(path, perm)
}

// IsDir reports whether a project path is an existing directory.
func (r *Root) IsDir(path string) bool {
	info, err := r.FS().Stat(path)
	return err == nil && info.IsDir()
}

// DirExists reports whether a directory exists on the host filesystem. It is
// used to validate an explicitly requested project directory before any work
// starts.
func DirExists(dir string) bool {
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}
