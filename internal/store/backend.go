package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Backend is the project-level entry point: it turns a caller-supplied
// directory (or an opaque project key) into a Root and optionally discovers a
// project from a start location.
//
// The default backend is HostBackend (OSFS). Tests and embedders can install a
// different backend once, and every command that uses OpenProject/LocateProject
// will then use it without threading a filesystem through the CLI.
type Backend interface {
	Name() string
	Open(dir string) (*Root, error)
	Locate(startDir string) (*Root, error)
}

// HostBackend opens projects on the host filesystem.
type HostBackend struct{}

func (HostBackend) Name() string { return "os" }

func (HostBackend) Open(dir string) (*Root, error) {
	return Open(dir)
}

func (HostBackend) Locate(startDir string) (*Root, error) {
	return Locate(startDir)
}

// ProjectInfo is the workspace-level view of one project.
type ProjectInfo struct {
	Key    string
	Path   string
	Exists bool
	Name   string
	Config []byte
}

// WorkspaceLister is implemented by backends that can enumerate the projects
// directly below a workspace directory.
type WorkspaceLister interface {
	ListProjects(workspace string) ([]ProjectInfo, error)
}

func (HostBackend) ListProjects(workspace string) ([]ProjectInfo, error) {
	if workspace == "" {
		workspace = "."
	}
	abs, err := filepath.Abs(workspace)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	projects := make([]ProjectInfo, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		key := filepath.Join(abs, entry.Name())
		root, err := Open(key)
		if err != nil {
			continue
		}
		config, err := root.Read(root.NovelJSON())
		info := ProjectInfo{Key: key, Path: key}
		if err != nil {
			projects = append(projects, info)
			continue
		}
		info.Exists = true
		info.Config = config
		var parsed struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(config, &parsed) == nil && parsed.Name != "" {
			info.Name = parsed.Name
		} else {
			info.Name = entry.Name()
		}
		projects = append(projects, info)
	}
	return projects, nil
}

var backendRegistry struct {
	mu      sync.RWMutex
	current Backend
}

func init() {
	backendRegistry.current = HostBackend{}
}

// SetBackend installs the process-wide project backend. Passing nil restores
// the host filesystem backend. A backend is expected to be safe for concurrent
// use because CLI commands may open several roots in one process.
func SetBackend(backend Backend) {
	if backend == nil {
		backend = HostBackend{}
	}
	backendRegistry.mu.Lock()
	defer backendRegistry.mu.Unlock()
	backendRegistry.current = backend
}

// CurrentBackend returns the installed project backend.
func CurrentBackend() Backend {
	backendRegistry.mu.RLock()
	defer backendRegistry.mu.RUnlock()
	return backendRegistry.current
}

// OpenProject opens a project through the installed backend.
func OpenProject(dir string) (*Root, error) {
	backend := CurrentBackend()
	if backend == nil {
		return nil, fmt.Errorf("no storage backend installed")
	}
	return backend.Open(dir)
}

// LocateProject discovers a project through the installed backend.
func LocateProject(startDir string) (*Root, error) {
	backend := CurrentBackend()
	if backend == nil {
		return nil, fmt.Errorf("no storage backend installed")
	}
	return backend.Locate(startDir)
}

// ListProjects enumerates projects through the installed backend. A backend
// that cannot list a workspace returns an explicit error rather than silently
// pretending the workspace is empty.
func ListProjects(workspace string) ([]ProjectInfo, error) {
	backend := CurrentBackend()
	lister, ok := backend.(WorkspaceLister)
	if !ok {
		return nil, fmt.Errorf("storage backend %q does not support workspace listing", backend.Name())
	}
	return lister.ListProjects(workspace)
}
