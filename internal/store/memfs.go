package store

import (
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"
)

// MemFS is an in-memory FS implementation.
//
// It is what makes the storage seam worth having: a test can build a whole
// project in memory and drive a command against it, instead of chdir-ing the
// process into a temp directory (which is why this repository's tests could not
// run in parallel). Parent directories are created on demand, mirroring the
// "MkdirAll then write" pattern the real code uses.
type MemFS struct {
	mu    sync.RWMutex
	files map[string][]byte
	dirs  map[string]bool
}

// NewMemFS returns an empty in-memory filesystem with its root directory.
func NewMemFS() *MemFS {
	return &MemFS{
		files: map[string][]byte{},
		dirs:  map[string]bool{"/": true},
	}
}

// MemRoot returns a Root whose project directory is "book" inside a fresh
// in-memory filesystem. Tests use it to build a project without touching disk.
func MemRoot() *Root {
	return OpenFS("book", NewMemFS())
}

// key normalizes a host-shaped path into a slash path rooted inside MemFS.
func key(name string) string {
	cleaned := path.Clean(strings.ReplaceAll(name, `\`, "/"))
	if cleaned == "." {
		return "/"
	}
	if !strings.HasPrefix(cleaned, "/") {
		cleaned = "/" + cleaned
	}
	return cleaned
}

// SetFile seeds a file's contents directly, for tests that want fixtures.
func (m *MemFS) SetFile(name string, data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createParents(key(name))
	m.files[key(name)] = append([]byte(nil), data...)
}

// File returns a file's contents and whether it exists.
func (m *MemFS) File(name string) ([]byte, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, ok := m.files[key(name)]
	return append([]byte(nil), data...), ok
}

// Names lists every stored file path, sorted. It is used by tests that assert
// no temp files were left behind.
func (m *MemFS) Names() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.files))
	for name := range m.files {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (m *MemFS) createParents(name string) {
	for dir := path.Dir(name); dir != "/" && dir != "."; dir = path.Dir(dir) {
		m.dirs[dir] = true
	}
}

func (m *MemFS) ReadFile(name string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := key(name)
	if m.dirs[key] {
		return nil, &fs.PathError{Op: "read", Path: name, Err: errIsDir{}}
	}
	data, ok := m.files[key]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return append([]byte(nil), data...), nil
}

func (m *MemFS) WriteFile(name string, data []byte, perm os.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := key(name)
	if m.dirs[key] {
		return &fs.PathError{Op: "open", Path: name, Err: errIsDir{}}
	}
	m.createParents(key)
	m.files[key] = append([]byte(nil), data...)
	return nil
}

func (m *MemFS) AppendFile(name string, data []byte, perm os.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := key(name)
	m.createParents(key)
	m.files[key] = append(m.files[key], data...)
	return nil
}

func (m *MemFS) MkdirAll(dir string, perm os.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := key(dir)
	if _, ok := m.files[key]; ok {
		return &fs.PathError{Op: "mkdir", Path: dir, Err: fs.ErrExist}
	}
	m.dirs[key] = true
	m.createParents(key)
	return nil
}

func (m *MemFS) ReadDir(dir string) ([]os.DirEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := key(dir)
	if !m.dirs[key] && !m.hasChildren(key) {
		return nil, &fs.PathError{Op: "open", Path: dir, Err: fs.ErrNotExist}
	}
	seen := map[string]bool{}
	entries := make([]os.DirEntry, 0)
	for name := range m.files {
		if child, ok := directChild(key, name); ok && !seen[child] {
			seen[child] = true
			entries = append(entries, memDirEntry{name: child})
		}
	}
	for name := range m.dirs {
		if name == key {
			continue
		}
		if child, ok := directChild(key, name); ok && !seen[child] {
			seen[child] = true
			entries = append(entries, memDirEntry{name: child, dir: true})
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	return entries, nil
}

func (m *MemFS) hasChildren(dir string) bool {
	for name := range m.files {
		if _, ok := directChild(dir, name); ok {
			return true
		}
	}
	for name := range m.dirs {
		if _, ok := directChild(dir, name); ok {
			return true
		}
	}
	return false
}

func directChild(dir, candidate string) (string, bool) {
	if dir == candidate || !strings.HasPrefix(candidate, dir+"/") {
		return "", false
	}
	rest := strings.TrimPrefix(candidate, dir+"/")
	if rest == "" {
		return "", false
	}
	if index := strings.Index(rest, "/"); index >= 0 {
		return rest[:index], true
	}
	return rest, true
}

func (m *MemFS) Stat(name string) (os.FileInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := key(name)
	if data, ok := m.files[key]; ok {
		return memFileInfo{name: path.Base(key), size: int64(len(data))}, nil
	}
	if m.dirs[key] {
		return memFileInfo{name: path.Base(key), dir: true}, nil
	}
	return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
}

func (m *MemFS) Remove(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := key(name)
	if _, ok := m.files[key]; ok {
		delete(m.files, key)
		return nil
	}
	if m.dirs[key] {
		for file := range m.files {
			if file == key || strings.HasPrefix(file, key+"/") {
				return &fs.PathError{Op: "remove", Path: name, Err: errIsDir{}}
			}
		}
		delete(m.dirs, key)
		return nil
	}
	return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotExist}
}

func (m *MemFS) Rename(oldpath, newpath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	oldKey, newKey := key(oldpath), key(newpath)
	data, ok := m.files[oldKey]
	if !ok {
		return &fs.PathError{Op: "rename", Path: oldpath, Err: fs.ErrNotExist}
	}
	m.createParents(newKey)
	m.files[newKey] = data
	delete(m.files, oldKey)
	return nil
}

// errIsDir mirrors the platform's "is a directory" error without depending on
// syscall error values, so MemFS behaves the same on every host.
type errIsDir struct{}

func (errIsDir) Error() string { return "is a directory" }

func (errIsDir) Is(target error) bool { return target == fs.ErrInvalid }

type memDirEntry struct {
	name string
	dir  bool
}

func (e memDirEntry) Name() string               { return e.name }
func (e memDirEntry) IsDir() bool                { return e.dir }
func (e memDirEntry) Type() fs.FileMode          { return e.mode().Type() }
func (e memDirEntry) Info() (fs.FileInfo, error) { return memFileInfo{name: e.name, dir: e.dir}, nil }
func (e memDirEntry) mode() fs.FileMode {
	if e.dir {
		return fs.ModeDir | DirMode
	}
	return FileMode
}

type memFileInfo struct {
	name string
	size int64
	dir  bool
}

func (i memFileInfo) Name() string { return i.name }
func (i memFileInfo) Size() int64  { return i.size }
func (i memFileInfo) Mode() fs.FileMode {
	if i.dir {
		return fs.ModeDir | DirMode
	}
	return FileMode
}
func (i memFileInfo) ModTime() time.Time { return time.Time{} }
func (i memFileInfo) IsDir() bool        { return i.dir }
func (i memFileInfo) Sys() any           { return nil }

var _ FS = (*MemFS)(nil)
