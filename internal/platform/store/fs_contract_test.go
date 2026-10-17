package store_test

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"novelgen/internal/platform/store"
)

// TestFSContract runs the same storage contract against the host filesystem and
// the in-memory backend. A new backend should be added here before it is wired
// into Root: these are the semantics the rest of the project relies on, not
// implementation details of MemFS.
func TestFSContract(t *testing.T) {
	for _, tc := range fsContractCases(t) {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runFSContract(t, tc.fs, tc.dir)
		})
	}
}

// TestRootAtomicWriteContract pins the guarantee Root.Write provides on every
// backend: a reader sees the old or the new whole file, never a temp file.
func TestRootAtomicWriteContract(t *testing.T) {
	for _, tc := range fsContractCases(t) {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := store.OpenFS(tc.dir, tc.fs)
			path := root.LogFile("atomic.txt")
			if err := root.WriteText(path, "first"); err != nil {
				t.Fatalf("first write: %v", err)
			}
			if err := root.WriteText(path, "second"); err != nil {
				t.Fatalf("second write: %v", err)
			}
			data, err := root.Read(path)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if string(data) != "second" {
				t.Fatalf("content = %q, want second", data)
			}
			entries, err := root.FS().ReadDir(root.LogsDir())
			if err != nil {
				t.Fatalf("readdir: %v", err)
			}
			for _, entry := range entries {
				if strings.Contains(entry.Name(), ".tmp") {
					t.Fatalf("atomic write left temp file %q", entry.Name())
				}
			}
		})
	}
}

type fsContractCase struct {
	name string
	fs   store.FS
	dir  string
}

func fsContractCases(t *testing.T) []fsContractCase {
	t.Helper()
	mem := store.NewMemFS()
	if err := mem.MkdirAll("/book", store.DirMode); err != nil {
		t.Fatalf("seed mem dir: %v", err)
	}
	return []fsContractCase{
		{name: "os", fs: store.OSFS{}, dir: t.TempDir()},
		{name: "mem", fs: mem, dir: "/book"},
	}
}

func runFSContract(t *testing.T, fsys store.FS, dir string) {
	t.Helper()
	if err := fsys.MkdirAll(dir, store.DirMode); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	file := filepath.Join(dir, "a.txt")
	if err := fsys.WriteFile(file, []byte("one"), store.FileMode); err != nil {
		t.Fatalf("write: %v", err)
	}
	data, err := fsys.ReadFile(file)
	if err != nil || string(data) != "one" {
		t.Fatalf("read = %q, %v", data, err)
	}

	if err := fsys.WriteFileExclusive(file, []byte("two"), store.FileMode); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("exclusive write on existing file = %v, want fs.ErrExist", err)
	}
	exclusive := filepath.Join(dir, "b.txt")
	if err := fsys.WriteFileExclusive(exclusive, []byte("two"), store.FileMode); err != nil {
		t.Fatalf("exclusive write: %v", err)
	}

	if err := fsys.AppendFile(file, []byte("!"), store.FileMode); err != nil {
		t.Fatalf("append: %v", err)
	}
	handle, err := fsys.OpenAppend(file, store.FileMode)
	if err != nil {
		t.Fatalf("open append: %v", err)
	}
	if _, err := handle.Write([]byte("?")); err != nil {
		t.Fatalf("append handle write: %v", err)
	}
	if err := handle.Close(); err != nil {
		t.Fatalf("append handle close: %v", err)
	}
	data, err = fsys.ReadFile(file)
	if err != nil || string(data) != "one!?" {
		t.Fatalf("appended content = %q, %v", data, err)
	}

	info, err := fsys.Stat(file)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.IsDir() || info.Size() != int64(len("one!?")) {
		t.Fatalf("stat = dir=%t size=%d", info.IsDir(), info.Size())
	}
	if info.ModTime().IsZero() {
		t.Fatal("stat mod time is zero")
	}

	entries, err := fsys.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) < 2 {
		t.Fatalf("readdir returned %d entries, want at least 2", len(entries))
	}

	renamed := filepath.Join(dir, "c.txt")
	if err := fsys.Rename(file, renamed); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if _, err := fsys.ReadFile(file); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("read old path = %v, want fs.ErrNotExist", err)
	}
	if data, err := fsys.ReadFile(renamed); err != nil || string(data) != "one!?" {
		t.Fatalf("read renamed = %q, %v", data, err)
	}

	// Rename must replace an existing destination, which is what Root.Write's
	// temp-file swap depends on.
	if err := fsys.WriteFile(exclusive, []byte("old"), store.FileMode); err != nil {
		t.Fatalf("seed overwrite destination: %v", err)
	}
	if err := fsys.WriteFile(file, []byte("new"), store.FileMode); err != nil {
		t.Fatalf("seed overwrite source: %v", err)
	}
	if err := fsys.Rename(file, exclusive); err != nil {
		t.Fatalf("replace rename: %v", err)
	}
	if data, err := fsys.ReadFile(exclusive); err != nil || string(data) != "new" {
		t.Fatalf("replaced content = %q, %v", data, err)
	}

	if err := fsys.Remove(filepath.Join(dir, "missing.txt")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("remove missing = %v, want fs.ErrNotExist", err)
	}
}
