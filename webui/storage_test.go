package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

// hostStorageForTests keeps the web UI tests focused on HTTP handlers while the
// production path is still the novelgen storage command. The exec protocol has
// its own tests below.
func hostStorageForTests(stdin []byte, args ...string) ([]byte, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("missing storage operation")
	}
	op := args[0]
	var project, path, root string
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--project":
			if i+1 < len(args) {
				i++
				project = args[i]
			}
		case "--path":
			if i+1 < len(args) {
				i++
				path = args[i]
			}
		case "--root":
			if i+1 < len(args) {
				i++
				root = args[i]
			}
		}
	}
	if path == "" {
		path = "."
	}
	fail := func(code string, err error) ([]byte, error) {
		return json.Marshal(storageEnvelope{OK: false, Code: code, Error: err.Error()})
	}
	switch op {
	case "read":
		data, err := os.ReadFile(filepath.Join(project, filepath.FromSlash(path)))
		if err != nil {
			return fail("not_found", err)
		}
		return json.Marshal(storageEnvelope{OK: true, Content: string(data), Size: int64(len(data))})
	case "list":
		entries, err := os.ReadDir(filepath.Join(project, filepath.FromSlash(path)))
		if err != nil {
			return fail("not_found", err)
		}
		out := make([]storageEntry, 0, len(entries))
		for _, entry := range entries {
			item := storageEntry{Name: entry.Name(), IsDir: entry.IsDir()}
			if info, infoErr := entry.Info(); infoErr == nil {
				item.Size = info.Size()
				item.ModTime = info.ModTime()
			}
			out = append(out, item)
		}
		return json.Marshal(storageEnvelope{OK: true, Entries: out})
	case "stat":
		info, err := os.Stat(filepath.Join(project, filepath.FromSlash(path)))
		if err != nil {
			return fail("not_found", err)
		}
		return json.Marshal(storageEnvelope{OK: true, Entry: &storageEntry{
			Name: info.Name(), Path: path, IsDir: info.IsDir(), Size: info.Size(), ModTime: info.ModTime(),
		}})
	case "write":
		target := filepath.Join(project, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fail("storage_error", err)
		}
		if err := os.WriteFile(target, stdin, 0o644); err != nil {
			return fail("storage_error", err)
		}
		return []byte(`{"ok":true}`), nil
	case "projects":
		entries, err := os.ReadDir(root)
		if err != nil {
			return fail("not_found", err)
		}
		projects := make([]storageProject, 0, len(entries))
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			key := filepath.Join(root, entry.Name())
			data, readErr := os.ReadFile(filepath.Join(key, "novel.json"))
			item := storageProject{Key: key, Path: key}
			if readErr != nil {
				projects = append(projects, item)
				continue
			}
			item.Exists = true
			item.Config = data
			var config struct {
				Name string `json:"name"`
			}
			if json.Unmarshal(data, &config) == nil && config.Name != "" {
				item.Name = config.Name
			} else {
				item.Name = entry.Name()
			}
			projects = append(projects, item)
		}
		return json.Marshal(storageEnvelope{OK: true, Projects: projects})
	default:
		return nil, fmt.Errorf("unsupported storage operation %q", op)
	}
}

func TestStorageCallParsesExecResponse(t *testing.T) {
	previous := runStorage
	defer func() { runStorage = previous }()
	var got []string
	runStorage = func(_ []byte, args ...string) ([]byte, error) {
		got = args
		return []byte(`{"ok":true,"content":"hello","size":5}`), nil
	}
	data, err := readProjectFile("book", "novel.json")
	if err != nil {
		t.Fatalf("readProjectFile: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("data = %q, want hello", data)
	}
	want := []string{"read", "--project", "book", "--path", "novel.json"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("args = %v, want %v", got, want)
	}
}

func TestStorageCallMapsNotFound(t *testing.T) {
	previous := runStorage
	defer func() { runStorage = previous }()
	runStorage = func(_ []byte, _ ...string) ([]byte, error) {
		return []byte(`{"ok":false,"code":"not_found","error":"missing"}`), errors.New("exit status 1")
	}
	if _, err := readProjectFile("book", "missing.json"); !isNotExist(err) {
		t.Fatalf("error = %v, want not_found", err)
	}
}

func TestProjectGuardAcceptsLogicalStorageKey(t *testing.T) {
	previous := runStorage
	defer func() { runStorage = previous }()
	runStorage = func(_ []byte, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "stat" {
			return []byte(`{"ok":true,"entry":{"name":"novel.json","is_dir":false,"size":2}}`), nil
		}
		return []byte(`{"ok":false,"code":"not_found","error":"missing"}`), errors.New("exit status 1")
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(projectGuard())
	router.GET("/probe", func(c *gin.Context) {
		c.String(http.StatusOK, c.GetString(projectContextKey))
	})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/probe?project=logical-book-key", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "logical-book-key" {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
}
