package store

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
)

// FS is the filesystem seam behind Root. It exists so the storage layer can be
// exercised (and later re-implemented) without the host filesystem; it is
// deliberately small, so that a new backend is a day's work rather than a
// rewrite. Anything project-specific belongs on Root instead.
//
// The contract is "project files are read and written through Root, and Root
// only ever calls these methods". Two capabilities exist because callers
// genuinely need them: WriteFileExclusive allocates a file name race-free (agent
// logs, checkpoints), and OpenAppend keeps a streaming handle open (the run
// log). Everything else is whole-file IO, and a new backend only has to
// implement these ten methods.
type FS interface {
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm os.FileMode) error
	WriteFileExclusive(name string, data []byte, perm os.FileMode) error
	AppendFile(name string, data []byte, perm os.FileMode) error
	OpenAppend(name string, perm os.FileMode) (io.WriteCloser, error)
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

// WriteFileExclusive creates a file and fails with fs.ErrExist when it is
// already there, so callers can allocate a unique name without a race.
func (OSFS) WriteFileExclusive(name string, data []byte, perm os.FileMode) error {
	file, err := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
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

func (OSFS) OpenAppend(name string, perm os.FileMode) (io.WriteCloser, error) {
	return os.OpenFile(name, os.O_APPEND|os.O_CREATE|os.O_WRONLY, perm)
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
// place, so an interrupted write (Ctrl-C, crash) can never leave a half-written
// artifact behind: a reader sees either the previous content or the whole new
// content. Everything the project persists goes through here; the previous
// scattered writes could truncate outline.json mid-write.
//
// The rename is what makes the artifact appear atomically. Nothing is fsynced,
// so a machine-level crash may still lose the newest content, but it cannot
// leave a truncated artifact behind.
//
// The cost of the pattern is that a crash between the write and the rename can
// leave a stray "<name>.tmp<pid>-<n>" file next to the real one. That is a
// harmless artifact rather than corrupted project state, and it is what makes
// the rename worth it.
func writeFileAtomic(fs FS, path string, data []byte, perm os.FileMode) error {
	tmp := fmt.Sprintf("%s.tmp%d-%d", path, os.Getpid(), tempCounter.Add(1))
	if err := fs.WriteFile(tmp, data, perm); err != nil {
		// Deliberately no fallback to a plain write: silently degrading to a
		// non-atomic write trades the guarantee away at exactly the moment it
		// matters, and a backend that cannot host a sibling temp file cannot
		// host the destination either.
		return err
	}
	if err := fs.Rename(tmp, path); err != nil {
		_ = fs.Remove(tmp)
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}

// WriteExclusive writes a project file that must not exist yet, failing when the
// path is already taken. Callers use it to allocate a unique file name even when
// several writers race for it (agent log files, patch checkpoints).
func (r *Root) WriteExclusive(path string, data []byte, perm os.FileMode) error {
	if err := r.FS().MkdirAll(filepath.Dir(path), DirMode); err != nil {
		return err
	}
	return r.FS().WriteFileExclusive(path, data, perm)
}

// normalizeExt makes an extension start with a dot, so callers may pass either
// "json" or ".json".
func normalizeExt(ext string) string {
	if ext != "" && !strings.HasPrefix(ext, ".") {
		return "." + ext
	}
	return ext
}

// writeUniqueFile writes data to <dir>/<base><ext>, adding a numeric suffix
// until it finds a free name, and returns the path it used.
//
// The name is taken with O_EXCL rather than by checking whether the file exists:
// two writers in the same second, or two workers racing, would otherwise both
// see a free name and silently overwrite each other. Everything that allocates
// "one file per event" (patch checkpoints, agent logs, run logs) goes through
// here for that reason.
func writeUniqueFile(fileSystem FS, dir, base, ext string, data []byte, perm os.FileMode) (string, error) {
	if err := fileSystem.MkdirAll(dir, DirMode); err != nil {
		return "", err
	}
	ext = normalizeExt(ext)
	for attempt := 1; attempt <= 1000; attempt++ {
		name := base + ext
		if attempt > 1 {
			name = fmt.Sprintf("%s_%d%s", base, attempt, ext)
		}
		path := filepath.Join(dir, name)
		err := fileSystem.WriteFileExclusive(path, data, perm)
		if err == nil {
			return path, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return "", err
		}
	}
	return "", fmt.Errorf("could not allocate a unique file name in %s", dir)
}

// OpenAppend opens a project file for appending and hands back a handle the
// caller closes. It exists for output a project streams for as long as it runs
// (the run log), which cannot be expressed as a whole-file write.
func (r *Root) OpenAppend(path string, perm os.FileMode) (io.WriteCloser, error) {
	if err := r.FS().MkdirAll(filepath.Dir(path), DirMode); err != nil {
		return nil, err
	}
	return r.FS().OpenAppend(path, perm)
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
	_, err := r.Stat(path)
	return err == nil
}

// ExistsErr is Exists with the backend error preserved. Use it when a failed
// read must not be mistaken for "artifact absent" (for example on a remote
// backend with transient errors).
func (r *Root) ExistsErr(path string) (bool, error) {
	_, err := r.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, err
}

// Stat returns file information for a project path without collapsing a
// permission or backend failure into "missing". Callers that need to
// distinguish "not there yet" from "could not read it" should use this method
// and check errors.Is(err, fs.ErrNotExist) themselves.
func (r *Root) Stat(path string) (os.FileInfo, error) {
	return r.FS().Stat(path)
}

// Remove deletes a project file. A missing file is not an error, matching the
// best-effort cleanup callers used to write by hand.
func (r *Root) Remove(path string) error {
	if err := r.FS().Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
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

// Walk visits every file below dir depth-first, sorted by name inside each
// directory, and calls visit with the file's path and info. Directories are
// descended into but never passed to visit. It exists so recursive listing (the
// agent log plane) goes through the filesystem seam instead of filepath.WalkDir.
func (r *Root) Walk(dir string, visit func(path string, info os.FileInfo) error) error {
	entries, err := r.FS().ReadDir(dir)
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			if err := r.Walk(path, visit); err != nil {
				return err
			}
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if err := visit(path, info); err != nil {
			return err
		}
	}
	return nil
}

// DirExists reports whether a directory exists on the host filesystem. It is
// used to validate an explicitly requested project directory before any work
// starts.
func DirExists(dir string) bool {
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}
