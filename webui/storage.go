package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// runStorage is the only bridge to project content. Production uses the
// novelgen binary's read-only storage commands; tests replace it with a host
// fixture implementation or a scripted fake.
var runStorage = func(stdin []byte, args ...string) ([]byte, error) {
	cmd := exec.Command(getNovelGenPath(), append([]string{"storage"}, args...)...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	return cmd.Output()
}

type storageEntry struct {
	Name    string    `json:"name"`
	Path    string    `json:"path,omitempty"`
	IsDir   bool      `json:"is_dir"`
	Size    int64     `json:"size,omitempty"`
	ModTime time.Time `json:"mod_time,omitempty"`
}

type storageProject struct {
	Key    string          `json:"key"`
	Path   string          `json:"path"`
	Name   string          `json:"name,omitempty"`
	Exists bool            `json:"exists"`
	Config json.RawMessage `json:"config,omitempty"`
}

type storageEnvelope struct {
	OK       bool             `json:"ok"`
	Code     string           `json:"code,omitempty"`
	Error    string           `json:"error,omitempty"`
	Content  string           `json:"content,omitempty"`
	Size     int64            `json:"size,omitempty"`
	Entry    *storageEntry    `json:"entry,omitempty"`
	Entries  []storageEntry   `json:"entries,omitempty"`
	Projects []storageProject `json:"projects,omitempty"`
}

type storageError struct {
	Code    string
	Message string
}

func (e *storageError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func isNotExist(err error) bool {
	var storageErr *storageError
	return errors.As(err, &storageErr) && storageErr.Code == "not_found"
}

func storageCall(stdin []byte, args ...string) (storageEnvelope, error) {
	out, runErr := runStorage(stdin, args...)
	var envelope storageEnvelope
	trimmed := bytes.TrimSpace(out)
	if len(trimmed) == 0 {
		if runErr != nil {
			return envelope, runErr
		}
		return envelope, fmt.Errorf("storage command returned no JSON")
	}
	if err := json.Unmarshal(trimmed, &envelope); err != nil {
		if runErr != nil {
			return envelope, fmt.Errorf("%w: %s", runErr, strings.TrimSpace(string(trimmed)))
		}
		return envelope, fmt.Errorf("decode storage response: %w", err)
	}
	if !envelope.OK {
		message := envelope.Error
		if message == "" {
			message = "storage operation failed"
		}
		return envelope, &storageError{Code: envelope.Code, Message: message}
	}
	return envelope, nil
}

func readProjectFile(project, rel string) ([]byte, error) {
	envelope, err := storageCall(nil, "read", "--project", project, "--path", rel)
	if err != nil {
		return nil, err
	}
	return []byte(envelope.Content), nil
}

func readProjectFileAt(project, fullPath string) ([]byte, error) {
	rel, err := projectRelative(project, fullPath)
	if err != nil {
		return nil, err
	}
	return readProjectFile(project, rel)
}

func listProjectDir(project, rel string) ([]os.DirEntry, error) {
	envelope, err := storageCall(nil, "list", "--project", project, "--path", rel)
	if err != nil {
		return nil, err
	}
	entries := make([]os.DirEntry, 0, len(envelope.Entries))
	for _, item := range envelope.Entries {
		entries = append(entries, storageDirEntry{entry: item})
	}
	return entries, nil
}

func listProjectDirAt(project, fullPath string) ([]os.DirEntry, error) {
	rel, err := projectRelative(project, fullPath)
	if err != nil {
		return nil, err
	}
	return listProjectDir(project, rel)
}

func statProjectPath(project, rel string) (os.FileInfo, error) {
	envelope, err := storageCall(nil, "stat", "--project", project, "--path", rel)
	if err != nil {
		return nil, err
	}
	if envelope.Entry == nil {
		return nil, fmt.Errorf("storage stat returned no entry")
	}
	return storageFileInfo{entry: *envelope.Entry}, nil
}

func statProjectPathAt(project, fullPath string) (os.FileInfo, error) {
	rel, err := projectRelative(project, fullPath)
	if err != nil {
		return nil, err
	}
	return statProjectPath(project, rel)
}

func writeProjectFile(project, rel string, data []byte) error {
	_, err := storageCall(data, "write", "--project", project, "--path", rel)
	return err
}

func writeProjectFileAt(project, fullPath string, data []byte) error {
	rel, err := projectRelative(project, fullPath)
	if err != nil {
		return err
	}
	return writeProjectFile(project, rel, data)
}

func listProjectsAt(workspace string) ([]storageProject, error) {
	envelope, err := storageCall(nil, "projects", "--root", workspace)
	if err != nil {
		return nil, err
	}
	return envelope.Projects, nil
}

// resolveProjectContentPath validates a client path. Host projects keep the
// symlink-aware allowlist; logical/remote projects are validated by the storage
// command's project-relative path checks.
func resolveProjectContentPath(project, rel string) (string, error) {
	clean, err := cleanContentPath(rel)
	if err != nil {
		return "", err
	}
	if filepath.IsAbs(project) {
		if _, statErr := os.Stat(project); statErr == nil {
			return resolveWithinRoot(project, clean)
		}
	}
	return filepath.Join(project, clean), nil
}

func projectRelative(project, fullPath string) (string, error) {
	base, err := filepath.Abs(project)
	if err != nil {
		return "", err
	}
	target, err := filepath.Abs(fullPath)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("path escapes project root: %q", fullPath)
	}
	return rel, nil
}

type storageDirEntry struct {
	entry storageEntry
}

func (e storageDirEntry) Name() string { return e.entry.Name }
func (e storageDirEntry) IsDir() bool  { return e.entry.IsDir }
func (e storageDirEntry) Type() fs.FileMode {
	if e.entry.IsDir {
		return fs.ModeDir
	}
	return 0
}
func (e storageDirEntry) Info() (fs.FileInfo, error) {
	return storageFileInfo{entry: e.entry}, nil
}

type storageFileInfo struct {
	entry storageEntry
}

func (i storageFileInfo) Name() string       { return i.entry.Name }
func (i storageFileInfo) Size() int64        { return i.entry.Size }
func (i storageFileInfo) ModTime() time.Time { return i.entry.ModTime }
func (i storageFileInfo) IsDir() bool        { return i.entry.IsDir }
func (i storageFileInfo) Sys() any           { return nil }
func (i storageFileInfo) Mode() fs.FileMode {
	if i.entry.IsDir {
		return fs.ModeDir | 0o755
	}
	return 0o644
}
