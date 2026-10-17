package store

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ResolveProjectPath joins a client-supplied project-relative path to the
// project root and rejects traversal or absolute paths. It is the generic
// counterpart to the typed path helpers for callers that need to read files not
// yet modelled by Root (web UI editor files, legacy reports, and similar).
func (r *Root) ResolveProjectPath(rel string) (string, error) {
	clean, err := cleanProjectRelativePath(rel)
	if err != nil {
		return "", err
	}
	base := filepath.Clean(r.Dir())
	full := filepath.Clean(filepath.Join(base, clean))
	within, err := filepath.Rel(base, full)
	if err != nil {
		return "", err
	}
	if within == ".." || strings.HasPrefix(within, ".."+string(filepath.Separator)) || filepath.IsAbs(within) {
		return "", fmt.Errorf("%w: path escapes project root %q", fs.ErrInvalid, rel)
	}
	if hostDir, ok := r.HostDir(); ok {
		if err := validateHostContainment(hostDir, full); err != nil {
			return "", err
		}
	}
	return full, nil
}

// ReadRelative reads a project-relative file through the project's FS.
func (r *Root) ReadRelative(rel string) ([]byte, error) {
	path, err := r.ResolveProjectPath(rel)
	if err != nil {
		return nil, err
	}
	return r.Read(path)
}

// ListRelative lists a project-relative directory through the project's FS.
func (r *Root) ListRelative(rel string) ([]os.DirEntry, error) {
	path, err := r.ResolveProjectPath(rel)
	if err != nil {
		return nil, err
	}
	return r.FS().ReadDir(path)
}

// StatRelative stats a project-relative path through the project's FS.
func (r *Root) StatRelative(rel string) (os.FileInfo, error) {
	path, err := r.ResolveProjectPath(rel)
	if err != nil {
		return nil, err
	}
	return r.Stat(path)
}

// WriteRelative writes a project-relative file atomically through Root.Write.
func (r *Root) WriteRelative(rel string, data []byte, perm os.FileMode) error {
	path, err := r.ResolveProjectPath(rel)
	if err != nil {
		return err
	}
	return r.Write(path, data, perm)
}

func cleanProjectRelativePath(rel string) (string, error) {
	raw := strings.TrimSpace(rel)
	if raw == "" {
		return ".", nil
	}
	if strings.ContainsRune(raw, '\x00') {
		return "", fmt.Errorf("%w: invalid project path", fs.ErrInvalid)
	}
	raw = strings.ReplaceAll(raw, `\`, "/")
	if strings.HasPrefix(raw, "//") || strings.HasPrefix(raw, "/") {
		return "", fmt.Errorf("%w: invalid project path %q", fs.ErrInvalid, rel)
	}
	if len(raw) >= 2 && raw[1] == ':' {
		return "", fmt.Errorf("%w: invalid project path %q", fs.ErrInvalid, rel)
	}
	for _, part := range strings.Split(raw, "/") {
		if part == ".." {
			return "", fmt.Errorf("%w: invalid project path %q", fs.ErrInvalid, rel)
		}
	}
	clean := filepath.Clean(raw)
	if clean == "." {
		return ".", nil
	}
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: invalid project path %q", fs.ErrInvalid, rel)
	}
	return clean, nil
}

// validateHostContainment follows symlinks for the deepest existing ancestor of
// target and verifies that it remains below the resolved project root. MemFS and
// other backends do not have host symlinks and skip this check.
func validateHostContainment(root, target string) error {
	rootResolved, err := filepath.EvalSymlinks(filepath.Clean(root))
	if err != nil {
		return err
	}
	current := filepath.Clean(target)
	for {
		if resolved, evalErr := filepath.EvalSymlinks(current); evalErr == nil {
			rel, relErr := filepath.Rel(rootResolved, resolved)
			if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
				return fmt.Errorf("%w: path escapes project root %q", fs.ErrInvalid, target)
			}
			return nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return nil
		}
		current = parent
	}
}
